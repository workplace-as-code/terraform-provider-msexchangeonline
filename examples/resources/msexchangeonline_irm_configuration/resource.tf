# Purview Message Encryption on, with the Encrypt-Only and Do Not Forward
# options in Outlook on the web
resource "msexchangeonline_irm_configuration" "this" {
  azure_rms_licensing_enabled       = true
  internal_licensing_enabled        = true
  simplified_client_access_enabled  = true
  transport_decryption_setting      = "Optional"
  journal_report_decryption_enabled = true
  enable_portal_tracking_logs       = true
}
