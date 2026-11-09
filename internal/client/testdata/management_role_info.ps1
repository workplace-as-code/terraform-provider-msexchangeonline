# management_role_info.ps1 - runs bridge/scripts/management_role_info.ps1 against
# fake role cmdlets (run by rbac_scripts_test.go).
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Roles = @{
    "Transport Rules" = [pscustomobject]@{ Name = "Transport Rules"; Parent = ""; IsRootRole = $true; Description = "Built in" }
    "TF-CI-Role"      = [pscustomobject]@{ Name = "TF-CI-Role"; Parent = "contoso.onmicrosoft.com\Mail Recipients"; IsRootRole = $false; Description = "" }
}
function global:Get-ManagementRole {
    param($Identity)
    if (-not $global:Roles.ContainsKey($Identity)) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    $global:Roles[$Identity]
}
function global:Get-ManagementRoleEntry {
    param($Identity)
    foreach ($n in "Set-User", "Get-Mailbox") { [pscustomobject]@{ Name = $n } }
}

"builtin: " + (& $Script -Name "Transport Rules")
"custom: " + (& $Script -Name "TF-CI-Role")
"missing: [" + (& $Script -Name "Nope") + "]"
