# hosted_content_filter_policy.ps1 - runs bridge/scripts/hosted_content_filter_policy.ps1
# against fake cmdlets (run by policy_scripts_test.go): the quarantine policy for
# spam is the SpamQuarantineTag parameter and property.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Sent = [System.Collections.Generic.List[string]]::new()
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
function global:Get-HostedContentFilterPolicy {
    [CmdletBinding()] param($Identity)
    [pscustomobject]@{ Identity = $Identity; Name = $Identity; SpamQuarantineTag = "AdminOnlyAccessPolicy" }
}
function global:Get-HostedContentFilterRule { [CmdletBinding()] param($Identity) }
function global:New-HostedContentFilterPolicy {
    param([Parameter(ValueFromRemainingArguments)] $Rest)
    $global:Sent.Add("new: $($Rest -join ' ')")
}
function global:Set-HostedContentFilterPolicy {
    param([Parameter(ValueFromRemainingArguments)] $Rest)
    $global:Sent.Add("set: $($Rest -join ' ')")
}

& $Script -Action Update -Identity "TF-CI" -QuarantineTag "AdminOnlyAccessPolicy"
& $Script -Action Create -Name "TF-CI" -QuarantineTag "AdminOnlyAccessPolicy"
$global:Sent
