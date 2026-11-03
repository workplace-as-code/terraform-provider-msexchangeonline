# New Exchange Online Plan 2 mailboxes: 50 MB messages, 30 days to recover items.
resource "msexchangeonline_mailbox_plan" "enterprise" {
  name                          = "ExchangeOnlineEnterprise"
  max_send_size_mb              = 50
  max_receive_size_mb           = 50
  retain_deleted_items_for_days = 30
}
