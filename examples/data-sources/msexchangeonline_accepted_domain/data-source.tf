data "msexchangeonline_accepted_domain" "primary" {
  name = "contoso.com"
}

output "primary_domain_type" {
  description = "Type of the contoso.com accepted domain: Authoritative or InternalRelay."
  value       = data.msexchangeonline_accepted_domain.primary.domain_type
}
