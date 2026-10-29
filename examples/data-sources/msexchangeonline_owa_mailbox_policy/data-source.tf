data "msexchangeonline_owa_mailbox_policy" "default" {
  name = "OwaMailboxPolicy-Default"
}

output "attachments_enabled" {
  description = "Whether the default Outlook on the web policy allows attachments."
  value       = data.msexchangeonline_owa_mailbox_policy.default.attachments_enabled
}
