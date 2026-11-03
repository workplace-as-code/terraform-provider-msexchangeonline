data "msexchangeonline_sharing_policy" "default" {
  name = "Default Sharing Policy"
}

output "default_sharing_domains" {
  description = "What users can share, and with whom, by default."
  value       = data.msexchangeonline_sharing_policy.default.domains
}
