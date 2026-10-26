# Terraform Provider for Microsoft Exchange Online

Manage Exchange Online configuration as code with Terraform: mail flow, anti-spam, anti-phishing, anti-malware, Defender for Office 365, client access policies and organization settings.

[![CI](https://github.com/workplace-as-code/terraform-provider-msexchangeonline/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/workplace-as-code/terraform-provider-msexchangeonline/actions/workflows/ci.yml)
[![Terraform Registry](https://img.shields.io/github/v/release/workplace-as-code/terraform-provider-msexchangeonline?label=registry&logo=terraform&color=7B42BC)](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> [!WARNING]
> **Use at your own risk.** This is a community project, maintained on a best-effort basis and provided as is, without warranty (MIT license). Each release is tested by hand on a test tenant before it is published; nothing is tested continuously against Microsoft 365. Run `terraform plan` and try your configuration on a non-production tenant before applying it to production.

- Review every change with `terraform plan` before it reaches the tenant
- Detect configuration drift made in the admin center
- Adopt existing objects safely: attributes left out of the configuration are never written
- Start from an existing tenant: `terraform query` finds its objects and generates their configuration
- Promote tested configuration from a QA tenant to production through Git
- Keep a full audit trail in version control

---

## Quick Start

The provider is published as `workplace-as-code/msexchangeonline` and its resources are prefixed `msexchangeonline_`.

```hcl
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

# Block executable attachments
resource "msexchangeonline_transport_rule" "block_executables" {
  name     = "Block executable attachments"
  priority = 1
  enabled  = true

  attachment_is_executable = true

  action         = "Reject"
  reject_message = "Executable attachments are not permitted."
}

# Never auto-forward mail to a banking partner
resource "msexchangeonline_remote_domain" "partner" {
  name                 = "Banking Partner"
  domain_name          = "partner-bank.com"
  auto_forward_enabled = false
}
```

```bash
terraform init
terraform plan   # Preview changes — nothing applied yet
terraform apply  # Apply to your tenant
```

---

## Supported Resources

Full reference in [docs/resources](docs/resources/). The [Resources and availability](docs/guides/availability.md) guide lists the Exchange edition and the license each resource needs.

| Area | Resources |
|---|---|
| Mail flow | `msexchangeonline_transport_rule`, `msexchangeonline_accepted_domain`, `msexchangeonline_remote_domain`, `msexchangeonline_inbound_connector`, `msexchangeonline_outbound_connector`, `msexchangeonline_transport_config`, `msexchangeonline_dkim_signing_config` |
| Anti-spam | `msexchangeonline_hosted_connection_filter_policy`, `msexchangeonline_tenant_allow_block_list_item`, `msexchangeonline_hosted_content_filter_policy`, `msexchangeonline_hosted_content_filter_rule`, `msexchangeonline_outbound_spam_filter_policy`, `msexchangeonline_hosted_outbound_spam_filter_rule`, `msexchangeonline_quarantine_policy` |
| Anti-malware and anti-phishing | `msexchangeonline_malware_filter_policy`, `msexchangeonline_malware_filter_rule`, `msexchangeonline_anti_phish_policy`, `msexchangeonline_anti_phish_rule`, `msexchangeonline_external_in_outlook` |
| Defender for Office 365 | `msexchangeonline_safe_links_policy`, `msexchangeonline_safe_links_rule`, `msexchangeonline_safe_attachment_policy`, `msexchangeonline_safe_attachment_rule` |
| Clients and devices | `msexchangeonline_owa_mailbox_policy`, `msexchangeonline_mobile_device_mailbox_policy` |
| Organization | `msexchangeonline_organization_config`, `msexchangeonline_email_address_policy`, `msexchangeonline_retention_policy`, `msexchangeonline_retention_policy_tag`, `msexchangeonline_application_access_policy` |

Data sources: `msexchangeonline_accepted_domain`, `msexchangeonline_accepted_domains`, `msexchangeonline_mailbox`, `msexchangeonline_organization_config`.

### Related providers

- [workplace-as-code/msexchangeserver](https://github.com/workplace-as-code/terraform-provider-msexchangeserver): Exchange Server (on-premises) configuration.
- [workplace-as-code/msteams](https://github.com/workplace-as-code/terraform-provider-msteams): Microsoft Teams policies and tenant settings, built on the same PowerShell bridge.

---

## Installation

### Requirements
- Terraform >= 1.5
- PowerShell 7+ (`pwsh`) with the ExchangeOnlineManagement v3 module (3.1.0 or later for OIDC)
- An Entra ID app registration with the `Exchange.ManageAsApp` permission. [`tools/new_role_group.ps1`](tools/new_role_group.ps1) grants it only the Exchange roles the provider needs (see the [Authentication guide](docs/guides/authentication.md))
- A way to authenticate as that app: a certificate (`.pfx` file, base64 content or Windows certificate store), or OIDC from GitHub Actions or another platform with a public OIDC issuer, with no stored secret

The PowerShell bridge scripts are embedded in the provider binary: nothing else needs to be installed. The provider keeps a few PowerShell processes open during a run, so the Exchange connection is opened once per process instead of once per resource (see [Performance tuning](#performance-tuning)).

### Performance tuning

| Variable | Default | Effect |
|---|---|---|
| `EXCHANGE_MAX_CONCURRENCY` | `4` | Maximum number of PowerShell processes running at once |
| `EXCHANGE_SCRIPT_TIMEOUT` | `10m` | Maximum duration of one bridge call |
| `EXCHANGE_BRIDGE_MODE` | `persistent` | `oneshot` starts a new PowerShell process for every call |
| `EXCHANGE_WORKER_MAX_REQUESTS` | `200` | Requests served by a PowerShell process before it is replaced |
| `EXCHANGE_BRIDGE_DIR` | unset | Run the bridge scripts from this folder instead of the embedded copy (script development only) |

---

## Documentation

- [Getting Started](docs/guides/getting_started.md)
- [Authentication](docs/guides/authentication.md): [certificate](docs/guides/auth_client_certificate.md), [OIDC with GitHub Actions](docs/guides/auth_oidc_github.md), [OIDC with any platform](docs/guides/auth_oidc.md)
- [Import an existing tenant](docs/guides/import_existing.md)
- [Detect drift in CI](docs/guides/drift_detection.md)
- [Resources and availability](docs/guides/availability.md)
- [Troubleshooting](docs/guides/troubleshooting.md)
- [All Resources](docs/resources/)
- [All Data Sources](docs/data-sources/)

---

## Contributing

Contributions are welcome. Read the [contributing guide](CONTRIBUTING.md), then open an issue for a [bug](.github/ISSUE_TEMPLATE/bug_report.md), a [feature](.github/ISSUE_TEMPLATE/feature_request.md) or a [new resource](.github/ISSUE_TEMPLATE/new_resource.md).

## License

[MIT](LICENSE)
