resource "msexchangeonline_outbound_spam_filter_policy" "strict" {
  name                          = "Strict Outbound Spam Policy"
  action_when_threshold_reached = "BlockUser"
  auto_forwarding_mode          = "Off"

  recipient_limit_external_per_hour = 500
  recipient_limit_internal_per_hour = 1000
  recipient_limit_per_day           = 1000

  notify_outbound_spam            = true
  notify_outbound_spam_recipients = ["security@contoso.com"]

  priority = 1
  enabled  = true

  sender_domain_is = ["contoso.com"]
}
