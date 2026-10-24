# Delete items in Deleted Items after 30 days.
resource "msexchangeonline_retention_policy_tag" "deleted_items" {
  name                    = "Deleted Items - 30 days"
  type                    = "DeletedItems"
  age_limit_for_retention = 30
  retention_action        = "DeleteAndAllowRecovery"
}

# Archive everything after two years.
resource "msexchangeonline_retention_policy_tag" "archive" {
  name                    = "Default - Archive after 2 years"
  type                    = "All"
  age_limit_for_retention = 730
  retention_action        = "MoveToArchive"
}

resource "msexchangeonline_retention_policy" "standard" {
  name = "Standard MRM"
  retention_policy_tag_links = [
    msexchangeonline_retention_policy_tag.deleted_items.name,
    msexchangeonline_retention_policy_tag.archive.name,
  ]
}
