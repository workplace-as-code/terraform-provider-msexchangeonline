data "msexchangeonline_cas_mailbox_plan" "enterprise" {
  name = "ExchangeOnlineEnterprise"
}

output "new_mailboxes_have_pop" {
  description = "Whether new Plan 2 mailboxes can use POP3."
  value       = data.msexchangeonline_cas_mailbox_plan.enterprise.pop_enabled
}
