# Disable SMTP AUTH and legacy TLS, keep reply-all storm protection.
resource "msexchangeonline_transport_config" "this" {
  smtp_client_authentication_disabled = true
  allow_legacy_tls_clients            = false
  reply_all_storm_protection_enabled  = true
}
