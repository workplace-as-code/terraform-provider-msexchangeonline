data "msexchangeonline_organization_relationship" "fabrikam" {
  name = "Fabrikam"
}

output "fabrikam_free_busy_level" {
  description = "Free/busy details shared with Fabrikam."
  value       = data.msexchangeonline_organization_relationship.fabrikam.free_busy_access_level
}
