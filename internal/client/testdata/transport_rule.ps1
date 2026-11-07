# transport_rule.ps1 - runs bridge/scripts/transport_rule.ps1 against fake
# transport rule cmdlets (run by rule_scripts_test.go): an encryption-only rule
# has no basic action, and an update clears the actions and conditions that
# left the configuration.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
$global:Rule = $null
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$fields = "RejectMessageReasonText", "Quarantine", "SetSCL", "FromAddressContainsWords", "SubjectContainsWords",
    "AttachmentHasExecutableContent", "ApplyRightsProtectionTemplate", "ApplyRightsProtectionCustomizationTemplate",
    "FromScope", "SentToScope", "RecipientDomainIs", "SenderDomainIs", "SubjectOrBodyContainsWords",
    "ExceptIfRecipientDomainIs"
function global:Get-TransportRule { param($Identity, $ErrorAction) $global:Rule }
# The fake cmdlets take a splat: record "name=value" for each parameter.
function global:Save-Params($Verb, $Bound) {
    $keys = $Bound.Keys | Where-Object { $_ -ne "Identity" } | Sort-Object
    $global:Calls.Add("$Verb " + (($keys | ForEach-Object { "$_=$(if ($null -eq $Bound[$_]) { '<null>' } else { $Bound[$_] -join '|' })" }) -join ","))
    if (-not $global:Rule) {
        $global:Rule = [pscustomobject]@{ Identity = $Bound["Name"]; Name = $Bound["Name"]; Priority = 0; State = "Enabled" }
        foreach ($f in $global:Fields) { $global:Rule | Add-Member -NotePropertyName $f -NotePropertyValue $null }
    }
    foreach ($k in $keys) {
        if ($k -in $global:Fields) {
            $v = $Bound[$k]
            # Exchange returns the rights protection template as an object.
            if ($k -eq "ApplyRightsProtectionTemplate" -and $v) { $v = [pscustomobject]@{ TemplateName = $v } }
            $global:Rule.$k = $v
        }
    }
}
$global:Fields = $fields
function global:New-TransportRule {
    param($Name, $Enabled, $Priority, $RejectMessageReasonText, $Quarantine, $SetSCL, $FromAddressContainsWords,
        $SubjectContainsWords, $AttachmentHasExecutableContent, $ApplyRightsProtectionTemplate,
        $ApplyRightsProtectionCustomizationTemplate, $FromScope, $SentToScope, $RecipientDomainIs, $SenderDomainIs,
        $SubjectOrBodyContainsWords, $ExceptIfRecipientDomainIs)
    Save-Params "New" $PSBoundParameters
    $global:Rule
}
function global:Set-TransportRule {
    param($Identity, $Name, $Priority, $RejectMessageReasonText, $Quarantine, $SetSCL, $FromAddressContainsWords,
        $SubjectContainsWords, $AttachmentHasExecutableContent, $ApplyRightsProtectionTemplate,
        $ApplyRightsProtectionCustomizationTemplate, $FromScope, $SentToScope, $RecipientDomainIs, $SenderDomainIs,
        $SubjectOrBodyContainsWords, $ExceptIfRecipientDomainIs)
    Save-Params "Set" $PSBoundParameters
}

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject @($Object))))
}

"create: " + (& $Script -Action Create -Name TF-Encrypt -SentToScope NotInOrganization `
    -ApplyRightsProtectionTemplate Encrypt -ApplyRightsProtectionCustomizationTemplate "OME Configuration")
"create calls: " + ($global:Calls -join "; ")
$global:Calls.Clear()

"update: " + (& $Script -Action Update -Identity TF-Encrypt -Name TF-Encrypt -RuleAction Quarantine `
    -RecipientDomainIs (ConvertTo-B64 @("contoso.com")) -ExceptIfRecipientDomainIs (ConvertTo-B64 @("fabrikam.com")))
"update calls: " + ($global:Calls -join "; ")
