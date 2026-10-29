data "msexchangeonline_hosted_content_filter_rule" "strict" {
  name = "Strict Spam Filter Rule"
}

output "sent_to_member_of" {
  description = "The groups the strict spam filter rule applies to."
  value       = data.msexchangeonline_hosted_content_filter_rule.strict.sent_to_member_of
}
