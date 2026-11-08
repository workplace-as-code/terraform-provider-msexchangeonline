# management_role.ps1 - custom management roles: a copy of a parent role that
# keeps only some of its cmdlets (role entries) and, for some of them, only
# some parameters.
#
#   -Identity  role name
#   -Values    base64 JSON: Parent, Description, Cmdlets (the cmdlets kept) and
#              CmdletParameters (cmdlet -> parameters kept)
#
# Output: Name, Parent, Description, Cmdlets (sorted) and CmdletParameters, the
# entries whose parameters differ from the parent's. Built-in (root) roles are
# refused: Microsoft manages them.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$wanted = if ($Values) { ConvertFrom-Json -InputObject ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Values))) } else { $null }

# Get-BridgeRoleEntries returns the entries of a role: cmdlet -> sorted parameters.
function Get-BridgeRoleEntries([string]$Role) {
    $entries = [ordered]@{}
    foreach ($e in @(Invoke-BridgeRetry { Get-ManagementRoleEntry -Identity "$Role\*" })) {
        $entries[[string]$e.Name] = [string[]]@(@(Get-BridgeProp $e "Parameters" @()) | ForEach-Object { [string]$_ } | Sort-Object)
    }
    $entries
}

# Get-BridgeLeafName returns the role name of an identity: Exchange Online
# returns the parent as "<tenant>\<role>".
function Get-BridgeLeafName([string]$Value) {
    @($Value -split '\\' | Where-Object { $_ })[-1]
}

# Find-BridgeKey returns the key of $Map equal to $Name whatever the case, or $null.
function Find-BridgeKey($Map, [string]$Name) {
    foreach ($k in $Map.Keys) { if ($k -ieq $Name) { return $k } }
    $null
}

# The wanted parameters of a cmdlet, or $null when they are not restricted.
function Get-BridgeWantedParameters([string]$Cmdlet) {
    if ($null -eq $wanted -or $null -eq $wanted.CmdletParameters) { return $null }
    foreach ($p in $wanted.CmdletParameters.PSObject.Properties) {
        if ($p.Name -ieq $Cmdlet) { return [string[]]@($p.Value | ForEach-Object { [string]$_ } | Sort-Object) }
    }
    $null
}

function Read-BridgeRole {
    $role = Invoke-BridgeRetry { Get-ManagementRole -Identity $Identity }
    if ([bool](Get-BridgeProp $role "IsRootRole" $false)) {
        throw "$Identity is a built-in role: only custom roles can be managed."
    }
    $parent = [string](Get-BridgeProp $role "Parent" "")
    if ($parent) { $parent = Get-BridgeLeafName $parent }
    $entries = Get-BridgeRoleEntries $Identity
    $parentEntries = if ($parent) { Get-BridgeRoleEntries $parent } else { [ordered]@{} }
    $restricted = [ordered]@{}
    foreach ($cmdlet in $entries.Keys) {
        $p = Find-BridgeKey $parentEntries $cmdlet
        if ($null -ne $p -and (($entries[$cmdlet] -join ',') -ne ($parentEntries[$p] -join ','))) {
            $restricted[$cmdlet] = $entries[$cmdlet]
        }
    }
    ConvertTo-Json -Compress -Depth 4 -InputObject ([ordered]@{
        Name             = [string]$role.Name
        Parent           = $parent
        Description      = [string](Get-BridgeProp $role "Description" "")
        Cmdlets          = [string[]]@($entries.Keys | Sort-Object)
        CmdletParameters = $restricted
    })
}

# Sync-BridgeRoleParameters restricts the parameters of the wanted cmdlets, and
# gives back all the parent's parameters to the cmdlets no longer restricted.
function Sync-BridgeRoleParameters($ParentEntries) {
    $entries = Get-BridgeRoleEntries $Identity
    foreach ($cmdlet in $entries.Keys) {
        $p = Find-BridgeKey $ParentEntries $cmdlet
        $want = Get-BridgeWantedParameters $cmdlet
        if ($null -eq $want -and $null -ne $p) { $want = $ParentEntries[$p] }
        if ($null -eq $want) { continue }
        if ($null -ne $p) {
            $extra = @($want | Where-Object { $ParentEntries[$p] -notcontains $_ })
            if ($extra.Count -gt 0) { throw "$cmdlet in the parent role has no parameter $($extra -join ', ')." }
        }
        if (($entries[$cmdlet] -join ',') -ne ($want -join ',')) {
            Invoke-BridgeRetry { Set-ManagementRoleEntry -Identity "$Identity\$cmdlet" -Parameters $want }
        }
    }
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgeRole } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $parentEntries = Get-BridgeRoleEntries $wanted.Parent
            $cmdlets = [string[]]@($wanted.Cmdlets | ForEach-Object {
                $k = Find-BridgeKey $parentEntries $_
                if ($null -eq $k) { throw "$_ is not in the parent role $($wanted.Parent)." }
                $k
            } | Sort-Object)
            # Check the parameters before creating the role: a failed create must
            # not leave a role behind.
            foreach ($c in $cmdlets) {
                $want = Get-BridgeWantedParameters $c
                if ($null -eq $want) { continue }
                $extra = @($want | Where-Object { $parentEntries[$c] -notcontains $_ })
                if ($extra.Count -gt 0) { throw "$c in the parent role has no parameter $($extra -join ', ')." }
            }
            $new = @{ Name = $Identity; Parent = $wanted.Parent; EnabledCmdlets = $cmdlets }
            if ($null -ne $wanted.PSObject.Properties["Description"]) { $new["Description"] = $wanted.Description }
            Invoke-BridgeRetry -Create { New-ManagementRole @new | Out-Null }
            Sync-BridgeRoleParameters $parentEntries
            Read-BridgeRole
        }

        "Update" {
            $role = Invoke-BridgeRetry { Get-ManagementRole -Identity $Identity }
            $parentName = Get-BridgeLeafName ([string]$role.Parent)
            $parentEntries = Get-BridgeRoleEntries $parentName
            # The description never changes here: Exchange has no cmdlet for
            # it, so a new description replaces the role.
            $entries = Get-BridgeRoleEntries $Identity
            foreach ($c in @($wanted.Cmdlets)) {
                if ($null -ne (Find-BridgeKey $entries $c)) { continue }
                $k = Find-BridgeKey $parentEntries $c
                if ($null -eq $k) { throw "$c is not in the parent role $parentName." }
                Invoke-BridgeRetry { Add-ManagementRoleEntry -Identity "$Identity\$k" | Out-Null }
            }
            foreach ($c in @($entries.Keys)) {
                if (@($wanted.Cmdlets | Where-Object { $_ -ieq $c }).Count -eq 0) {
                    Invoke-BridgeRetry { Remove-ManagementRoleEntry -Identity "$Identity\$c" }
                }
            }
            Sync-BridgeRoleParameters $parentEntries
            Read-BridgeRole
        }

        "Delete" {
            try {
                Remove-ManagementRole -Identity $Identity
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("ManagementRole $Action failed: $($_.Exception.Message)")
    exit 1
}
