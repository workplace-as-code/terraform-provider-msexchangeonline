data "msexchangeonline_hosted_outbound_spam_filter_rule" "bulk_senders" {
  name = "Bulk senders"
}

output "from_member_of" {
  description = "The groups the outbound spam rule applies to."
  value       = data.msexchangeonline_hosted_outbound_spam_filter_rule.bulk_senders.from_member_of
}
