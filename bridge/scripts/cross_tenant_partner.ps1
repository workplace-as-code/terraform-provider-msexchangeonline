# cross_tenant_partner.ps1 - the Exchange capabilities of the Microsoft 365
# cross-tenant access policy of one partner tenant (Microsoft Graph
# /policies/crossTenantAccessPolicy/partners/{id}/m365Capabilities).
#
# Only the Exchange capabilities (calendar availability, MailTips, calendar
# sharing) are read and changed: the others (profile card, Places, migration)
# are never touched. The resource owns all the Exchange capabilities of the
# partner: Update removes those left out of the configuration.
#
#   -PartnerTenantId  tenant ID of the partner
#   -Capabilities     base64 JSON array of {name, allowed, includedGroups,
#                     excludedGroups, includedUsers, excludedUsers}
#   -Known            base64 JSON array of the capability names in the state
#
# Graph answers an empty list, not an error, when the app lacks the Entra role
# Exchange Administrator: an empty list is only "gone" once a capability read
# by name answers 404.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,
    [Parameter(Mandatory)] [ValidatePattern('^[0-9a-fA-F-]{36}$')] [string]$PartnerTenantId,
    [string]$Capabilities = "",
    [string]$Known        = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$ExchangeCapabilities = @(Get-BridgeExchangeCapabilities)
$base = "/v1.0/policies/crossTenantAccessPolicy/partners/$($PartnerTenantId.ToLowerInvariant())/m365Capabilities"

# Invoke-PartnerGraph adds what to fix when Graph refuses for want of the role.
function Invoke-PartnerGraph([string]$Method, [string]$Path, $Body = $null) {
    try {
        Invoke-BridgeGraph -Method $Method -Path $Path -Body $Body
    } catch {
        if ($_.Exception.Message -match '^Graph 403 .*administrative role') {
            throw "The app needs the Microsoft Entra role Exchange Administrator to manage the cross-tenant capabilities of partner $PartnerTenantId ($($_.Exception.Message))"
        }
        throw
    }
}

# Get-PartnerCapabilities returns the Exchange capabilities of the partner.
function Get-PartnerCapabilities {
    $list = Invoke-PartnerGraph GET $base
    @(@($list.value) | Where-Object { $ExchangeCapabilities -ccontains [string]$_.name })
}

# Test-PartnerRole returns only when a capability read by name answers 404:
# without the Entra role the list is empty and only a read by name says why
# (403), and a capability found by name means the list cannot be trusted.
function Test-PartnerRole([string[]]$Names) {
    $probe = @($Names | Where-Object { $_ }) + @($ExchangeCapabilities[0]) | Select-Object -First 1
    try { Invoke-PartnerGraph GET "$base/$probe" | Out-Null } catch {
        if ($_.Exception.Message -match '^Graph 404') { return }
        throw
    }
    throw "Microsoft Graph listed no Exchange capability for partner $PartnerTenantId, but $probe exists: retry later"
}

# The "All" scope uses the resource type Graph stores for the capability.
function Get-AllType([string]$Name) { if ($Name -like "crossTenantCalendarSharing*") { "group" } else { "user" } }

function ConvertTo-PartnerBody($Desired) {
    $included = @(@($Desired.includedGroups) | ForEach-Object { [ordered]@{ resourceId = "$_".ToLowerInvariant(); resourceType = "group" } }) +
                @(@($Desired.includedUsers)  | ForEach-Object { [ordered]@{ resourceId = "$_".ToLowerInvariant(); resourceType = "user" } })
    if ($included.Count -eq 0) { $included = @([ordered]@{ resourceId = "All"; resourceType = (Get-AllType $Desired.name) }) }
    $excluded = @(@($Desired.excludedGroups) | ForEach-Object { [ordered]@{ resourceId = "$_".ToLowerInvariant(); resourceType = "group" } }) +
                @(@($Desired.excludedUsers)  | ForEach-Object { [ordered]@{ resourceId = "$_".ToLowerInvariant(); resourceType = "user" } })
    [ordered]@{
        "@odata.type" = "#microsoft.graph.$($Desired.name)"
        inboundAccess = [ordered]@{ isAllowed = [bool]$Desired.allowed; resourceScopes = [ordered]@{ included = @($included); excluded = @($excluded) } }
    }
}

