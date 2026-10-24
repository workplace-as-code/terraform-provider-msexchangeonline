# Hybrid on-premises outbound connector
resource "msexchangeonline_outbound_connector" "onprem" {
  name           = "To On-Premises"
  enabled        = true
  connector_type = "OnPremises"

  smart_hosts  = ["mail.contoso.local"]
  tls_settings = "CertificateVerification"
  comment      = "Routes mail to on-premises Exchange for local delivery"
}

# Partner organization outbound connector with enforced TLS
resource "msexchangeonline_outbound_connector" "partner" {
  name              = "To Partner Org"
  enabled           = true
  connector_type    = "Partner"
  recipient_domains = ["partner.com"]
  tls_settings      = "DomainValidation"
  comment           = "Enforced TLS to partner.com"
}
