# Allow a partner that sends as contoso.com from its own mail servers.
resource "msexchangeonline_tenant_allow_block_list_spoof_item" "partner" {
  spoofed_user           = "contoso.com"
  sending_infrastructure = "mail.fabrikam.com"
  spoof_type             = "External"
  action                 = "Allow"
}
