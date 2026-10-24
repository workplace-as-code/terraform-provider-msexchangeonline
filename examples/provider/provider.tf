terraform {
  required_providers {
    msexchangeonline = {
      source  = "workplace-as-code/msexchangeonline"
      version = "~> 0.1"
    }
  }
}

provider "msexchangeonline" {
  tenant_id            = "contoso.onmicrosoft.com"
  client_id            = var.client_id
  certificate_path     = var.certificate_path
  certificate_password = var.certificate_password
}

resource "msexchangeonline_remote_domain" "partner" {
  name                 = "Partner"
  domain_name          = "partner.example"
  auto_forward_enabled = false
}
