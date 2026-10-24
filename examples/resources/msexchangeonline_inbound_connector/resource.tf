# Hybrid on-premises inbound connector
resource "msexchangeonline_inbound_connector" "onprem" {
  name           = "From On-Premises"
  enabled        = true
  connector_type = "OnPremises"

  sender_ip_addresses = ["203.0.113.10/32"]
  require_tls         = true
  comment             = "Inbound connector for Exchange hybrid deployment"
}

# Partner organization inbound connector with certificate validation
resource "msexchangeonline_inbound_connector" "partner" {
  name           = "From Partner Org"
  enabled        = true
  connector_type = "Partner"

  sender_domains                  = ["partner.com"]
  require_tls                     = true
  restrict_domains_to_certificate = true
  tls_sender_certificate_name     = "mail.partner.com"
  comment                         = "Enforced TLS from partner.com"
}
