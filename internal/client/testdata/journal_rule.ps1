# journal_rule.ps1 - runs bridge/scripts/journal_rule.ps1 against fake journal
# rule cmdlets (run by rule_scripts_test.go): the state is read from Enabled,
# and Recipient = "" must clear the recipient instead of being sent as is.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
$global:Rule = $null
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
function global:Get-JournalRule {
    param($Identity)
    if (-not $global:Rule) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    $global:Rule
}
function global:New-JournalRule {
    param($Name, $JournalEmailAddress, $Recipient, $Scope, $Enabled)
    $global:Calls.Add("New " + (($PSBoundParameters.Keys | Sort-Object) -join ","))
    $global:Rule = [pscustomobject]@{
        Name = $Name; JournalEmailAddress = $JournalEmailAddress; Recipient = $Recipient; Scope = $Scope; Enabled = [bool]$Enabled
    }
}
function global:Set-JournalRule {
    param($Identity, $JournalEmailAddress, $Recipient, $Scope)
    $set = $PSBoundParameters.Keys | Where-Object { $_ -ne "Identity" } | Sort-Object
    $global:Calls.Add("Set " + (($set | ForEach-Object { "$_=$($PSBoundParameters[$_])" }) -join ","))
    foreach ($k in $set) { $global:Rule.$k = $PSBoundParameters[$k] }
}
function global:Enable-JournalRule { param($Identity) $global:Calls.Add("Enable"); $global:Rule.Enabled = $true }
function global:Disable-JournalRule { param($Identity) $global:Calls.Add("Disable"); $global:Rule.Enabled = $false }

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject $Object)))
}
$props = ConvertTo-B64 @("JournalEmailAddress", "Recipient", "Scope", "Enabled")

$values = ConvertTo-B64 @{ JournalEmailAddress = "journal@archive.example"; Recipient = ""; Scope = "Global"; Enabled = $false }
"create: " + (& $Script -Action Create -Identity TF-Journal -Values $values -Properties $props)
"create calls: " + ($global:Calls -join "; ")
$global:Calls.Clear()

$global:Rule.Recipient = "user@contoso.com"
$values = ConvertTo-B64 @{ Recipient = ""; Enabled = $true }
"update: " + (& $Script -Action Update -Identity TF-Journal -Values $values -Properties $props)
"update calls: " + ($global:Calls -join "; ")
