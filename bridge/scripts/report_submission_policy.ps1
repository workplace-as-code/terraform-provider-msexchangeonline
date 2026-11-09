# report_submission_policy.ps1 — Reads and writes how users report messages: the
# report submission policy and its rule, kept in step. A tenant never configured
# only has a virtual policy (zero Guid): create it first.
#   -Values            base64 JSON object: cmdlet parameter -> value
#   -ReportingMailbox  absent = unmanaged, "" = no mailbox
#   -Languages         base64 JSON array of objects: Language and MultiLanguage* lists
#   -ExpectedMailbox   the mailbox in the state, for reads

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Update")] [string]$Action,

    [string]$Values = "",
    [AllowEmptyString()] [string]$ReportingMailbox,
    [string]$Languages,
    [string]$ExpectedMailbox = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$PolicyIdentity = "DefaultReportSubmissionPolicy"
$RuleName = "DefaultReportSubmissionRule"
$BoolProperties = "EnableReportToMicrosoft", "EnableThirdPartyAddress", "DisableQuarantineReportingOption",
    "EnableUserEmailNotification", "EnableCustomNotificationSender", "EnableOrganizationBranding",
    "PreSubmitMessageEnabled", "PostSubmitMessageEnabled"
$TextProperties = "NotificationFooterMessage", "JunkReviewResultMessage", "NotJunkReviewResultMessage",
    "PhishingReviewResultMessage", "PreSubmitMessage", "PreSubmitMessageTitle", "PostSubmitMessage", "PostSubmitMessageTitle"
# The lists of the customized messages, by position in MultiLanguageSetting.
$LanguageLists = "MultiLanguagePreSubmitMessageTitleForJunk", "MultiLanguagePreSubmitMessageForJunk",
    "MultiLanguagePreSubmitMessageButtonTextForJunk", "MultiLanguagePreSubmitMessageButtonLinkForJunk",
    "MultiLanguagePreSubmitMessageTitleForNotJunk", "MultiLanguagePreSubmitMessageForNotJunk",
    "MultiLanguagePreSubmitMessageButtonTextForNotJunk", "MultiLanguagePreSubmitMessageButtonLinkForNotJunk",
    "MultiLanguagePreSubmitMessageTitleForPhishing", "MultiLanguagePreSubmitMessageForPhishing",
    "MultiLanguagePreSubmitMessageButtonTextForPhishing", "MultiLanguagePreSubmitMessageButtonLinkForPhishing",
    "MultiLanguagePostSubmitMessageTitleForJunk", "MultiLanguagePostSubmitMessageForJunk",
    "MultiLanguagePostSubmitMessageButtonTextForJunk", "MultiLanguagePostSubmitMessageButtonLinkForJunk",
    "MultiLanguagePostSubmitMessageTitleForPhishing", "MultiLanguagePostSubmitMessageForPhishing",
    "MultiLanguagePostSubmitMessageButtonTextForPhishing", "MultiLanguagePostSubmitMessageButtonLinkForPhishing"
$CustomizedAddressFlags = "ReportJunkToCustomizedAddress", "ReportNotJunkToCustomizedAddress", "ReportPhishToCustomizedAddress"
$MicrosoftAddressLists = "ReportJunkAddresses", "ReportNotJunkAddresses", "ReportPhishAddresses"

function Get-BridgePolicy { Invoke-BridgeRetry { @(Get-ReportSubmissionPolicy) | Select-Object -First 1 } }
function Get-BridgeSubmissionRule { Invoke-BridgeRetry { @(Get-ReportSubmissionRule) | Select-Object -First 1 } }

function Get-BridgeStrings {
    param($Values)
    @($Values | Where-Object { $_ } | ForEach-Object { [string]$_ })
}

function Test-BridgeSameMailbox {
    param([string]$Value, [string]$Expected)
    if ($Value -eq $Expected) { return $true }
    # The rule reads back as the primary address of the mailbox.
    try { return [string](Get-Recipient -Identity $Expected -ErrorAction Stop).PrimarySmtpAddress -eq $Value }
    catch { return $false }
}

# Get-BridgeReportingMailbox returns the mailbox for the state: the expected one
# when the rule and the policy agree with it, otherwise the value that differs,
# so the plan shows a change and the next apply realigns them.
function Get-BridgeReportingMailbox {
    param($Policy, $Rule, [string]$Expected)
    if ($null -eq $Rule) { return "" }
    $ruleAddress = @(Get-BridgeStrings (Get-BridgeProp $Rule "SentTo"))
    if ($ruleAddress.Count -eq 0) { return "" }
    if (-not $Expected) { return $ruleAddress[0] }
    $lists = if ([bool](Get-BridgeProp $Policy "EnableThirdPartyAddress" $false)) { @("ThirdPartyReportAddresses") } else { $MicrosoftAddressLists }
    $addresses = @(foreach ($l in $lists) { Get-BridgeStrings (Get-BridgeProp $Policy $l) })
    if ($addresses.Count -eq 0) { return "" }
    foreach ($value in @($ruleAddress[0]) + $addresses) {
        if (-not (Test-BridgeSameMailbox $value $Expected)) { return $value }
    }
    $Expected
}

