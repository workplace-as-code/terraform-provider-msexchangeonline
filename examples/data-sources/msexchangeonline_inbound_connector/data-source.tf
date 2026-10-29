data "msexchangeonline_inbound_connector" "on_premises" {
  name = "From On-Premises"
}

output "sender_ip_addresses" {
  description = "The on-premises IP addresses the connector accepts mail from."
  value       = data.msexchangeonline_inbound_connector.on_premises.sender_ip_addresses
}
