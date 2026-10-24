# The policy sets no recipient condition, so it has no rule of its own: the
# standalone rule below scopes it. Exchange links a policy to at most one rule.
resource "msexchangeonline_safe_attachment_policy" "dynamic" {
  name                   = "Dynamic Delivery"
  safe_attachment_action = "DynamicDelivery"
}

resource "msexchangeonline_safe_attachment_rule" "all_users" {
  name                   = "Safe Attachment Rule - All Users"
  safe_attachment_policy = msexchangeonline_safe_attachment_policy.dynamic.name
  priority               = 1
  enabled                = true

  recipient_domain_is = ["contoso.com"]
}
