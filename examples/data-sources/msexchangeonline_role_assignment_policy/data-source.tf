data "msexchangeonline_role_assignment_policy" "default" {
  name = "Default Role Assignment Policy"
}

output "default_end_user_roles" {
  description = "What users can change on their own mailbox by default."
  value       = data.msexchangeonline_role_assignment_policy.default.roles
}
