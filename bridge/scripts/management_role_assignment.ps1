# management_role_assignment.ps1 - management role assignments of role groups
# (-SecurityGroup) and of applications (-App, RBAC for Applications).
#
#   -Identity  assignment name ("" on Create: Exchange names it unless Name is set)
#   -Values    base64 JSON: Name, Role, RoleGroup or App, the scopes
#              (CustomRecipientWriteScope, ExclusiveRecipientWriteScope,
#              CustomResourceScope, RecipientAdministrativeUnitScope,
#              RecipientGroupScope: "" = none), RecipientRelativeWriteScope, Enabled
#
# The scopes are authoritative: on Update, a scope set in Exchange and left out
# of the configuration is removed, which gives the role back its built-in scope
# (RecipientRelativeWriteScope, Organization by default).

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity = "",
    [string]$Values   = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$wanted = if ($Values) { ConvertFrom-Json -InputObject ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Values))) } else { $null }
$ScopeParams = "CustomRecipientWriteScope", "ExclusiveRecipientWriteScope", "CustomResourceScope",
    "RecipientAdministrativeUnitScope", "RecipientGroupScope"

# Get-BridgeWanted returns a value of the configuration, or $Default.
function Get-BridgeWanted([string]$Name, $Default = "") {
    if ($null -ne $wanted -and $null -ne $wanted.PSObject.Properties[$Name]) { $wanted.$Name } else { $Default }
}

function Read-BridgeAssignment([string]$Name) {
    $a = Invoke-BridgeRetry { Get-ManagementRoleAssignment -Identity $Name }
    $type = [string](Get-BridgeProp $a "RoleAssigneeType" "")
    $assignee = [string](Get-BridgeProp $a "RoleAssigneeName" "")
    $out = [ordered]@{ Name = [string]$a.Name; Role = [string](Get-BridgeProp $a "Role" ""); RoleGroup = ""; App = "" }
    if ($type -eq "RoleGroup") {
        $out.RoleGroup = $assignee
    } else {
        # An application assignment names the Exchange service principal: the
        # configuration names it by app ID.
        $sp = Invoke-BridgeRetry { Get-ServicePrincipal -Identity $assignee }
        $out.App = [string](Get-BridgeProp $sp "AppId" "")
    }
    foreach ($p in $ScopeParams) { $out[$p] = [string](Get-BridgeProp $a $p "") }
    $out.RecipientRelativeWriteScope = [string](Get-BridgeProp $a "RecipientWriteScope" "")
    # Exchange may report an administrative unit or group scope as the custom
    # recipient scope, with RecipientWriteScope naming its kind.
    $kinds = @{ AdministrativeUnit = "RecipientAdministrativeUnitScope"; Group = "RecipientGroupScope" }
    $kind = $kinds[$out.RecipientRelativeWriteScope]
    if ($kind -and -not $out[$kind] -and $out.CustomRecipientWriteScope) {
        $out[$kind] = $out.CustomRecipientWriteScope
        $out.CustomRecipientWriteScope = ""
    }
    $out.Enabled = [bool](Get-BridgeProp $a "Enabled" $true)
    ConvertTo-Json -Compress -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgeAssignment $Identity } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $new = @{ Role = $wanted.Role }
            if (Get-BridgeWanted "RoleGroup") { $new.SecurityGroup = $wanted.RoleGroup } else { $new.App = $wanted.App }
            if (Get-BridgeWanted "Name") { $new.Name = $wanted.Name }
            foreach ($p in $ScopeParams) { if (Get-BridgeWanted $p) { $new[$p] = $wanted.$p } }
            if (Get-BridgeWanted "RecipientRelativeWriteScope") { $new.RecipientRelativeWriteScope = $wanted.RecipientRelativeWriteScope }
            $created = Invoke-BridgeRetry -Create { New-ManagementRoleAssignment @new }
            $name = [string](@($created)[0].Name)
            if ((Get-BridgeWanted "Enabled" $true) -eq $false) {
                Invoke-BridgeRetry { Set-ManagementRoleAssignment -Identity $name -Enabled $false }
            }
            Read-BridgeAssignment $name
        }

        "Update" {
            $current = Invoke-BridgeRetry { Get-ManagementRoleAssignment -Identity $Identity }
            $set = @{}
            $scoped = $false
            foreach ($p in $ScopeParams) {
                $want = [string](Get-BridgeWanted $p)
                if ($want) { $scoped = $true }
                if ($want -and $want -ne [string](Get-BridgeProp $current $p "")) { $set[$p] = $want }
            }
            $relative = [string](Get-BridgeWanted "RecipientRelativeWriteScope")
            $hadScope = @($ScopeParams | Where-Object { [string](Get-BridgeProp $current $_ "") }).Count -gt 0
            if (-not $scoped -and ($hadScope -or ($relative -and $relative -ne [string](Get-BridgeProp $current "RecipientWriteScope" "")))) {
                $set.RecipientRelativeWriteScope = if ($relative) { $relative } else { "Organization" }
            }
            $enabled = Get-BridgeWanted "Enabled" $null
            if ($null -ne $enabled -and [bool]$enabled -ne [bool](Get-BridgeProp $current "Enabled" $true)) { $set.Enabled = [bool]$enabled }
            if ($set.Count -gt 0) { Invoke-BridgeRetry { Set-ManagementRoleAssignment -Identity $Identity @set } }
            Read-BridgeAssignment $Identity
        }

        "Delete" {
            try {
                Remove-ManagementRoleAssignment -Identity $Identity
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("ManagementRoleAssignment $Action failed: $($_.Exception.Message)")
    exit 1
}