function Write-BridgePolicy {
    param($Policy, $Rule, [string]$Expected)
    $out = [ordered]@{}
    foreach ($p in $BoolProperties) { $out[$p] = [bool](Get-BridgeProp $Policy $p $false) }
    foreach ($p in $TextProperties) { $out[$p] = [string](Get-BridgeProp $Policy $p "") }
    $out["NotificationSenderAddress"] = [string](@(Get-BridgeStrings (Get-BridgeProp $Policy "NotificationSenderAddress")) | Select-Object -First 1)
    $out["ReportingMailbox"] = [string](Get-BridgeReportingMailbox $Policy $Rule $Expected)
    $names = @(Get-BridgeStrings (Get-BridgeProp $Policy "MultiLanguageSetting"))
    $out["Languages"] = [object[]]@(for ($i = 0; $i -lt $names.Count; $i++) {
        $language = [ordered]@{ Language = $names[$i] }
        foreach ($l in $LanguageLists) {
            $values = @(Get-BridgeProp $Policy $l)
            $language[$l] = if ($i -lt $values.Count -and $null -ne $values[$i]) { [string]$values[$i] } else { "" }
        }
        $language
    })
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" { Write-BridgePolicy (Get-BridgePolicy) (Get-BridgeSubmissionRule) $ExpectedMailbox }

        "Update" {
            $splat = ConvertFrom-BridgeValues $Values
            if ($splat.ContainsKey("NotificationSenderAddress") -and -not $splat["NotificationSenderAddress"]) { $splat["NotificationSenderAddress"] = $null }
            $policy = Get-BridgePolicy
            if ([string](Get-BridgeProp $policy "Guid" "") -in "", "00000000-0000-0000-0000-000000000000") {
                Invoke-BridgeRetry -Create { New-ReportSubmissionPolicy | Out-Null }
                $policy = Get-BridgePolicy
            }

            if ($PSBoundParameters.ContainsKey("ReportingMailbox")) {
                $thirdParty = if ($splat.ContainsKey("EnableThirdPartyAddress")) { [bool]$splat["EnableThirdPartyAddress"] }
                              else { [bool](Get-BridgeProp $policy "EnableThirdPartyAddress" $false) }
                $useMicrosoftLists = $ReportingMailbox -and -not $thirdParty
                $useThirdPartyList = $ReportingMailbox -and $thirdParty
                foreach ($f in $CustomizedAddressFlags) { $splat[$f] = [bool]$useMicrosoftLists }
                foreach ($l in $MicrosoftAddressLists) { $splat[$l] = if ($useMicrosoftLists) { [string[]]@($ReportingMailbox) } else { $null } }
                $splat["ThirdPartyReportAddresses"] = if ($useThirdPartyList) { [string[]]@($ReportingMailbox) } else { $null }
            }

            if ($PSBoundParameters.ContainsKey("Languages")) {
                # Set- merges each list by position, and a language put back
                # recovers its old text: every list is always written whole.
                $wanted = @(ConvertFrom-BridgeJson $Languages | Where-Object { $null -ne $_ } | Sort-Object Language)
                if ($wanted.Count -eq 0) { throw "Exchange Online cannot remove the last language of the report submission policy." }
                $splat["MultiLanguageSetting"] = [string[]]@($wanted | ForEach-Object { $_.Language })
                foreach ($l in $LanguageLists) {
                    $splat[$l] = [string[]]@($wanted | ForEach-Object { [string](Get-BridgeProp $_ $l "") })
                }
            }

            if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-ReportSubmissionPolicy -Identity $PolicyIdentity @splat | Out-Null } }

            if ($PSBoundParameters.ContainsKey("ReportingMailbox")) {
                $rule = Get-BridgeSubmissionRule
                if ($ReportingMailbox) {
                    if ($null -ne $rule) {
                        Invoke-BridgeRetry { Set-ReportSubmissionRule -Identity $RuleName -SentTo $ReportingMailbox | Out-Null }
                    } else {
                        Invoke-BridgeRetry -Create { New-ReportSubmissionRule -Name $RuleName -ReportSubmissionPolicy $PolicyIdentity -SentTo $ReportingMailbox | Out-Null }
                    }
                } elseif ($null -ne $rule) {
                    Invoke-BridgeRetry { Remove-ReportSubmissionRule -Identity $RuleName | Out-Null }
                }
            }
            $expected = if ($PSBoundParameters.ContainsKey("ReportingMailbox")) { $ReportingMailbox } else { $ExpectedMailbox }
            Write-BridgePolicy (Get-BridgePolicy) (Get-BridgeSubmissionRule) $expected
        }
    }
} catch {
    [Console]::Error.WriteLine("report_submission_policy $Action failed: $($_.Exception.Message)")
    exit 1
}
