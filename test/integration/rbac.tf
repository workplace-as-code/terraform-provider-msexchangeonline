# Exchange RBAC delegation. The app needs the Role Management role
# (tools/new_role_group.ps1 -IncludeRoleManagement): test/run.sh sets test_rbac
# when EXCHANGE_TEST_RBAC is set.

variable "test_rbac" {
  description = "Set by test/run.sh to test the Exchange RBAC resources."
  type        = bool
  default     = false
}

locals {
  rbac = var.test_rbac ? 1 : 0
}

resource "msexchangeonline_management_scope" "test" {
  count                        = local.rbac
  name                         = "TF-CI-Scope"
  recipient_restriction_filter = local.updated ? "CustomAttribute15 -eq 'TF-CI-UPDATED'" : "CustomAttribute15 -eq 'TF-CI'"
}

resource "msexchangeonline_role_group" "test" {
  count       = local.rbac
  name        = "TF-CI-RoleGroup"
  description = local.updated ? "Integration test role group (updated)" : "Integration test role group"
}

data "msexchangeonline_role_group" "organization_management" {
  count = local.rbac
  name  = "Organization Management"
}

check "built_in_role_group" {
  assert {
    condition     = local.rbac == 0 || one(data.msexchangeonline_role_group.organization_management[*].name) == "Organization Management"
    error_message = "The Organization Management role group could not be read."
  }
}

# A help desk role that can read recipients and change phone numbers only.
resource "msexchangeonline_management_role" "test" {
  count       = local.rbac
  name        = "TF-CI-Role"
  parent      = "Mail Recipients"
  description = "Integration test role"
  cmdlets     = local.updated ? ["Get-Mailbox", "Get-Recipient", "Get-User", "Set-User"] : ["Get-Mailbox", "Get-Recipient", "Set-User"]
  cmdlet_parameters = {
    "Set-User" = ["Identity", "Phone", "MobilePhone"]
  }
}

# The custom role, limited to the recipients of the scope.
resource "msexchangeonline_management_role_assignment" "scoped" {
  count                        = local.rbac
  name                         = "TF-CI-Assignment-Scoped"
  role                         = msexchangeonline_management_role.test[0].name
  role_group                   = msexchangeonline_role_group.test[0].name
  custom_recipient_write_scope = msexchangeonline_management_scope.test[0].name
  enabled                      = !local.updated
}

# A built-in role on the whole organization.
resource "msexchangeonline_management_role_assignment" "builtin" {
  count      = local.rbac
  name       = "TF-CI-Assignment-Builtin"
  role       = "Distribution Groups"
  role_group = msexchangeonline_role_group.test[0].name
}
