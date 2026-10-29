data "msexchangeonline_safe_attachment_rule" "all_users" {
  name = "Safe Attachment Rule - All Users"
}

output "enabled" {
  description = "Whether the Safe Attachments rule for all users applies."
  value       = data.msexchangeonline_safe_attachment_rule.all_users.enabled
}
