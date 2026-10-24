# list.ps1 — Lists the import IDs of the existing objects of one type, for the
# list resources (terraform query).
#
#   -Noun      cmdlet noun, e.g. "TransportRule": Get-<noun> returns the objects
#   -Property  property holding the import ID (Identity or Name)
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
    [ValidateSet("Identity", "Name")] [string]$Property = "Identity"
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
                [string](Get-BridgeProp $object $Property "")
            }
        }
    )
    ConvertTo-Json -Compress -InputObject ([string[]]@($ids | Where-Object { $_ }))
} catch {
    [Console]::Error.WriteLine("Listing $Noun failed: $($_.Exception.Message)")
    exit 1
}
