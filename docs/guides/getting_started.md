---
page_title: "Getting Started"
subcategory: "Guides"
---

# Getting Started

This guide walks you through setting up the Microsoft Exchange Online provider to manage your Microsoft 365 configuration as code.

## Prerequisites

- Terraform >= 1.5
- PowerShell 7+ installed (`pwsh --version`)
- ExchangeOnlineManagement module: `Install-Module ExchangeOnlineManagement -Scope CurrentUser`
- An administrator account to create the app registration and grant its permissions (Exchange Administrator or Global Administrator)

## Step 1 — Create the App Registration

Create an Entra ID app registration with a certificate, the `Exchange.ManageAsApp` application permission and the least-privilege role group: follow steps 1 to 4 of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication). Exchange Online app-only access requires a certificate: client secrets are not supported.

Note the tenant's `.onmicrosoft.com` domain (not the tenant GUID), the client ID, the path of the `.pfx` file and its password.

## Step 2 — Configure the Provider

```hcl
# providers.tf
terraform {
  required_providers {
    msexchangeonline = {
      source  = "workplace-as-code/msexchangeonline"
      version = "~> 0.1"
    }
  }
}

provider "msexchangeonline" {
  tenant_id            = var.tenant_id
  client_id            = var.client_id
  certificate_path     = var.certificate_path
  certificate_password = var.certificate_password
}
```

Declare the variables used by the provider block:

```terraform
# variables.tf
variable "tenant_id" {
  type = string
}

variable "client_id" {
  type = string
}

variable "certificate_path" {
  type = string
}

variable "certificate_password" {
  type      = string
  sensitive = true
}
```

Give them values in a `terraform.tfvars` file, or with `TF_VAR_<name>` environment variables. Keep `terraform.tfvars` out of version control: it holds the certificate password.

```terraform
# terraform.tfvars
tenant_id            = "contoso.onmicrosoft.com"
client_id            = "00000000-0000-0000-0000-000000000000"
certificate_path     = "/path/to/terraform-msexchangeonline.pfx"
certificate_password = "your-cert-password"
```

Or set everything through environment variables (recommended for CI/CD) and leave the provider block empty: `provider "msexchangeonline" {}`. In that case, skip `variables.tf` and `terraform.tfvars`.

```bash
export EXCHANGE_TENANT_ID="contoso.onmicrosoft.com"
export EXCHANGE_CLIENT_ID="your-client-id"
export EXCHANGE_CERTIFICATE_PATH="/path/to/cert.pfx"
export EXCHANGE_CERTIFICATE_PASSWORD="your-cert-password"
```

## Step 3 — Your First Resource

```hcl
# main.tf
resource "msexchangeonline_transport_rule" "block_executables" {
  name    = "Block executable attachments"
  enabled = true

  attachment_is_executable = true
  action                   = "Reject"
  reject_message           = "Executable attachments are not permitted."
}
```

Without `priority`, Exchange adds the rule after the existing ones.

```bash
terraform init
terraform plan    # Preview — nothing is changed yet
terraform apply   # Apply to your tenant
```

## Next Steps

- [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication): permissions, certificates, production setup
- [Import an existing tenant](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/import_existing): generate the configuration of the objects already in place
- [Detect drift in CI](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/drift_detection): run `terraform plan` on a schedule to detect changes made outside Terraform
- [Resources and availability](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/availability): every resource, the Exchange edition it works with and the license it needs
