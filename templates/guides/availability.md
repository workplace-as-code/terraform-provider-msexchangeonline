---
page_title: "Resources and Availability"
subcategory: "Guides"
---

# Resources and Availability

This guide lists every resource of the provider and the license it needs. Every resource is covered by the integration tests against a real tenant, except the ones marked otherwise.

Exchange Server is managed by a separate provider, [workplace-as-code/msexchangeserver](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest).

See the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication) to set up the app registration.

## Mail flow

| Resource | Notes |
|---|---|
| [`msexchangeonline_transport_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/transport_rule) | Mail flow rules |
| [`msexchangeonline_remote_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/remote_domain) | Auto-reply, auto-forward and formatting per external domain |
| [`msexchangeonline_accepted_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/accepted_domain) | Adopts a domain added in the Microsoft 365 admin center; destroying only removes it from state |
| [`msexchangeonline_inbound_connector`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/inbound_connector) |  |
| [`msexchangeonline_outbound_connector`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/outbound_connector) |  |
| [`msexchangeonline_transport_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/transport_config) | Singleton: SMTP AUTH, reply-all storm protection, ... |
| [`msexchangeonline_dkim_signing_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/dkim_signing_config) | Needs a custom domain and its DNS records (not covered by the integration tests) |

## Anti-spam, anti-malware and anti-phishing (Exchange Online Protection)

| Resource | Notes |
|---|---|
| [`msexchangeonline_hosted_connection_filter_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/hosted_connection_filter_policy) | Singleton: IP allow and block lists |
| [`msexchangeonline_tenant_allow_block_list_item`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/tenant_allow_block_list_item) | Blocked or allowed senders, URLs, file hashes, IP addresses |
| [`msexchangeonline_hosted_content_filter_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/hosted_content_filter_policy) | Inbound spam filter |
| [`msexchangeonline_hosted_content_filter_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/hosted_content_filter_rule) |  |
| [`msexchangeonline_outbound_spam_filter_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/outbound_spam_filter_policy) | Sending limits and automatic forwarding |
| [`msexchangeonline_hosted_outbound_spam_filter_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/hosted_outbound_spam_filter_rule) |  |
| [`msexchangeonline_malware_filter_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/malware_filter_policy) |  |
| [`msexchangeonline_malware_filter_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/malware_filter_rule) |  |
| [`msexchangeonline_anti_phish_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/anti_phish_policy) | Impersonation settings need Defender for Office 365 |
| [`msexchangeonline_anti_phish_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/anti_phish_rule) |  |
| [`msexchangeonline_quarantine_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/quarantine_policy) |  |
| [`msexchangeonline_external_in_outlook`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/external_in_outlook) | Singleton: the External tag in Outlook |

New Exchange Online tenants must be customized once before custom policies can be created: an Exchange administrator runs `Enable-OrganizationCustomization`. The provider reports it when it is needed.

## Defender for Office 365 (Plan 1 or 2 license)

| Resource | Notes |
|---|---|
| [`msexchangeonline_safe_links_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/safe_links_policy) | Needs Defender for Office 365 |
| [`msexchangeonline_safe_links_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/safe_links_rule) | Needs Defender for Office 365 |
| [`msexchangeonline_safe_attachment_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/safe_attachment_policy) | Needs Defender for Office 365 |
| [`msexchangeonline_safe_attachment_rule`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/safe_attachment_rule) | Needs Defender for Office 365 |

## Clients, devices and organization

| Resource | Notes |
|---|---|
| [`msexchangeonline_organization_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/organization_config) | Singleton |
| [`msexchangeonline_owa_mailbox_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/owa_mailbox_policy) | Outlook on the web |
| [`msexchangeonline_mobile_device_mailbox_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/mobile_device_mailbox_policy) | Exchange ActiveSync devices |
| [`msexchangeonline_email_address_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/email_address_policy) | Microsoft 365 Groups only |
| [`msexchangeonline_retention_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/retention_policy) | Messaging records management |
| [`msexchangeonline_retention_policy_tag`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/retention_policy_tag) |  |
| [`msexchangeonline_application_access_policy`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/application_access_policy) | Limits the mailboxes a Microsoft Graph app can access |

## Data sources

| Data source |
|---|
| [`msexchangeonline_accepted_domain`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/data-sources/accepted_domain) |
| [`msexchangeonline_accepted_domains`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/data-sources/accepted_domains) |
| [`msexchangeonline_mailbox`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/data-sources/mailbox) |
| [`msexchangeonline_organization_config`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/data-sources/organization_config) |

## Singletons

Tenant-wide settings (`organization_config`, `transport_config`, `hosted_connection_filter_policy`, `external_in_outlook`) always exist. Creating the resource adopts the current settings, and destroying it only removes it from Terraform state. Only the attributes set in the configuration are managed: the others keep their current value. Use an `import` block to start from the current settings:

```terraform
import {
  to = msexchangeonline_transport_config.this
  id = "default"
}

resource "msexchangeonline_transport_config" "this" {
  smtp_client_authentication_disabled = true
}
```

## Policies and rules

Exchange Online Protection scopes each protection policy with a rule that lists the recipients (or, for outbound spam, the senders) it applies to. A policy is linked to at most one rule.

The policy resources can manage their rule themselves: set a condition such as `recipient_domain_is` on the policy and the provider creates a rule with the same name. Without any condition, the policy has no rule of its own and you scope it with the standalone rule resource:

```terraform
# Policy scoped by its own rule
resource "msexchangeonline_malware_filter_policy" "finance" {
  name                = "Finance"
  enable_file_filter  = true
  recipient_domain_is = ["finance.contoso.com"]
}

# Policy scoped by a standalone rule
resource "msexchangeonline_malware_filter_policy" "executives" {
  name               = "Executives"
  enable_file_filter = true
}

resource "msexchangeonline_malware_filter_rule" "executives" {
  name                  = "Executives"
  malware_filter_policy = msexchangeonline_malware_filter_policy.executives.name
  sent_to_member_of     = ["executives@contoso.com"]
}
```

| Policy | Standalone rule |
|---|---|
| `msexchangeonline_anti_phish_policy` | `msexchangeonline_anti_phish_rule` |
| `msexchangeonline_malware_filter_policy` | `msexchangeonline_malware_filter_rule` |
| `msexchangeonline_hosted_content_filter_policy` | `msexchangeonline_hosted_content_filter_rule` |
| `msexchangeonline_outbound_spam_filter_policy` | `msexchangeonline_hosted_outbound_spam_filter_rule` |
| `msexchangeonline_safe_links_policy` | `msexchangeonline_safe_links_rule` |
| `msexchangeonline_safe_attachment_policy` | `msexchangeonline_safe_attachment_rule` |

When `priority` is omitted, Exchange gives the rule the lowest priority, so adding a rule never reorders the existing ones.
