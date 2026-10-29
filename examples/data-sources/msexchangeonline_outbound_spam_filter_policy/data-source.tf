data "msexchangeonline_outbound_spam_filter_policy" "default" {
  name = "Default"
}

output "recipient_limit_per_day" {
  description = "Daily recipient limit of the default outbound spam policy."
  value       = data.msexchangeonline_outbound_spam_filter_policy.default.recipient_limit_per_day
}
