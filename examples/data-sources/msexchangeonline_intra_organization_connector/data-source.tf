data "msexchangeonline_intra_organization_connector" "hybrid" {
  name = "HybridIOC - 00000000-0000-0000-0000-000000000000"
}

output "hybrid_discovery_endpoint" {
  value = data.msexchangeonline_intra_organization_connector.hybrid.discovery_endpoint
}
