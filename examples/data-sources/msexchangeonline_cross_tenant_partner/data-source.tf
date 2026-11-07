data "msexchangeonline_cross_tenant_partner" "fabrikam" {
  partner_tenant_id = "00000000-0000-0000-0000-000000000000"
}

output "fabrikam_capabilities" {
  description = "The Exchange capabilities opened to Fabrikam."
  value       = [for c in data.msexchangeonline_cross_tenant_partner.fabrikam.capabilities : c.name]
}
