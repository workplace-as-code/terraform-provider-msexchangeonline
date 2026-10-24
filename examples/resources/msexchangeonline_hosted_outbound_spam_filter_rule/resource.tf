# A strict outbound policy for a sending service, scoped by a standalone rule.
resource "msexchangeonline_outbound_spam_filter_policy" "bulk_senders" {
  name                          = "Bulk senders"
  recipient_limit_per_day       = 2000
  action_when_threshold_reached = "BlockUser"
}

resource "msexchangeonline_hosted_outbound_spam_filter_rule" "bulk_senders" {
  name                               = "Bulk senders"
  hosted_outbound_spam_filter_policy = msexchangeonline_outbound_spam_filter_policy.bulk_senders.name
  from                               = ["newsletter@contoso.com"]
}
