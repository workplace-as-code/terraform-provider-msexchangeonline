# Block a spoofing domain and a phishing URL.
resource "msexchangeonline_tenant_allow_block_list_item" "spoofed_domain" {
  list_type = "Sender"
  entry     = "contoso-billing.example"
  action    = "Block"
  notes     = "Lookalike domain used in invoice fraud"
}

resource "msexchangeonline_tenant_allow_block_list_item" "phishing_url" {
  list_type       = "Url"
  entry           = "login.contoso-billing.example/*"
  action          = "Block"
  expiration_date = "2027-01-01T00:00:00Z"
}
