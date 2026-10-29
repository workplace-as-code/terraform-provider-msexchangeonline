data "msexchangeonline_quarantine_policy" "full" {
  name = "DefaultFullAccessPolicy"
}

output "full_access_permissions" {
  description = "What users may do with their quarantined messages under the built-in policy."
  value       = data.msexchangeonline_quarantine_policy.full.end_user_quarantine_permissions_value
}
