# role_assignment_policy.ps1 - runs bridge/scripts/role_assignment_policy.ps1
# against fake cmdlets (run by rbac_scripts_test.go): a change of roles adds
# and removes only the roles that differ.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Policy = $null
function global:Get-RoleAssignmentPolicy {
    param($Identity)
    if (-not $global:Policy) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    $global:Policy
}
function global:New-RoleAssignmentPolicy {
    param($Name, [string[]]$Roles, $Description, $IsDefault)
    $global:Calls.Add("New $Name Roles=$($Roles -join ',')")
    $global:Policy = [pscustomobject]@{ Name = $Name; Description = "$Description"; IsDefault = [bool]$IsDefault; AssignedRoles = [string[]]$Roles }
}
function global:Set-RoleAssignmentPolicy { param($Identity, $Description, $IsDefault) $global:Calls.Add("SetPolicy $Identity") }
function global:New-ManagementRoleAssignment {
    param($Policy, $Role)
    $global:Calls.Add("Add $Role")
    $global:Policy.AssignedRoles = [string[]](@($global:Policy.AssignedRoles) + $Role)
}
function global:Get-ManagementRoleAssignment {
    param($RoleAssignee, $Role)
    $roles = if ($Role) { @($Role) } else { @($global:Policy.AssignedRoles) }
    foreach ($r in $roles) {
        [pscustomobject]@{ Name = "$r-$RoleAssignee"; Role = $r; RoleAssigneeName = $RoleAssignee; RoleAssigneeType = "RoleAssignmentPolicy" }
    }
}
# Exchange refuses to remove a policy that still has role assignments.
function global:Remove-RoleAssignmentPolicy {
    param($Identity)
    if (@($global:Policy.AssignedRoles).Count -gt 0) { throw "There is at least one role assignment for this assignment policy." }
    $global:Calls.Add("RemovePolicy $Identity")
}
function global:Remove-ManagementRoleAssignment {
    param([Parameter(ValueFromPipeline)]$InputObject, $Identity)
    process {
        $role = if ($InputObject) { $InputObject.Role } else { $Identity }
        $global:Calls.Add("Remove $role")
        $global:Policy.AssignedRoles = [string[]]@($global:Policy.AssignedRoles | Where-Object { $_ -ne $role })
    }
}

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject $Object)))
}
"create: " + (& $Script -Action Create -Identity TF-CI-Policy -Values (ConvertTo-B64 @{ Roles = @("MyBaseOptions", "MyVoiceMail") }))
$global:Calls.Clear()
"update: " + (& $Script -Action Update -Identity TF-CI-Policy -Values (ConvertTo-B64 @{ Roles = @("mybaseoptions", "MyTextMessaging") }))
"update calls: " + ($global:Calls -join "; ")
$global:Calls.Clear()
"delete: " + (& $Script -Action Delete -Identity TF-CI-Policy 2>&1)
"delete calls: " + ($global:Calls -join "; ")
