data "msexchangeonline_hosted_content_filter_policy" "default" {
  name = "Default"
}

output "spam_action" {
  description = "Action taken on spam by the default anti-spam policy."
  value       = data.msexchangeonline_hosted_content_filter_policy.default.spam_action
}
