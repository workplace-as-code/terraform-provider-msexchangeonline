# quarantine_global_settings.ps1 - runs bridge/scripts/quarantine_global_settings.ps1
# against fake cmdlets (run by quarantine_scripts_test.go).
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Calls = [System.Collections.Generic.List[string]]::new()
$Zero = "00000000-0000-0000-0000-000000000000"
function New-Fake([string]$Guid) {
    [pscustomobject]@{
        Guid = $Guid; Identity = "x"; Name = "x"
        EndUserSpamNotificationFrequency = "1.00:00:00"; EndUserSpamNotificationCustomFromAddress = ""
        OrganizationBrandingEnabled = $false
        MultiLanguageSetting = @(); MultiLanguageSenderName = @(); EsnCustomSubject = @(); MultiLanguageCustomDisclaimer = @()
    }
}
function global:Get-QuarantinePolicy {
    param($QuarantinePolicyType)
    if ($QuarantinePolicyType -ne "GlobalQuarantinePolicy") { throw "must read by type" }
    $global:Policy
}
function global:New-QuarantinePolicy {
    param($Name, $QuarantinePolicyType)
    $global:Calls.Add("New $Name $QuarantinePolicyType")
    $global:Policy.Guid = "11111111-1111-1111-1111-111111111111"
}
function global:Set-QuarantinePolicy {
    param(
        [Parameter(ValueFromPipeline)] $InputObject,
        $EndUserSpamNotificationFrequency, $EndUserSpamNotificationCustomFromAddress, $OrganizationBrandingEnabled,
        $MultiLanguageSetting, $MultiLanguageSenderName, $EsnCustomSubject, $MultiLanguageCustomDisclaimer
    )
    process {
        if ($InputObject.Guid -eq "00000000-0000-0000-0000-000000000000") { throw "object couldn't be found" }
        $parts = foreach ($k in ($PSBoundParameters.Keys | Where-Object { $_ -ne "InputObject" } | Sort-Object)) {
            $v = $PSBoundParameters[$k]
            if ($null -eq $v) { "$k=<null>" }
            elseif ($v -is [array] -and $v.Count -eq 0) { "$k=<empty>" }
            else { "$k=$($v -join ',')" }
        }
        $global:Calls.Add("Set $($parts -join ' ')")
    }
}
function ConvertTo-B64($v) { [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -Depth 5 -InputObject $v))) }
function Step($label, $guid, $values, $languages) {
    $global:Policy = New-Fake $guid
    $global:Calls.Clear()
    $p = @{ Action = "Update"; Values = (ConvertTo-B64 $values) }
    if ($null -ne $languages) { $p["Languages"] = ConvertTo-B64 $languages }
    & $Script @p | Out-Null
    "$label calls: $($global:Calls -join '; ')"
}

$freq = @{ EndUserSpamNotificationFrequency = "04:00:00" }
Step "virtual" $Zero $freq $null
Step "real" "22222222-2222-2222-2222-222222222222" $freq $null
Step "languages" "22222222-2222-2222-2222-222222222222" @{} @(
    @{ Language = "French"; SenderName = "Détails"; Subject = "Objet"; Disclaimer = "d'assistance" },
    @{ Language = "English"; SenderName = "Sender"; Subject = ""; Disclaimer = "Help" }
)
Step "clear" "22222222-2222-2222-2222-222222222222" @{ EndUserSpamNotificationCustomFromAddress = "" } @()

$global:Policy = New-Fake $Zero
"get virtual: " + (& $Script -Action Get)
$global:Policy = New-Fake "22222222-2222-2222-2222-222222222222"
$global:Policy.MultiLanguageSetting = @("English", "French")
$global:Policy.MultiLanguageSenderName = @("Sender", "Détails")
$global:Policy.EsnCustomSubject = @("", "Objet")
$global:Policy.MultiLanguageCustomDisclaimer = @("Help", "d'assistance")
"get languages: " + (& $Script -Action Get)
