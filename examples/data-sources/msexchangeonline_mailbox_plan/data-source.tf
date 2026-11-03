data "msexchangeonline_mailbox_plan" "enterprise" {
  name = "ExchangeOnlineEnterprise"
}

output "enterprise_mailbox_quota_mb" {
  description = "Size, in MB, at which new Plan 2 mailboxes stop sending and receiving."
  value       = data.msexchangeonline_mailbox_plan.enterprise.prohibit_send_receive_quota_mb
}
