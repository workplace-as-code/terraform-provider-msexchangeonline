terraform {
  required_providers {
    msexchangeonline = {
      source = "workplace-as-code/msexchangeonline"
    }
  }
}

# Auth is read from EXCHANGE_* env vars — no credentials in this file.
provider "msexchangeonline" {
}

variable "primary_domain" {
  description = "Initial .onmicrosoft.com domain of the tenant (e.g. contoso.onmicrosoft.com). Set via TF_VAR_primary_domain."
  type        = string
}

variable "test_mailbox" {
  description = "Email address of an existing mailbox to test the mailbox data source. Leave empty to skip."
  type        = string
  default     = ""
}

variable "phase" {
  description = "Set to \"update\" by test/run.sh to change one or more attributes of every resource in place."
  type        = string
  default     = "create"

  validation {
    condition     = contains(["create", "update"], var.phase)
    error_message = "phase must be \"create\" or \"update\"."
  }
}

variable "connection_filter_ip_block_list" {
  description = "Set by test/run.sh to test a write to the connection filter policy, then to restore it. null leaves it unmanaged."
  type        = list(string)
  default     = null
}

variable "external_tag_allow_list" {
  description = "Set by test/run.sh to test a write to the External tag settings, then to restore them. null leaves them unmanaged."
  type        = list(string)
  default     = null
}

variable "reply_all_storm_block_duration_hours" {
  description = "Set by test/run.sh to test a write to the transport config, then to restore it. null leaves it unmanaged."
  type        = number
  default     = null
}

variable "test_mail_enabled_group" {
  description = "Email address of an existing mail-enabled security group, to test application access policies. Leave empty to skip."
  type        = string
  default     = ""
}

variable "test_dkim_domain" {
  description = "Accepted domain whose DKIM signing configuration is tested. The configuration cannot be deleted: it is adopted when it exists, and disabled on destroy. Leave empty to skip."
  type        = string
  default     = ""
}

variable "test_dkim_enabled" {
  description = "Enable DKIM signing for test_dkim_domain. Publish its two CNAME records first: run.sh prints them."
  type        = bool
  default     = false
}

variable "test_app_id" {
  description = "Client ID of an app registration, to test application access policies with test_mail_enabled_group."
  type        = string
  default     = ""
}

locals {
  updated = var.phase == "update"
  comment = local.updated ? "[TF-CI-TEST] updated" : "[TF-CI-TEST] created"
}

# Defender for Office 365 resources are in defender.tf.

# ---------------------------------------------------------------------------
# Data sources
# ---------------------------------------------------------------------------

data "msexchangeonline_accepted_domain" "primary" {
  name = var.primary_domain
}

data "msexchangeonline_accepted_domains" "all" {}

data "msexchangeonline_organization_config" "tenant" {}

data "msexchangeonline_mailbox" "test" {
  count    = var.test_mailbox != "" ? 1 : 0
  identity = var.test_mailbox
}

# ---------------------------------------------------------------------------
# Tenant-wide objects: imported, never modified. test/run.sh removes them from
# state before the destroy, so the tenant keeps them.
# ---------------------------------------------------------------------------

# Exchange Online cannot create or remove accepted domains: creating the
# resource adopts the initial domain and destroying it only forgets it.
resource "msexchangeonline_accepted_domain" "initial" {
  name        = var.primary_domain
  domain_name = var.primary_domain
}

import {
  to = msexchangeonline_organization_config.this
  id = "organization"
}

# No attribute is set: Terraform only records the current values.
resource "msexchangeonline_organization_config" "this" {}

import {
  to = msexchangeonline_hosted_connection_filter_policy.this
  id = "Default"
}

resource "msexchangeonline_hosted_connection_filter_policy" "this" {
  ip_block_list = var.connection_filter_ip_block_list
}

import {
  to = msexchangeonline_external_in_outlook.this
  id = "default"
}

resource "msexchangeonline_external_in_outlook" "this" {
  allow_list = var.external_tag_allow_list
}

# Built-in objects adopted with a minimal configuration: the plan must show no
# change, since attributes left out of the configuration are never written.
import {
  to = msexchangeonline_remote_domain.default
  id = "Default"
}

resource "msexchangeonline_remote_domain" "default" {
  name        = "Default"
  domain_name = "*"
}

import {
  to = msexchangeonline_hosted_content_filter_policy.default
  id = "Default"
}

resource "msexchangeonline_hosted_content_filter_policy" "default" {
  name = "Default"
}

import {
  to = msexchangeonline_mobile_device_mailbox_policy.default
  id = "Default"
}

resource "msexchangeonline_mobile_device_mailbox_policy" "default" {
  name = "Default"
}

import {
  to = msexchangeonline_transport_config.this
  id = "default"
}

resource "msexchangeonline_transport_config" "this" {
  reply_all_storm_block_duration_hours = var.reply_all_storm_block_duration_hours
}

# ---------------------------------------------------------------------------
# Mail flow
# ---------------------------------------------------------------------------

