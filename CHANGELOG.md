# Changelog

## 0.1.1 (Unreleased)

### Bug fixes

- Certificate authentication with a `.pfx` file works on macOS: loading the certificate failed with "This platform does not support loading with EphemeralKeySet".
- On a machine whose language is not English, a missing object is recognized again: Exchange Online now always answers in English, so `terraform query` no longer fails on a tenant without application access policies and objects deleted outside Terraform are removed from the state.

## 0.1.0 (October 24, 2026)

First release.

### Resources

- Mail flow: `msexchangeonline_transport_rule`, `msexchangeonline_remote_domain`, `msexchangeonline_accepted_domain`, `msexchangeonline_inbound_connector`, `msexchangeonline_outbound_connector`, `msexchangeonline_transport_config`, `msexchangeonline_dkim_signing_config`.
- Exchange Online Protection: `msexchangeonline_hosted_connection_filter_policy`, `msexchangeonline_tenant_allow_block_list_item`, `msexchangeonline_hosted_content_filter_policy`, `msexchangeonline_hosted_content_filter_rule`, `msexchangeonline_outbound_spam_filter_policy`, `msexchangeonline_hosted_outbound_spam_filter_rule`, `msexchangeonline_malware_filter_policy`, `msexchangeonline_malware_filter_rule`, `msexchangeonline_anti_phish_policy`, `msexchangeonline_anti_phish_rule`, `msexchangeonline_quarantine_policy`, `msexchangeonline_external_in_outlook`.
- Defender for Office 365: `msexchangeonline_safe_links_policy`, `msexchangeonline_safe_links_rule`, `msexchangeonline_safe_attachment_policy`, `msexchangeonline_safe_attachment_rule`. `safe_attachment_action` takes the values of the Defender portal, including `Off`.
- Clients, devices and organization: `msexchangeonline_organization_config`, `msexchangeonline_owa_mailbox_policy`, `msexchangeonline_mobile_device_mailbox_policy`, `msexchangeonline_email_address_policy`, `msexchangeonline_retention_policy`, `msexchangeonline_retention_policy_tag`, `msexchangeonline_application_access_policy`.

### Data sources

- `msexchangeonline_accepted_domain`, `msexchangeonline_accepted_domains`, `msexchangeonline_mailbox`, `msexchangeonline_organization_config`.

### Provider

- A `certificate_path` that does not point to a readable file fails at plan time with a clear message.
- Destroying a resource that cannot be deleted (tenant-wide settings, accepted domains) shows a warning: the object is only removed from the Terraform state and keeps its current settings.
- Existing configuration can be imported in bulk: every resource has a list resource for `terraform query` (Terraform 1.14+), which finds the existing objects and generates their configuration with `-generate-config-out`. Resources also have a resource identity, so `import` blocks accept `identity = { id = "..." }`.
- Exchange Online with app-only certificate authentication (`.pfx` file or certificate thumbprint), in the global, GCC, GCC High, DoD and China clouds.
- The PowerShell scripts are embedded in the provider binary. PowerShell sessions are kept open during a run, with a cap on concurrent sessions and a timeout per call (`EXCHANGE_MAX_CONCURRENCY`, `EXCHANGE_SCRIPT_TIMEOUT`, ...).
- `accepted_domain` adopts a domain added in the Microsoft 365 admin center, and destroying it only removes it from Terraform state: Exchange Online cannot create or remove accepted domains.
- Every resource only manages the attributes set in the configuration: an attribute left out is never written, so importing or adopting an existing object never resets one of its settings to a value chosen by the provider. New objects get the Exchange defaults.
- Policies scoped by recipients or senders create their rule only while a condition is set, so they can also be scoped by the standalone rule resources. A rule without `priority` gets the lowest priority instead of reordering the existing rules.
- `dkim_signing_config` adopts the existing DKIM configuration of a domain instead of failing, since a DKIM configuration cannot be deleted, and a `key_size` that Exchange cannot apply fails with a clear message.
- An imported `application_access_policy` reads its scope group as the group's primary email address, so a configuration that uses the address plans no change instead of replacing the policy.
- Clear error messages for tenants that need `Enable-OrganizationCustomization`, and automatic retry of transient Exchange Online errors: every read is sent again up to three times, and updates are retried in the scripts; a creation is never sent twice.
- `tools/new_role_group.ps1` creates a least-privilege Exchange role group for the provider's app registration. It only counts roles that grant all the parameters of a cmdlet, and skips the cmdlets the organization does not have.
