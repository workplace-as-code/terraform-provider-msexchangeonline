data "msexchangeonline_retention_policy_tag" "deleted_items" {
  name = "Deleted Items - 30 days"
}

output "age_limit_for_retention" {
  description = "Days before items in Deleted Items are removed."
  value       = data.msexchangeonline_retention_policy_tag.deleted_items.age_limit_for_retention
}
