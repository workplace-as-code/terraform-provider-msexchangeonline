resource "msexchangeonline_dkim_signing_config" "contoso_com" {
  domain_name             = "contoso.com"
  enabled                 = true
  key_size                = 2048
  header_canonicalization = "Relaxed"
  body_canonicalization   = "Relaxed"
}

# After applying, publish these CNAME values in your DNS zone:
output "dkim_cname_selector1" {
  description = "Publish this CNAME in DNS for selector1._domainkey.contoso.com"
  value       = msexchangeonline_dkim_signing_config.contoso_com.selector1_cname
}

output "dkim_cname_selector2" {
  description = "Publish this CNAME in DNS for selector2._domainkey.contoso.com"
  value       = msexchangeonline_dkim_signing_config.contoso_com.selector2_cname
}
