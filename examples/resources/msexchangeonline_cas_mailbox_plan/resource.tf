# New Exchange Online Plan 2 mailboxes start without POP, IMAP or EWS.
resource "msexchangeonline_cas_mailbox_plan" "enterprise" {
  name         = "ExchangeOnlineEnterprise"
  pop_enabled  = false
  imap_enabled = false
  ews_enabled  = false
}
