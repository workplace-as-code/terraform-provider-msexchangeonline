# management_role.ps1 - runs bridge/scripts/management_role.ps1 against fake
# role cmdlets (run by rbac_scripts_test.go): role entries are added, removed
# and restricted to match the configuration, whatever the case Exchange uses.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Roles = @{ "Mail Recipients" = [pscustomobject]@{ Name = "Mail Recipients"; Parent = ""; IsRootRole = $true; Description = "" } }
$global:Entries = @{ "Mail Recipients" = [ordered]@{
    "Get-Mailbox" = @("Identity", "ResultSize"); "Get-Recipient" = @("Identity")
    "Get-User" = @("Identity"); "Set-User" = @("Department", "Identity", "MobilePhone", "Phone")
} }
function global:Get-ManagementRole {
    param($Identity)
    if (-not $global:Roles.ContainsKey($Identity)) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    # Exchange Online returns the parent with the tenant in front of it.
    $r = $global:Roles[$Identity]
    if ($r.Parent) { $r = $r.PSObject.Copy(); $r.Parent = "contoso.onmicrosoft.com\$($r.Parent)" }
    $r
}
function global:Get-ManagementRoleEntry {
    param($Identity)
    $role = $Identity -replace '\\\*$', ''
    foreach ($k in $global:Entries[$role].Keys) {
        [pscustomobject]@{ Name = $k; Role = $role; Parameters = [string[]]$global:Entries[$role][$k] }
    }
}
function global:New-ManagementRole {
    param($Name, $Parent, [string[]]$EnabledCmdlets, $Description)
    $global:Calls.Add("New $Name Parent=$Parent EnabledCmdlets=$($EnabledCmdlets -join ',')")
    $global:Roles[$Name] = [pscustomobject]@{ Name = $Name; Parent = $Parent; IsRootRole = $false; Description = "$Description" }
    $global:Entries[$Name] = [ordered]@{}
    foreach ($c in $EnabledCmdlets) { $global:Entries[$Name][$c] = $global:Entries[$Parent][$c] }
}
function global:Remove-ManagementRole { param($Identity) $global:Calls.Add("RemoveRole $Identity"); $global:Roles.Remove($Identity) }
function global:Add-ManagementRoleEntry {
    param($Identity)
    $global:Calls.Add("Add $Identity")
    $role, $cmdlet = $Identity -split '\\'
    $global:Entries[$role][$cmdlet] = $global:Entries[$global:Roles[$role].Parent][$cmdlet]
}
function global:Remove-ManagementRoleEntry {
    param($Identity)
    $global:Calls.Add("Remove $Identity")
    $role, $cmdlet = $Identity -split '\\'
    $global:Entries[$role].Remove($cmdlet)
}
function global:Set-ManagementRoleEntry {
    param($Identity, [string[]]$Parameters)
    $global:Calls.Add("SetEntry $Identity $(($Parameters | Sort-Object) -join ',')")
    $role, $cmdlet = $Identity -split '\\'
    $global:Entries[$role][$cmdlet] = $Parameters
}

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -Depth 4 -InputObject $Object)))
}
function Invoke-Step($Label, $Action, $Values) {
    $global:Calls.Clear()
    $out = & $Script -Action $Action -Identity TF-CI-Role -Values (ConvertTo-B64 $Values) 2>&1
    "${Label}: $out"
    "$Label calls: $($global:Calls -join '; ')"
}

Invoke-Step "create" Create @{ Parent = "Mail Recipients"; Cmdlets = @("Get-Mailbox", "Set-User")
    CmdletParameters = @{ "Set-User" = @("Identity", "Phone") } }
Invoke-Step "update" Update @{ Parent = "Mail Recipients"; Cmdlets = @("get-mailbox", "Get-User"); CmdletParameters = @{} }
Invoke-Step "restrict" Update @{ Parent = "Mail Recipients"; Cmdlets = @("Get-Mailbox", "Get-User", "Set-User")
    CmdletParameters = @{ "Set-User" = @("Identity", "Phone") } }
Invoke-Step "unrestrict" Update @{ Parent = "Mail Recipients"; Cmdlets = @("Get-Mailbox", "Get-User", "Set-User"); CmdletParameters = @{} }
"builtin: " + (& $Script -Action Get -Identity "Mail Recipients" 2>&1)
$global:Roles.Remove("TF-CI-Role")
$global:Calls.Clear()
"notinparent: " + (& $Script -Action Create -Identity TF-CI-Role -Values (ConvertTo-B64 @{ Parent = "Mail Recipients"; Cmdlets = @("New-Mailbox"); CmdletParameters = @{} }) 2>&1)
"notinparent calls: $($global:Calls -join '; ')"
$global:Calls.Clear()
"badparam: " + (& $Script -Action Create -Identity TF-CI-Role -Values (ConvertTo-B64 @{ Parent = "Mail Recipients"; Cmdlets = @("Set-User"); CmdletParameters = @{ "Set-User" = @("Identity", "Bogus") } }) 2>&1)
"badparam calls: [$($global:Calls -join '; ')]"