resource "msexchangeonline_transport_rule" "test" {
  name     = "TF-CI-Rule"
  priority = 0
  enabled  = false

  subject_contains = local.updated ? ["[TF-CI-TEST]", "[TF-CI-UPDATED]"] : ["[TF-CI-TEST]"]
  action           = "Quarantine"
}

resource "msexchangeonline_remote_domain" "test" {
  name        = "TF-CI-Domain"
  domain_name = "tf-ci-test.example"

  auto_reply_enabled            = false
  auto_forward_enabled          = local.updated
  trusted_mail_inbound_enabled  = false
  trusted_mail_outbound_enabled = false
  allowed_oof_type              = "None"
}

resource "msexchangeonline_inbound_connector" "test" {
  name           = "TF-CI-Inbound"
  enabled        = false
  connector_type = "Partner"
  sender_domains = ["tf-ci-test.example"]
  require_tls    = local.updated
  comment        = local.comment
}

resource "msexchangeonline_outbound_connector" "test" {
  name              = "TF-CI-Outbound"
  enabled           = false
  connector_type    = "Partner"
  recipient_domains = ["tf-ci-test.example"]
  use_mx_record     = true
  tls_settings      = local.updated ? "CertificateValidation" : "EncryptionOnly"
  comment           = local.comment
}

# ---------------------------------------------------------------------------
# Anti-phishing: a policy scoped by its own rule, and a policy scoped by a
# standalone rule resource.
# ---------------------------------------------------------------------------

resource "msexchangeonline_anti_phish_policy" "test" {
  name = "TF-CI-AntiPhish"

  # The settings that need Defender for Office 365 are tested in defender.tf.
  enable_spoof_intelligence     = true
  authentication_fail_action    = local.updated ? "Quarantine" : "MoveToJmf"
  enable_unauthenticated_sender = true
  enable_via_tag                = true

  recipient_domain_is = [var.primary_domain]
  priority            = 0
  enabled             = false
}

resource "msexchangeonline_anti_phish_policy" "unscoped" {
  name                       = "TF-CI-AntiPhish-Unscoped"
  authentication_fail_action = local.updated ? "Quarantine" : "MoveToJmf"
}

resource "msexchangeonline_anti_phish_rule" "test" {
  name              = "TF-CI-AntiPhishRule"
  anti_phish_policy = msexchangeonline_anti_phish_policy.unscoped.name
  enabled           = false

  recipient_domain_is = [var.primary_domain]
  sent_to             = local.updated ? ["tf-ci@${var.primary_domain}"] : []
}

# ---------------------------------------------------------------------------
# Anti-malware
# ---------------------------------------------------------------------------

resource "msexchangeonline_malware_filter_policy" "test" {
  name = "TF-CI-MalwareFilter"

  zap_enabled        = true
  enable_file_filter = true
  file_types         = local.updated ? ["exe", "bat", "ps1", "vbs"] : ["exe", "bat", "ps1"]

  recipient_domain_is = [var.primary_domain]
  priority            = 0
  enabled             = false
}

resource "msexchangeonline_malware_filter_policy" "unscoped" {
  name        = "TF-CI-MalwareFilter-Unscoped"
  zap_enabled = !local.updated
}

resource "msexchangeonline_malware_filter_rule" "test" {
  name                  = "TF-CI-MalwareRule"
  malware_filter_policy = msexchangeonline_malware_filter_policy.unscoped.name
  enabled               = false

  recipient_domain_is           = [var.primary_domain]
  except_if_recipient_domain_is = local.updated ? ["tf-ci-test.example"] : []
}

# ---------------------------------------------------------------------------
# Anti-spam
# ---------------------------------------------------------------------------

resource "msexchangeonline_hosted_content_filter_policy" "test" {
  name = "TF-CI-SpamFilter"

  spam_action    = local.updated ? "Quarantine" : "MoveToJmf"
  bulk_threshold = local.updated ? 6 : 7

  recipient_domain_is = [var.primary_domain]
  priority            = 0
  enabled             = false
}

resource "msexchangeonline_hosted_content_filter_policy" "unscoped" {
  name           = "TF-CI-SpamFilter-Unscoped"
  bulk_threshold = local.updated ? 5 : 6
}

resource "msexchangeonline_hosted_content_filter_rule" "test" {
  name                         = "TF-CI-SpamRule"
  hosted_content_filter_policy = msexchangeonline_hosted_content_filter_policy.unscoped.name
  enabled                      = false

  recipient_domain_is = local.updated ? [var.primary_domain, "tf-ci-test.example"] : [var.primary_domain]
}

resource "msexchangeonline_outbound_spam_filter_policy" "test" {
  name = "TF-CI-OutboundSpam"

  action_when_threshold_reached = "BlockUser"
  auto_forwarding_mode          = local.updated ? "Off" : "Automatic"
  recipient_limit_per_day       = local.updated ? 900 : 1000

  sender_domain_is = [var.primary_domain]
  priority         = 0
  enabled          = false
}

# A policy without sender condition, scoped by a standalone rule.
resource "msexchangeonline_outbound_spam_filter_policy" "unscoped" {
  name                    = "TF-CI-OutboundSpam-Unscoped"
  recipient_limit_per_day = local.updated ? 800 : 700
}

