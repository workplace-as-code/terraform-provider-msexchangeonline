# ome_configuration.ps1 - runs bridge/scripts/ome_configuration.ps1 against fake
# branding template cmdlets (run by rbac_scripts_test.go): the default template
# is adopted (never created nor removed), a custom template is created with
# -Identity, the logo travels as base64 and the expiry comes back in days.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Templates = @{ "OME Configuration" = [pscustomobject]@{ Identity = "OME Configuration"; DisclaimerText = ""
    Image = $null; ExternalMailExpiryInterval = [timespan]::Zero; OTPEnabled = $true } }
function global:Get-OMEConfiguration {
    param($Identity)
    if (-not $global:Templates.ContainsKey($Identity)) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    $global:Templates[$Identity]
}
function global:Save-Template($Verb, $Bound) {
    $id = $Bound["Identity"]
    $keys = $Bound.Keys | Where-Object { $_ -ne "Identity" } | Sort-Object
    $global:Calls.Add("$Verb $id " + (($keys | ForEach-Object { "$_=$($Bound[$_].GetType().Name)" }) -join ","))
    if (-not $global:Templates.ContainsKey($id)) {
        $global:Templates[$id] = [pscustomobject]@{ Identity = $id; DisclaimerText = ""; Image = $null
            ExternalMailExpiryInterval = [timespan]::Zero; OTPEnabled = $true }
    }
    $t = $global:Templates[$id]
    foreach ($k in $keys) {
        if ($k -eq "ExternalMailExpiryInDays") { $t.ExternalMailExpiryInterval = [timespan]::FromDays($Bound[$k]) }
        else { $t.$k = $Bound[$k] }
    }
}
function global:New-OMEConfiguration { param($Identity, $DisclaimerText, [byte[]]$Image, $ExternalMailExpiryInDays) Save-Template "New" $PSBoundParameters }
function global:Set-OMEConfiguration { param($Identity, $DisclaimerText, [byte[]]$Image, $ExternalMailExpiryInDays) Save-Template "Set" $PSBoundParameters }
function global:Remove-OMEConfiguration { param($Identity) $global:Calls.Add("Remove $Identity") }

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject $Object)))
}
$props = ConvertTo-B64 @("DisclaimerText", "Image", "ExternalMailExpiryInDays", "OTPEnabled")
$logo = [Convert]::ToBase64String([byte[]](137, 80, 78, 71))
"default: " + (& $Script -Action Create -Noun OMEConfiguration -Identity "OME Configuration" -Values (ConvertTo-B64 @{ DisclaimerText = "Confidential" }) -Properties $props)
"default delete: " + (& $Script -Action Delete -Noun OMEConfiguration -Identity "OME Configuration")
"custom: " + (& $Script -Action Create -Noun OMEConfiguration -Identity "TF-CI-OME" -Values (ConvertTo-B64 @{ Image = $logo; ExternalMailExpiryInDays = 7 }) -Properties $props)
"custom delete: " + (& $Script -Action Delete -Noun OMEConfiguration -Identity "TF-CI-OME")
"calls: " + ($global:Calls -join "; ")
