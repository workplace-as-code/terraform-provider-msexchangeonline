data "msexchangeonline_dkim_signing_config" "contoso" {
  domain_name = "contoso.com"
}

output "selector1_cname" {
  description = "The first CNAME record to publish in the contoso.com DNS zone."
  value       = data.msexchangeonline_dkim_signing_config.contoso.selector1_cname
}
