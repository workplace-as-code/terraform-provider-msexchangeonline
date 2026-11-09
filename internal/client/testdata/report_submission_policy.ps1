# report_submission_policy.ps1 - runs bridge/scripts/report_submission_policy.ps1
# against fake cmdlets (run by report_submission_scripts_test.go).
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Calls = [System.Collections.Generic.List[string]]::new()
$Zero = "00000000-0000-0000-0000-000000000000"
$Real = "22222222-2222-2222-2222-222222222222"
$LangNames = @(foreach ($m in "Pre", "Post") { foreach ($c in "Junk", "NotJunk", "Phishing") {
    if ($m -eq "Post" -and $c -eq "NotJunk") { continue }
    foreach ($k in "Title", "", "ButtonText", "ButtonLink") { "MultiLanguage${m}SubmitMessage${k}For$c" }
} })
function New-Policy([string]$Guid) {
    $h = [ordered]@{ Guid = $Guid; Identity = "x"; Name = "DefaultReportSubmissionPolicy"
        EnableReportToMicrosoft = $true; EnableThirdPartyAddress = $false; NotificationSenderAddress = @()
        ReportJunkAddresses = @(); ReportNotJunkAddresses = @(); ReportPhishAddresses = @(); ThirdPartyReportAddresses = @()
        MultiLanguageSetting = @() }
    foreach ($n in $LangNames) { $h[$n] = @() }
    [pscustomobject]$h
}
function global:Get-ReportSubmissionPolicy { $global:Policy }
function global:New-ReportSubmissionPolicy { $global:Calls.Add("New policy"); $global:Policy.Guid = "11111111-1111-1111-1111-111111111111" }
function global:Set-ReportSubmissionPolicy {
    [CmdletBinding()] param([Parameter(ValueFromRemainingArguments)] $Rest)
    $parts = @()
    for ($i = 0; $i -lt $Rest.Count; $i += 2) {
        $k = ([string]$Rest[$i]).Trim('-', ':'); $v = $Rest[$i + 1]
        if ($k -eq "Identity") { if ($v -ne "DefaultReportSubmissionPolicy") { throw "wrong identity $v" }; continue }
        if ($null -eq $v) { $parts += "$k=<null>" } else { $parts += "$k=$(@($v) -join ',')" }
    }
    $global:Calls.Add("Set " + (($parts | Sort-Object) -join ' '))
}
function global:Get-ReportSubmissionRule { [CmdletBinding()] param() $global:Rule }
function global:New-ReportSubmissionRule {
    param($Name, $ReportSubmissionPolicy, $SentTo)
    $global:Calls.Add("New rule $Name $ReportSubmissionPolicy $SentTo")
}
function global:Set-ReportSubmissionRule { param($Identity, $SentTo); $global:Calls.Add("Set rule $Identity $SentTo") }
function global:Remove-ReportSubmissionRule { param($Identity); $global:Calls.Add("Remove rule $Identity") }
function global:Get-Recipient {
    [CmdletBinding()] param($Identity)
    [pscustomobject]@{ PrimarySmtpAddress = $(if ($Identity -eq "alias@contoso.com") { "primary@contoso.com" } else { $Identity }) }
}
function ConvertTo-B64($v) { [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -Depth 5 -InputObject $v))) }
function Rule($to) { [pscustomobject]@{ Name = "DefaultReportSubmissionRule"; SentTo = [System.Collections.ArrayList]@($to) } }
function Step($label, $guid, $rule, $values, $extra) {
    $global:Policy = New-Policy $guid; $global:Rule = $rule; $global:Calls.Clear()
    $p = @{ Action = "Update"; Values = (ConvertTo-B64 $values) } + $extra
    & $Script @p | Out-Null
    "$label calls: $($global:Calls -join '; ')"
}
$notify = @{ EnableUserEmailNotification = $true }
Step "virtual" $Zero $null $notify @{}
Step "real" $Real $null $notify @{}
Step "microsoft" $Real $null @{} @{ ReportingMailbox = "a@contoso.com" }
Step "thirdparty" $Real $null @{ EnableThirdPartyAddress = $true; EnableReportToMicrosoft = $false } @{ ReportingMailbox = "a@contoso.com" }
Step "existing" $Real (Rule "a@contoso.com") @{} @{ ReportingMailbox = "b@contoso.com" }
Step "none" $Real (Rule "a@contoso.com") @{} @{ ReportingMailbox = "" }
Step "languages" $Real $null @{} @{ Languages = (ConvertTo-B64 @(
    @{ Language = "fr"; MultiLanguagePreSubmitMessageForJunk = "d'assistance" },
    @{ Language = "en"; MultiLanguagePreSubmitMessageForJunk = "" }
)) }

function Read($label, $rule, $addresses, $expected) {
    $global:Policy = New-Policy $Real; $global:Rule = $rule
    $global:Policy.ReportJunkAddresses = $addresses; $global:Policy.ReportNotJunkAddresses = $addresses; $global:Policy.ReportPhishAddresses = $addresses
    $p = @{ Action = "Get" }; if ($expected) { $p["ExpectedMailbox"] = $expected }
    $out = & $Script @p | ConvertFrom-Json
    "$label mailbox: [$($out.ReportingMailbox)]"
}
Read "gone" $null @("a@contoso.com") "a@contoso.com"
Read "differs" (Rule "a@contoso.com") @("b@contoso.com") "a@contoso.com"
Read "alias" (Rule "primary@contoso.com") @("alias@contoso.com") "alias@contoso.com"
Read "rulehand" (Rule "c@contoso.com") @("a@contoso.com") "a@contoso.com"
Read "import" (Rule "a@contoso.com") @("a@contoso.com") ""

# A mailbox the configuration no longer manages stays compared with the state.
$global:Policy = New-Policy $Real; $global:Rule = Rule "primary@contoso.com"
foreach ($l in "ReportJunkAddresses", "ReportNotJunkAddresses", "ReportPhishAddresses") { $global:Policy.$l = @("alias@contoso.com") }
"unmanaged mailbox: [" + (& $Script -Action Update -Values (ConvertTo-B64 $notify) -ExpectedMailbox "alias@contoso.com" | ConvertFrom-Json).ReportingMailbox + "]"

$global:Policy = New-Policy $Real; $global:Rule = $null
$global:Policy.MultiLanguageSetting = @("en", "fr")
foreach ($n in $LangNames) { $global:Policy.$n = @("", "") }
$global:Policy.MultiLanguagePreSubmitMessageForJunk = @("Hello", "Détails")
"get languages: " + (& $Script -Action Get)
