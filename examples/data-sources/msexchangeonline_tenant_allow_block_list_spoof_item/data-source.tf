data "msexchangeonline_tenant_allow_block_list_spoof_item" "partner" {
  id = "External:contoso.com:mail.fabrikam.com"
}

output "partner_spoof_action" {
  description = "Whether the partner is allowed or blocked."
  value       = data.msexchangeonline_tenant_allow_block_list_spoof_item.partner.action
}
