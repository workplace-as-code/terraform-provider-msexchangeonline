# role_assignment_policy.ps1 - role assignment policies and their end-user
# roles. The policy owns its role assignments: Update adds and removes them to
# match Roles.
#
#   -Identity  policy name
#   -Values    base64 JSON: Roles, Description, IsDefault
#
# Output: Name, Description, IsDefault, Roles (sorted).

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$wanted = if ($Values) { ConvertFrom-Json -InputObject ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Values))) } else { $null }

# Get-BridgePolicyRoles returns the names of the roles of the policy.
function Get-BridgePolicyRoles($Policy) {
    [string[]]@(@(Get-BridgeProp $Policy "AssignedRoles" @()) | ForEach-Object {
        # A role may come back as an identity ("\MyBaseOptions"): keep the name.
        @("$_" -split '\\' | Where-Object { $_ })[-1]
    } | Sort-Object)
}

function Read-BridgePolicy {
    $p = Invoke-BridgeRetry { Get-RoleAssignmentPolicy -Identity $Identity }
    ConvertTo-Json -Compress -InputObject ([ordered]@{
        Name        = [string]$p.Name
        Description = [string](Get-BridgeProp $p "Description" "")
        IsDefault   = [bool](Get-BridgeProp $p "IsDefault" $false)
        Roles       = Get-BridgePolicyRoles $p
    })
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgePolicy } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $new = @{ Name = $Identity; Roles = [string[]]@($wanted.Roles) }
            if ($null -ne $wanted.PSObject.Properties["Description"]) { $new.Description = $wanted.Description }
            if ($null -ne $wanted.PSObject.Properties["IsDefault"] -and $wanted.IsDefault) { $new.IsDefault = $true }
            Invoke-BridgeRetry -Create { New-RoleAssignmentPolicy @new | Out-Null }
            Read-BridgePolicy
        }

        "Update" {
            $p = Invoke-BridgeRetry { Get-RoleAssignmentPolicy -Identity $Identity }
            $set = @{}
            if ($null -ne $wanted.PSObject.Properties["Description"] -and [string](Get-BridgeProp $p "Description" "") -ne $wanted.Description) {
                $set.Description = $wanted.Description
            }
            if ($null -ne $wanted.PSObject.Properties["IsDefault"] -and $wanted.IsDefault -and -not [bool](Get-BridgeProp $p "IsDefault" $false)) {
                $set.IsDefault = $true
            }
            if ($set.Count -gt 0) { Invoke-BridgeRetry { Set-RoleAssignmentPolicy -Identity $Identity @set } }

            $current = Get-BridgePolicyRoles $p
            foreach ($role in @($wanted.Roles)) {
                if (@($current | Where-Object { $_ -ieq $role }).Count -eq 0) {
                    Invoke-BridgeRetry { New-ManagementRoleAssignment -Policy $Identity -Role $role | Out-Null }
                }
            }
            foreach ($role in $current) {
                if (@($wanted.Roles | Where-Object { $_ -ieq $role }).Count -eq 0) {
                    Invoke-BridgeRetry {
                        Get-ManagementRoleAssignment -RoleAssignee $Identity -Role $role |
                            Where-Object { [string](Get-BridgeProp $_ "RoleAssigneeType" "") -eq "RoleAssignmentPolicy" } |
                            Remove-ManagementRoleAssignment
                    }
                }
            }
            Read-BridgePolicy
        }

        "Delete" {
            # Exchange refuses to remove a policy that still has role
            # assignments: remove them first.
            try {
                $assignments = @(Invoke-BridgeRetry { Get-ManagementRoleAssignment -RoleAssignee $Identity } |
                    Where-Object { [string](Get-BridgeProp $_ "RoleAssigneeType" "") -eq "RoleAssignmentPolicy" })
                foreach ($a in $assignments) { Invoke-BridgeRetry { $a | Remove-ManagementRoleAssignment } }
                Remove-RoleAssignmentPolicy -Identity $Identity
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("RoleAssignmentPolicy $Action failed: $($_.Exception.Message)")
    exit 1
}
