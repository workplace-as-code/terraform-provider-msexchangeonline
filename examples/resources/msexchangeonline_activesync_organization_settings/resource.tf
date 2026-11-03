# Devices that match no device access rule wait in quarantine.
resource "msexchangeonline_activesync_organization_settings" "this" {
  default_access_level  = "Quarantine"
  admin_mail_recipients = ["mobility@contoso.com"]
  user_mail_insert      = "Your device is waiting for approval by the mobility team."
}