# ConvertTo-PartnerOutput is the state form: an "All" scope is no included set.
function ConvertTo-PartnerOutput($Capability) {
    $scopes = $Capability.inboundAccess.resourceScopes
    $ids = {
        param($List, [string]$Type)
        @(@($List) | Where-Object { $_ -and [string]$_.resourceType -eq $Type -and [string]$_.resourceId -ne "All" } |
            ForEach-Object { ([string]$_.resourceId).ToLowerInvariant() } | Sort-Object)
    }
    [ordered]@{
        name           = [string]$Capability.name
        allowed        = [bool]$Capability.inboundAccess.isAllowed
        includedGroups = @(& $ids $scopes.included "group")
        excludedGroups = @(& $ids $scopes.excluded "group")
        includedUsers  = @(& $ids $scopes.included "user")
        excludedUsers  = @(& $ids $scopes.excluded "user")
    }
}

function Write-PartnerState($Current) {
    $out = [ordered]@{
        PartnerTenantId = $PartnerTenantId.ToLowerInvariant()
        Capabilities    = @(@($Current) | Sort-Object { [string]$_.name } | ForEach-Object { ConvertTo-PartnerOutput $_ })
    }
    ConvertTo-Json -InputObject $out -Depth 6 -Compress
}

try {
    $knownNames = @(ConvertFrom-BridgeList $Known)
    $desired = @(ConvertFrom-BridgeList $Capabilities)
    switch ($Action) {
        "Get" {
            $current = @(Get-PartnerCapabilities)
            $attempt = 0
            while ($current.Count -eq 0) {
                try {
                    Test-PartnerRole $knownNames
                    exit 0   # no Exchange capability: gone
                } catch {
                    # The list can lag behind a write: read it again before failing.
                    if ($_.Exception.Message -notmatch 'retry later$' -or ++$attempt -ge 6) { throw }
                }
                Start-Sleep -Seconds (5 * $attempt)
                $current = @(Get-PartnerCapabilities)
            }
            Write-PartnerState $current
        }
        "Create" {
            $current = @(Get-PartnerCapabilities)
            if ($current.Count -gt 0) {
                throw "Partner $PartnerTenantId already has Exchange capabilities ($((@($current | ForEach-Object { $_.name }) | Sort-Object) -join ', ')): import it"
            }
            try {
                foreach ($d in $desired) { Invoke-PartnerGraph POST $base (ConvertTo-PartnerBody $d) | Out-Null }
            } catch {
                # Remove what this create added, so that the next apply creates
                # the partner again instead of asking for an import. Every
                # wanted capability is removed by name: the partner had none,
                # and a POST that got no answer may have created one.
                $failure = $_.Exception.Message
                $left = @(foreach ($d in $desired) {
                    try { Invoke-PartnerGraph DELETE "$base/$($d.name)" | Out-Null } catch {
                        if ($_.Exception.Message -notmatch '^Graph 404') { [string]$d.name }
                    }
                })
                if ($left.Count -gt 0) { $failure += " (could not remove $($left -join ', '): import the partner)" }
                throw $failure
            }
            Write-PartnerState @(Get-PartnerCapabilities)
        }
        "Update" {
            $current = @(Get-PartnerCapabilities)
            $byName = @{}
            foreach ($c in $current) { $byName[[string]$c.name] = $c }
            foreach ($c in $current) {
                if (@($desired | Where-Object { $_.name -ceq $c.name }).Count -eq 0) { Invoke-PartnerGraph DELETE "$base/$($c.name)" | Out-Null }
            }
            foreach ($d in $desired) {
                $body = ConvertTo-PartnerBody $d
                if (-not $byName.ContainsKey([string]$d.name)) {
                    Invoke-PartnerGraph POST $base $body | Out-Null
                } elseif ((ConvertTo-Json -InputObject (ConvertTo-PartnerOutput $byName[[string]$d.name]) -Compress) -ne
                          (ConvertTo-Json -InputObject (ConvertTo-PartnerOutput ([pscustomobject]@{ name = $d.name; inboundAccess = $body.inboundAccess })) -Compress)) {
                    Invoke-PartnerGraph PATCH "$base/$($d.name)" $body | Out-Null
                }
            }
            Write-PartnerState @(Get-PartnerCapabilities)
        }
        "Delete" {
            $current = @(Get-PartnerCapabilities)
            if ($current.Count -eq 0) { Test-PartnerRole $knownNames }
            foreach ($c in $current) { Invoke-PartnerGraph DELETE "$base/$($c.name)" | Out-Null }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("CrossTenantPartner $Action failed: $($_.Exception.Message)")
    exit 1
}
