# list.ps1 — Lists the import IDs of the existing objects of one type, for the
# list resources (terraform query).
#
#   -Noun      cmdlet noun, e.g. "TransportRule": Get-<noun> returns the objects
#   -Property  property holding the import ID (Identity, Name, or AppId for
#              service principals)
#
# Output: JSON array of strings.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidatePattern('^[A-Za-z]+$')] [string]$Noun,
    [ValidateSet("Identity", "Name", "AppId")] [string]$Property = "Identity"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

try {
    $ids = @(
        if ($Noun -eq "TenantAllowBlockListItems") {
            # Each list is read separately; an entry is imported as "<list_type>:<value>".
            foreach ($listType in "Sender", "Url", "FileHash", "IP") {
                foreach ($item in @(Invoke-BridgeRetry { Get-TenantAllowBlockListItems -ListType $listType })) {
                    $value = [string](Get-BridgeProp $item "Value" "")
                    if ($value) { "${listType}:$value" }
                }
            }
        } elseif ($Noun -eq "TenantAllowBlockListSpoofItems") {
            # Get- lists the entries only without -Identity. An entry is imported
            # as "<spoof_type>:<spoofed_user>:<sending_infrastructure>".
            foreach ($item in @(Invoke-BridgeRetry { @(Get-TenantAllowBlockListSpoofItems) })) {
                "$($item.SpoofType):$($item.SpoofedUser):$($item.SendingInfrastructure)"
            }
        } elseif ($Noun -eq "CrossTenantPartner") {
            # Microsoft Graph: the partners of the cross-tenant access policy
            # that have at least one Exchange capability.
            $exchange = @(Get-BridgeExchangeCapabilities)
            $partners = 0
            $found = 0
            $next = "/v1.0/policies/crossTenantAccessPolicy/partners?`$select=tenantId"
            while ($next) {
                $page = try { Invoke-BridgeGraph -Method GET -Path $next } catch {
                    if ($_.Exception.Message -match '^Graph 403') {
                        throw "Listing cross-tenant partners needs the Microsoft Graph application permission Policy.Read.All ($($_.Exception.Message))"
                    }
                    throw
                }
                foreach ($partner in @($page.value)) {
                    $id = ([string]$partner.tenantId).ToLowerInvariant()
                    $partners++
                    $caps = Invoke-BridgeGraph -Method GET -Path "/v1.0/policies/crossTenantAccessPolicy/partners/$id/m365Capabilities"
                    if (@(@($caps.value) | Where-Object { $exchange -ccontains [string]$_.name }).Count -gt 0) { $found++; $id }
                }
                $link = Get-BridgeProp $page '@odata.nextLink' ""
                $next = if ($link) { ([uri]$link).PathAndQuery } else { "" }
            }
            # Without the Entra role Graph lists no capability for any partner:
            # a read by name tells it from partners that really have none.
            if ($partners -gt 0 -and $found -eq 0) {
                try {
                    Invoke-BridgeGraph -Method GET -Path "/v1.0/policies/crossTenantAccessPolicy/partners/$id/m365Capabilities/$($exchange[0])" | Out-Null
                } catch {
                    if ($_.Exception.Message -match '^Graph 403 .*administrative role') {
                        throw "Listing cross-tenant partners needs the Microsoft Entra role Exchange Administrator ($($_.Exception.Message))"
                    }
                    if ($_.Exception.Message -notmatch '^Graph 404') { throw }
                }
            }
        } else {
            $cmdlet = Get-Command "Get-$Noun"
            # Some Get cmdlets return the first 1000 objects only by default.
            $size = if ($cmdlet.Parameters.ContainsKey("ResultSize")) { @{ ResultSize = "Unlimited" } } else { @{} }
            # Get-ApplicationAccessPolicy reports "couldn't be found" when there is none.
            $objects = try { @(Invoke-BridgeRetry { & "Get-$Noun" @size }) } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
                @()
            }
            foreach ($object in $objects) {
                # terraform query lists custom RBAC objects only.
                if (Test-BridgeBuiltIn $Noun $object) { continue }
                [string](Get-BridgeProp $object $Property "")
            }
        }
    )
    ConvertTo-Json -Compress -InputObject ([string[]]@($ids | Where-Object { $_ }))
} catch {
    [Console]::Error.WriteLine("Listing $Noun failed: $($_.Exception.Message)")
    exit 1
}
