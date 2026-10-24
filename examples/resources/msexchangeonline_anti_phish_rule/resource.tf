# The policy sets no recipient condition, so it has no rule of its own: the
# standalone rule below scopes it. Exchange links a policy to at most one rule.
resource "msexchangeonline_anti_phish_policy" "strict" {
  name                      = "Strict Anti-Phish"
  phish_threshold_level     = 2
  enable_spoof_intelligence = true
}

resource "msexchangeonline_anti_phish_rule" "executives" {
  name              = "Anti-Phish Rule - Executives"
  anti_phish_policy = msexchangeonline_anti_phish_policy.strict.name
  priority          = 1
  enabled           = true

  sent_to_member_of = ["executives@contoso.com"]
}
