# Dynamic Delivery — scan attachments without delaying message delivery
resource "msexchangeonline_safe_attachment_policy" "standard" {
  name = "Standard-SafeAttachments"

  # DynamicDelivery delivers the message immediately with a placeholder;
  # the real attachment is swapped back in once scanning completes
  safe_attachment_action = "DynamicDelivery"

  recipient_domain_is = ["contoso.com"]
  priority            = 0
}

# Strict policy for executives: messages with malicious attachments are blocked
resource "msexchangeonline_safe_attachment_policy" "strict" {
  name = "Strict-SafeAttachments-Executives"

  safe_attachment_action = "Block"

  recipient_domain_is = ["executives.contoso.com"]
  priority            = 0
}
