resource "msexchangeonline_organization_config" "org" {
  audit_disabled                             = false
  read_tracking_enabled                      = false
  focused_inbox_on                           = true
  mail_tips_all_tips_enabled                 = true
  mail_tips_external_recipients_tips_enabled = true
  mail_tips_large_audience_threshold         = 25
  message_recall_enabled                     = true
  oauth2_client_profile_enabled              = true
}
