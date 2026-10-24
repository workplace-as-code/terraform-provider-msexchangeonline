# Strict anti-phishing policy for executives and finance team
resource "msexchangeonline_anti_phish_policy" "strict" {
  name = "Strict-AntiPhish-Executives"

  # Aggressive detection threshold
  phish_threshold_level = 3

  # Spoof intelligence
  enable_spoof_intelligence  = true
  authentication_fail_action = "Quarantine"

  # Mailbox intelligence (AI-based impersonation detection)
  enable_mailbox_intelligence            = true
  enable_mailbox_intelligence_protection = true
  mailbox_intelligence_protection_action = "Quarantine"

  # Protect the organization's own domains from impersonation
  enable_organization_domains_protection = true

  # Visual indicators in Outlook
  enable_unauthenticated_sender    = true
  enable_via_tag                   = true
  enable_first_contact_safety_tips = true

  # Apply to the finance and executive domains
  recipient_domain_is = ["finance.contoso.com", "executives.contoso.com"]
  priority            = 0
}

# Standard anti-phishing policy for all other users
resource "msexchangeonline_anti_phish_policy" "standard" {
  name = "Standard-AntiPhish"

  phish_threshold_level      = 1
  enable_spoof_intelligence  = true
  authentication_fail_action = "MoveToJmf"

  enable_mailbox_intelligence = true

  enable_unauthenticated_sender = true
  enable_via_tag                = true

  recipient_domain_is = ["contoso.com"]
  priority            = 1
}
