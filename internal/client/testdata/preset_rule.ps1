# preset_rule.ps1 - runs bridge/scripts/eop_protection_policy_rule.ps1 against
# fake preset security policy rule cmdlets (run by rule_scripts_test.go): the
# rule is adopted, never created, and destroying it turns the preset off.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
$global:Rule = $null
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
function global:Get-EOPProtectionPolicyRule {
    param($Identity)
    if (-not $global:Rule) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    $global:Rule
}
function global:New-EOPProtectionPolicyRule { $global:Calls.Add("New") }
function global:Remove-EOPProtectionPolicyRule { $global:Calls.Add("Remove") }
function global:Set-EOPProtectionPolicyRule {
    param($Identity, $SentToMemberOf, $Comments)
    $set = $PSBoundParameters.Keys | Where-Object { $_ -ne "Identity" } | Sort-Object
    $global:Calls.Add("Set " + ($set -join ","))
    foreach ($k in $set) { $global:Rule.$k = $PSBoundParameters[$k] }
}
function global:Enable-EOPProtectionPolicyRule { param($Identity) $global:Calls.Add("Enable"); $global:Rule.State = "Enabled" }
function global:Disable-EOPProtectionPolicyRule { param($Identity) $global:Calls.Add("Disable"); $global:Rule.State = "Disabled" }

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject $Object)))
}
$name = "Standard Preset Security Policy"
$props = ConvertTo-B64 @("SentToMemberOf", "Enabled")
$values = ConvertTo-B64 @{ SentToMemberOf = @("Pilot"); Enabled = $true }

# Not turned on yet in the portal: no rule to adopt.
$out = & $Script -Action Create -Identity $name -Values $values -Properties $props 2>&1
"missing: $LASTEXITCODE $out"

$global:Rule = [pscustomobject]@{ Name = $name; State = "Disabled"; SentToMemberOf = $null; Comments = $null }
"create: " + (& $Script -Action Create -Identity $name -Values $values -Properties $props)
"create calls: " + ($global:Calls -join "; ")
$global:Calls.Clear()
"delete: " + (& $Script -Action Delete -Identity $name)
"delete calls: " + ($global:Calls -join "; ") + " | state " + $global:Rule.State
