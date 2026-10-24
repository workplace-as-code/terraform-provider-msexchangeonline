resource "msexchangeonline_hosted_content_filter_policy" "strict" {
  name                         = "Strict Spam Filter"
  spam_action                  = "Quarantine"
  high_confidence_spam_action  = "Quarantine"
  phish_spam_action            = "Quarantine"
  high_confidence_phish_action = "Quarantine"
  bulk_spam_action             = "Quarantine"
  bulk_threshold               = 4

  priority = 1
  enabled  = true

  recipient_domain_is = ["contoso.com"]
}

resource "msexchangeonline_hosted_content_filter_policy" "standard" {
  name                         = "Standard Spam Filter"
  spam_action                  = "MoveToJmf"
  high_confidence_spam_action  = "Quarantine"
  phish_spam_action            = "Quarantine"
  high_confidence_phish_action = "Quarantine"
  bulk_spam_action             = "MoveToJmf"
  bulk_threshold               = 7

  priority = 2
  enabled  = true
}
