data "msexchangeonline_retention_policy" "default" {
  name = "Default MRM Policy"
}

output "retention_policy_tag_links" {
  description = "The tags of the default retention policy."
  value       = data.msexchangeonline_retention_policy.default.retention_policy_tag_links
}
