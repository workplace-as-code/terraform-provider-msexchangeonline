data "msexchangeonline_management_role" "transport_rules" {
  name = "Transport Rules"
}

output "transport_rules_cmdlets" {
  description = "Cmdlets of the built-in Transport Rules role."
  value       = data.msexchangeonline_management_role.transport_rules.cmdlets
}
