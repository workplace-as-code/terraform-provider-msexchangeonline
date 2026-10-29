data "msexchangeonline_outbound_connector" "on_premises" {
  name = "To On-Premises"
}

output "smart_hosts" {
  description = "The on-premises servers that receive mail through the connector."
  value       = data.msexchangeonline_outbound_connector.on_premises.smart_hosts
}
