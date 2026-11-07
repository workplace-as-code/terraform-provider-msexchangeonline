# The help desk of the French subsidiary changes the phone numbers of its users.
# Refer to the role, role group and scope resources, for example
# msexchangeonline_management_role.helpdesk_phone.name, so that Terraform creates
# them first and deletes the assignment before them.
resource "msexchangeonline_management_role_assignment" "helpdesk" {
  role                         = "Helpdesk Phone"
  role_group                   = "Helpdesk France"
  custom_recipient_write_scope = "France"
}
