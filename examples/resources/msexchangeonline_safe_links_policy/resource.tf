# Safe Links policy covering email, Teams, and Office apps
resource "msexchangeonline_safe_links_policy" "standard" {
  name = "Standard-SafeLinks"

  # Enable protection across all surfaces
  enable_safe_links_for_email  = true
  enable_safe_links_for_teams  = true
  enable_safe_links_for_office = true

  # Scanning behavior
  scan_urls                  = true
  deliver_message_after_scan = true

  # Click tracking and enforcement
  track_clicks        = true
  allow_click_through = false

  # Apply to internal senders too
  enable_for_internal_senders = true

  # Internal tools excluded from URL rewriting
  do_not_rewrite_urls = [
    "https://intranet.contoso.com/*",
    "https://sharepoint.contoso.com/*",
  ]

  recipient_domain_is = ["contoso.com"]
  priority            = 0
}
