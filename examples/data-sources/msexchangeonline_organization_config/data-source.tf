data "msexchangeonline_organization_config" "tenant" {}

output "max_send_size" {
  description = "Largest message, in bytes, that users of the tenant can send."
  value       = data.msexchangeonline_organization_config.tenant.max_send_size
}
