# The policy sets no recipient condition, so it has no rule of its own: the
# standalone rule below scopes it. Exchange links a policy to at most one rule.
resource "msexchangeonline_safe_links_policy" "strict" {
  name                         = "Strict Safe Links"
  enable_safe_links_for_email  = true
  enable_safe_links_for_office = true
  enable_safe_links_for_teams  = true
  track_clicks                 = true
  allow_click_through          = false
  scan_urls                    = true
  deliver_message_after_scan   = true
}

resource "msexchangeonline_safe_links_rule" "all_users" {
  name              = "Safe Links Rule - All Users"
  safe_links_policy = msexchangeonline_safe_links_policy.strict.name
  priority          = 1
  enabled           = true

  recipient_domain_is = ["contoso.com"]
}
