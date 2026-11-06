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

variable "test_app_object_id" {
  description = "Object ID of the enterprise application of test_app_id, to test RBAC for Applications."
  type        = string
  default     = ""
}

locals {
  rbac_app = var.test_rbac && var.test_app_id != "" && var.test_app_object_id != "" ? 1 : 0
}

# RBAC for Applications: the test app can read the mailboxes of the scope only.
resource "msexchangeonline_service_principal" "test" {
  count        = local.rbac_app
  app_id       = var.test_app_id
  object_id    = var.test_app_object_id
  display_name = local.updated ? "TF-CI-App-Updated" : "TF-CI-App"
}

resource "msexchangeonline_management_role_assignment" "app" {
  count                 = local.rbac_app
  name                  = "TF-CI-Assignment-App"
  role                  = "Application Mail.Read"
  app                   = msexchangeonline_service_principal.test[0].app_id
  custom_resource_scope = msexchangeonline_management_scope.test[0].name
}
