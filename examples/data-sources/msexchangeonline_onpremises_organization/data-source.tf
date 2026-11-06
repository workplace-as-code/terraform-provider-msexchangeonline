data "msexchangeonline_onpremises_organization" "hybrid" {
  name = "Contoso Hybrid"
}

output "hybrid_domains" {
  value = data.msexchangeonline_onpremises_organization.hybrid.hybrid_domains
}