resource "msexchangeonline_hosted_outbound_spam_filter_rule" "test" {
  name                               = "TF-CI-OutboundSpamRule"
  hosted_outbound_spam_filter_policy = msexchangeonline_outbound_spam_filter_policy.unscoped.name
  enabled                            = false
  sender_domain_is                   = [var.primary_domain]
  except_if_from                     = local.updated ? ["tf-ci@${var.primary_domain}"] : []
  comments                           = local.comment
}

resource "msexchangeonline_tenant_allow_block_list_item" "sender" {
  list_type = "Sender"
  entry     = "tf-ci-blocked@tf-ci-test.example"
  action    = "Block"
  notes     = local.comment
}

resource "msexchangeonline_tenant_allow_block_list_item" "url" {
  list_type       = "Url"
  entry           = "tf-ci-test.example/blocked"
  action          = "Block"
  expiration_date = local.updated ? "2030-02-01T00:00:00Z" : "2030-01-01T00:00:00Z"
}

resource "msexchangeonline_dkim_signing_config" "test" {
  count                   = var.test_dkim_domain != "" ? 1 : 0
  domain_name             = var.test_dkim_domain
  enabled                 = var.test_dkim_enabled
  header_canonicalization = local.updated ? "Simple" : "Relaxed"
  body_canonicalization   = "Relaxed"
}

resource "msexchangeonline_application_access_policy" "test" {
  count                 = var.test_mail_enabled_group != "" && var.test_app_id != "" ? 1 : 0
  app_id                = var.test_app_id
  policy_scope_group_id = var.test_mail_enabled_group
  access_right          = "RestrictAccess"
  description           = local.comment
}

resource "msexchangeonline_quarantine_policy" "test" {
  name                                  = "TF-CI-Quarantine"
  end_user_quarantine_permissions_value = local.updated ? 27 : 0
  quarantine_retention_days             = 30
}

# ---------------------------------------------------------------------------
# Clients and devices
# ---------------------------------------------------------------------------

resource "msexchangeonline_owa_mailbox_policy" "test" {
  name = "TF-CI-OWA"

  attachments_enabled             = true
  recover_deleted_items_enabled   = true
  rules_enabled                   = true
  signatures_enabled              = true
  instant_messaging_enabled       = false
  active_sync_integration_enabled = false
  public_folders_enabled          = false
  theme_selection_enabled         = local.updated

  wac_viewing_on_private_computers_enabled = true
  wac_viewing_on_public_computers_enabled  = false

  web_parts_frame_options_type = "SameOrigin"
  outbound_charset             = "AlwaysUTF8"

  blocked_file_types = local.updated ? ["exe", "bat", "ps1", "vbs"] : ["exe", "bat", "ps1"]
}

resource "msexchangeonline_mobile_device_mailbox_policy" "test" {
  name                = "TF-CI-Mobile"
  password_enabled    = true
  min_password_length = local.updated ? 8 : 6
  allow_camera        = !local.updated
}

# ---------------------------------------------------------------------------
# Organization
# ---------------------------------------------------------------------------

# Exchange Online: email address policies only apply to Microsoft 365 Groups.
resource "msexchangeonline_email_address_policy" "test" {
  name = "TF-CI-AddressPolicy"
  # Templates must use an accepted domain, and the test tenant has only one:
  # nothing is changed in the update phase.
  enabled_email_address_templates = ["SMTP:@${var.primary_domain}"]
  # Exchange only accepts a priority next to an existing one: 1 is the only
  # valid value when the tenant has no other custom policy.
  priority = 1
}

resource "msexchangeonline_retention_policy_tag" "test" {
  name                    = "TF-CI-RetentionTag"
  type                    = "Personal"
  age_limit_for_retention = local.updated ? 60 : 30
  retention_action        = "DeleteAndAllowRecovery"
  comment                 = local.comment
}

resource "msexchangeonline_retention_policy" "test" {
  name                       = "TF-CI-Retention"
  retention_policy_tag_links = local.updated ? ["1 Week Delete", msexchangeonline_retention_policy_tag.test.name] : ["1 Week Delete"]
}

# ---------------------------------------------------------------------------
# Outputs — visible after apply to confirm data sources work
# ---------------------------------------------------------------------------

output "primary_domain_name" {
  description = "Domain name read by the accepted domain data source."
  value       = data.msexchangeonline_accepted_domain.primary.domain_name
}

output "all_domain_names" {
  description = "Names of every accepted domain, read by the accepted domains data source."
  value       = [for d in data.msexchangeonline_accepted_domains.all.domains : d.name]
}

output "org_max_send_size_bytes" {
  description = "Maximum send size of the tenant, in bytes, read by the organization config data source."
  value       = data.msexchangeonline_organization_config.tenant.max_send_size
}

output "mailbox_display_name" {
  description = "Display name of the test mailbox, when TF_VAR_test_mailbox is set."
  value       = var.test_mailbox != "" ? data.msexchangeonline_mailbox.test[0].display_name : "(skipped — set TF_VAR_test_mailbox to enable)"
}
