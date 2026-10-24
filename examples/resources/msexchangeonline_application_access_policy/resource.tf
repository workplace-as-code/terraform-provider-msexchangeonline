# The reporting app can only read the mailboxes of the Finance group.
resource "msexchangeonline_application_access_policy" "reporting" {
  app_id                = "11111111-2222-3333-4444-555555555555"
  policy_scope_group_id = "finance-mailboxes@contoso.com"
  access_right          = "RestrictAccess"
  description           = "Reporting app: Finance mailboxes only"
}
