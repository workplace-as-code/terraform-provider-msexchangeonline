# The policy sets no recipient condition, so it has no rule of its own: the
# standalone rule below scopes it. Exchange links a policy to at most one rule.
resource "msexchangeonline_hosted_content_filter_policy" "strict" {
  name           = "Strict Spam Filter"
  spam_action    = "Quarantine"
  bulk_threshold = 4
}

resource "msexchangeonline_hosted_content_filter_rule" "all_users" {
  name                         = "Strict Spam Filter Rule"
  hosted_content_filter_policy = msexchangeonline_hosted_content_filter_policy.strict.name
  priority                     = 1
  enabled                      = true

  recipient_domain_is = ["contoso.com"]
}
