data "msexchangeonline_role_group" "organization_management" {
  name = "Organization Management"
}

output "organization_management_description" {
  value = data.msexchangeonline_role_group.organization_management.description
}
