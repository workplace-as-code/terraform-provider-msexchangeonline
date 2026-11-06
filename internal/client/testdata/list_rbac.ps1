# list_rbac.ps1 - runs bridge/scripts/list.ps1 against fake RBAC cmdlets (run by
# rbac_scripts_test.go): terraform query must only list the custom role groups
# and roles, never the built-in ones Microsoft manages.
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
function global:Get-RoleGroup {
    param($Identity)
    if ($Identity) { return (RoleGroups) | Where-Object { $_.Name -eq $Identity } }
    RoleGroups
}
function global:RoleGroups {
    [pscustomobject]@{ Name = "Organization Management"; WellKnownObject = "ExOrgAdmin" }
    [pscustomobject]@{ Name = "ExchangeServiceAdmins_-1234567890"; WellKnownObject = "" }
    [pscustomobject]@{ Name = "Compliance Management"; WellKnownObject = "" }
    [pscustomobject]@{ Name = "TenantAdmins_-1234567"; WellKnownObject = "" }
    [pscustomobject]@{ Name = "HelpdeskAdmins_-7654321"; WellKnownObject = "" }
    [pscustomobject]@{ Name = "RIM-MailboxAdmins8f3c2a"; WellKnownObject = "" }
    [pscustomobject]@{ Name = "ISVMailboxUsers_-998877"; WellKnownObject = "" }
    [pscustomobject]@{ Name = "TF-CI-RoleGroup"; WellKnownObject = "" }
}
function global:Get-ManagementRole {
    [pscustomobject]@{ Name = "Mail Recipients"; IsRootRole = $true }
    [pscustomobject]@{ Name = "TF-CI-Role"; IsRootRole = $false }
}
function global:Get-ManagementRoleAssignment {
    [pscustomobject]@{ Name = "Mail Recipients-Organization Management"; RoleAssigneeType = "RoleGroup"; RoleAssigneeName = "Organization Management"; Delegating = $false }
    [pscustomobject]@{ Name = "TF-CI-Role-TF-CI-RoleGroup"; RoleAssigneeType = "RoleGroup"; RoleAssigneeName = "TF-CI-RoleGroup"; Delegating = $false }
    [pscustomobject]@{ Name = "TF-CI-Role-TF-CI-RoleGroup-Delegating"; RoleAssigneeType = "RoleGroup"; RoleAssigneeName = "TF-CI-RoleGroup"; Delegating = $true }
    [pscustomobject]@{ Name = "Application Mail.Read-TF-CI-App"; RoleAssigneeType = "ServicePrincipal"; RoleAssigneeName = "TF-CI-App"; Delegating = $false }
    [pscustomobject]@{ Name = "MyBaseOptions-Default Role Assignment Policy"; RoleAssigneeType = "RoleAssignmentPolicy"; RoleAssigneeName = "Default Role Assignment Policy"; Delegating = $false }
}
"role groups: " + (& $Script -Noun RoleGroup -Property Name)
"roles: " + (& $Script -Noun ManagementRole -Property Name)
"assignments: " + (& $Script -Noun ManagementRoleAssignment -Property Name)
