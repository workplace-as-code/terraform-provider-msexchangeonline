# Quarantine unknown devices, allow the managed iPhones.
resource "msexchangeonline_activesync_organization_settings" "this" {
  default_access_level  = "Quarantine"
  admin_mail_recipients = ["mobility@contoso.com"]
}

resource "msexchangeonline_activesync_device_access_rule" "iphone" {
  query_string   = "iPhone"
  characteristic = "DeviceModel"
  access_level   = "Allow"
}
