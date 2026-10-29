data "msexchangeonline_safe_links_rule" "all_users" {
  name = "Safe Links Rule - All Users"
}

output "enabled" {
  description = "Whether the Safe Links rule for all users applies."
  value       = data.msexchangeonline_safe_links_rule.all_users.enabled
}
