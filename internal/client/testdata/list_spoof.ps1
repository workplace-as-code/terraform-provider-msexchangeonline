# list_spoof.ps1 - runs bridge/scripts/list.ps1 against a fake spoof item list
# (run by spoof_scripts_test.go).
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
function global:Get-TenantAllowBlockListSpoofItems {
    param($Identity)
    if ($PSBoundParameters.ContainsKey("Identity")) { return }
    [pscustomobject]@{ Identity = "guid-1"; SpoofedUser = "contoso.com"; SendingInfrastructure = "192.0.2.1/24"; SpoofType = "External" }
    [pscustomobject]@{ Identity = "guid-2"; SpoofedUser = "bob@contoso.com"; SendingInfrastructure = "mail.contoso.com"; SpoofType = "Internal" }
}
"ids: " + (& $Script -Noun TenantAllowBlockListSpoofItems)
