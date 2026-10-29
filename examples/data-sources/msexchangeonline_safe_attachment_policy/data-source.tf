data "msexchangeonline_safe_attachment_policy" "default" {
  name = "Built-In Protection Policy"
}

output "safe_attachment_action" {
  description = "Action of the built-in Safe Attachments policy."
  value       = data.msexchangeonline_safe_attachment_policy.default.safe_attachment_action
}
