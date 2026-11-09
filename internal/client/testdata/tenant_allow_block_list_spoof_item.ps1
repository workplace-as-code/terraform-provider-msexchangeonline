# tenant_allow_block_list_spoof_item.ps1 - runs bridge/scripts/tenant_allow_block_list_spoof_item.ps1
# against fake cmdlets (run by spoof_scripts_test.go). Like Exchange Online, Get- only
# lists without -Identity, and the other cmdlets require -Identity Default.
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Items = [System.Collections.Generic.List[object]]::new()
$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Get-TenantAllowBlockListSpoofItems {
    param($Identity, $Action, $SpoofType)
    if ($PSBoundParameters.ContainsKey("Identity")) { return }
    $global:Items.ToArray()
}
function Assert-Default($Identity) { if ($Identity -ne "Default") { throw "Identity must be Default" } }
function global:New-TenantAllowBlockListSpoofItems {
    param($Identity, $Action, $SendingInfrastructure, $SpoofedUser, $SpoofType)
    Assert-Default $Identity
    $global:Calls.Add("New $SpoofType $SpoofedUser $SendingInfrastructure $Action")
    $item = [pscustomobject]@{ Identity = "guid-1"; SpoofedUser = $SpoofedUser; SendingInfrastructure = $SendingInfrastructure; SpoofType = $SpoofType; Action = $Action }
    $global:Items.Add($item)
    $item
}
function global:Set-TenantAllowBlockListSpoofItems {
    param($Identity, $Ids, $Action)
    Assert-Default $Identity
    $global:Calls.Add("Set $Ids $Action")
    foreach ($i in $global:Items) { if ($i.Identity -in $Ids) { $i.Action = $Action } }
}
function global:Remove-TenantAllowBlockListSpoofItems {
    param($Identity, $Ids)
    Assert-Default $Identity
    $global:Calls.Add("Remove $Ids")
    if (-not @($global:Items | Where-Object { $_.Identity -in $Ids })) { throw "Id : $Ids does not exist." }
    $global:Items.RemoveAll([Predicate[object]]{ param($o) $o.Identity -in $Ids }) | Out-Null
}
$pair = @{ SpoofType = "External"; SpoofedUser = "contoso.com"; SendingInfrastructure = "192.0.2.1/24" }

"absent: [" + (& $Script -Action Get @pair) + "]"
"create: " + (& $Script -Action Create @pair -ListAction Allow)
$upper = @{ SpoofType = "external"; SpoofedUser = "CONTOSO.com"; SendingInfrastructure = "192.0.2.1/24" }
"read: " + (& $Script -Action Get @upper)
$global:Calls.Clear()
"duplicate: " + (& $Script -Action Create @pair -ListAction Allow 2>&1)
"duplicate calls: [$($global:Calls -join '; ')]"
"update: " + (& $Script -Action Update @pair -ListAction Block)
"update calls: $($global:Calls -join '; ')"
"delete: " + (& $Script -Action Delete @pair)
"delete again: " + (& $Script -Action Delete @pair)
