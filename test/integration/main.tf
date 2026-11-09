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

variable "activesync_admin_mail_recipients" {
  description = "Set by test/run.sh to test a write to the ActiveSync settings, then to restore them. null leaves them unmanaged."
  type        = list(string)
  default     = null
}

variable "cas_mailbox_plan_pop_enabled" {
  description = "Set by test/run.sh to test a write to a CAS mailbox plan, then to restore it. null leaves it unmanaged."
  type        = bool
  default     = null
}

variable "mailbox_plan_retain_deleted_items_for_days" {
  description = "Set by test/run.sh to test a write to a mailbox plan, then to restore it. null leaves it unmanaged."
  type        = number
  default     = null
}

variable "organization_mail_tips_all_tips_enabled" {
  description = "Set by test/run.sh to test a write to MailTipsAllTipsEnabled, then to restore it. null leaves it unmanaged."
  type        = bool
  default     = null
}

variable "report_third_party" {
  description = "Set by test/run.sh, with test_mailbox, to test a non-Microsoft reporting tool, then restored. null leaves it unmanaged."
  type        = bool
  default     = null
}

variable "quarantine_notification_frequency" {
  description = "Set by test/run.sh to test a write to the global quarantine settings, then to restore it. null leaves it unmanaged."
  type        = string
  default     = null
}

variable "quarantine_sender_address" {
  description = "Set by test/run.sh, with test_mailbox, then restored. null leaves it unmanaged."
  type        = string
  default     = null
}

variable "quarantine_organization_branding_enabled" {
  description = "Set by test/run.sh, then restored. null leaves it unmanaged."
  type        = bool
  default     = null
}

variable "quarantine_languages" {
  description = "Set by test/run.sh to add customized notifications, then to remove them. null leaves them unmanaged."
  type = list(object({
    language    = string
    sender_name = optional(string)
    subject     = optional(string)
    disclaimer  = optional(string)
  }))
  default = null
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

variable "test_preset" {
  description = "Set by test/run.sh to test the rules of the Standard preset security policy, which must have been turned on once in the Defender portal. They are adopted, and turned off on destroy."
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

# Data sources built from resources, reading the built-in policies of every
# tenant (the Defender ones are in defender.tf).
data "msexchangeonline_anti_phish_policy" "default" {
  name = "Office365 AntiPhish Default"
}

data "msexchangeonline_hosted_content_filter_policy" "default" {
  name = "Default"
}

data "msexchangeonline_malware_filter_policy" "default" {
  name = "Default"
}

data "msexchangeonline_outbound_spam_filter_policy" "default" {
  name = "Default"
}

data "msexchangeonline_quarantine_policy" "default" {
  name = "DefaultFullAccessPolicy"
}

# A test object read back as a data source, once it exists.
data "msexchangeonline_malware_filter_policy" "test" {
  name       = msexchangeonline_malware_filter_policy.test.name
  depends_on = [msexchangeonline_malware_filter_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_malware_filter_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
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
resource "msexchangeonline_organization_config" "this" {
  mail_tips_all_tips_enabled = var.organization_mail_tips_all_tips_enabled
}

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

import {
  to = msexchangeonline_activesync_organization_settings.this
  id = "default"
}

resource "msexchangeonline_activesync_organization_settings" "this" {
  admin_mail_recipients = var.activesync_admin_mail_recipients
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

# Mailbox plans come with the licenses: adopted by display name, never created.
import {
  to = msexchangeonline_mailbox_plan.enterprise
  id = "ExchangeOnlineEnterprise"
}

resource "msexchangeonline_mailbox_plan" "enterprise" {
  name                          = "ExchangeOnlineEnterprise"
  retain_deleted_items_for_days = var.mailbox_plan_retain_deleted_items_for_days
}

import {
  to = msexchangeonline_cas_mailbox_plan.enterprise
  id = "ExchangeOnlineEnterprise"
}

resource "msexchangeonline_cas_mailbox_plan" "enterprise" {
  name        = "ExchangeOnlineEnterprise"
  pop_enabled = var.cas_mailbox_plan_pop_enabled
}

import {
  to = msexchangeonline_transport_config.this
  id = "default"
}

import {
  to = msexchangeonline_quarantine_global_settings.this
  id = "default"
}

import {
  to = msexchangeonline_report_submission_policy.this
  id = "default"
}

resource "msexchangeonline_report_submission_policy" "this" {
  reporting_mailbox                   = local.updated ? var.test_mailbox : ""
  enable_report_to_microsoft          = !local.updated
  enable_third_party_address          = var.report_third_party
  disable_quarantine_reporting_option = local.updated
  enable_user_email_notification      = local.updated
  enable_custom_notification_sender   = local.updated && var.test_mailbox != ""
  notification_sender_address         = local.updated ? var.test_mailbox : ""
  enable_organization_branding        = local.updated
  notification_footer_message         = local.comment
  junk_review_result_message          = local.comment
  not_junk_review_result_message      = local.comment
  phishing_review_result_message      = local.comment
  pre_submit_message_enabled          = !local.updated
  post_submit_message_enabled         = !local.updated
  pre_submit_message                  = local.comment
  pre_submit_message_title            = local.comment
  post_submit_message                 = local.comment
  post_submit_message_title           = local.comment

  languages = local.updated ? [
    { language = "en", pre_submit_junk_title = "TF-CI title" },
    { language = "fr", pre_submit_junk_message = "Contactez le service d'assistance : détails" },
    ] : [
    { language = "en", post_submit_phishing_title = "TF-CI created" },
  ]
}

resource "msexchangeonline_quarantine_global_settings" "this" {
  notification_frequency        = var.quarantine_notification_frequency
  sender_address                = var.quarantine_sender_address
  organization_branding_enabled = var.quarantine_organization_branding_enabled
  languages                     = var.quarantine_languages == null ? null : toset(var.quarantine_languages)
}

resource "msexchangeonline_transport_config" "this" {
  reply_all_storm_block_duration_hours = var.reply_all_storm_block_duration_hours
}

# Read only: turning the audit log on or off takes up to an hour and changes
# what the tenant records, so the suite never writes it.
import {
  to = msexchangeonline_admin_audit_log_config.this
  id = "default"
}

resource "msexchangeonline_admin_audit_log_config" "this" {}

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

# Recipient conditions and an exception; disabled, so no message is affected.
resource "msexchangeonline_transport_rule" "scope" {
  name    = "TF-CI-Rule-Scope"
  enabled = false
  # Created after TF-CI-Rule: two rules created at once race for priority 0.
  depends_on = [msexchangeonline_transport_rule.test]

  sent_to_scope                 = "NotInOrganization"
  recipient_domain_is           = local.updated ? ["tf-ci-test.example", "tf-ci-other.example"] : ["tf-ci-test.example"]
  except_if_recipient_domain_is = ["tf-ci-skip.example"]
  # null removes the condition (as [] does); [] would show a diff after import.
  subject_or_body_contains_words = local.updated ? null : ["[TF-CI-SCOPE]"]
  action                         = "MoveToJmf"
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
  quarantine_tag = local.updated ? "AdminOnlyAccessPolicy" : "DefaultFullAccessPolicy"
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

# Standard preset security policy, adopted and kept off: the test only changes
# its recipients. Destroying the resource turns the preset off.
resource "msexchangeonline_eop_protection_policy_rule" "standard" {
  count               = var.test_preset ? 1 : 0
  name                = "Standard Preset Security Policy"
  enabled             = false
  comments            = local.comment
  recipient_domain_is = [var.primary_domain]
  except_if_sent_to   = local.updated && var.test_mailbox != "" ? [var.test_mailbox] : []
}

# Disabled: a journal rule that delivers to an address that does not exist
# makes Microsoft contact the organization.
resource "msexchangeonline_journal_rule" "test" {
  name                  = "TF-CI-Journal"
  journal_email_address = "tf-ci-journal@tf-ci-test.example"
  scope                 = local.updated ? "External" : "Global"
  recipient             = local.updated && var.test_mailbox != "" ? var.test_mailbox : ""
  enabled               = false
}

resource "msexchangeonline_tenant_allow_block_list_item" "sender" {
  list_type = "Sender"
  entry     = "tf-ci-blocked@tf-ci-test.example"
  action    = "Block"
  notes     = local.comment
}

resource "msexchangeonline_tenant_allow_block_list_spoof_item" "test" {
  spoofed_user           = "tf-ci-spoof.example.com"
  sending_infrastructure = "tf-ci-infra.example.net"
  spoof_type             = "External"
  action                 = local.updated ? "Block" : "Allow"
}

resource "msexchangeonline_tenant_allow_block_list_spoof_item" "ip" {
  spoofed_user           = "tf-ci-spoof-ip.example.com"
  sending_infrastructure = "192.0.2.1/24"
  spoof_type             = "External"
  action                 = "Allow"
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

# Assigned to no mailbox: it has no effect. The update adds a domain with two
# actions, the case most likely to come back from Exchange in another form.
resource "msexchangeonline_sharing_policy" "test" {
  name = "TF-CI-SharingPolicy"
  domains = local.updated ? [
    "tf-ci-test.example:CalendarSharingFreeBusySimple",
    "tf-ci-partner.example:CalendarSharingFreeBusyDetail,ContactsSharing",
  ] : ["tf-ci-test.example:CalendarSharingFreeBusySimple"]
  enabled = !local.updated
}

# Matches a device model no phone reports: it applies to no device.
resource "msexchangeonline_activesync_device_access_rule" "test" {
  query_string   = "TF-CI-Device"
  characteristic = "DeviceModel"
  access_level   = local.updated ? "Quarantine" : "Block"
}

# Points to a domain that does not exist: Exchange stores the relationship
# without contacting the other organization.
resource "msexchangeonline_organization_relationship" "test" {
  name                     = "TF-CI-OrgRelationship"
  domain_names             = local.updated ? ["tf-ci-partner.example", "tf-ci-test.example"] : ["tf-ci-partner.example"]
  enabled                  = false
  free_busy_access_enabled = true
  free_busy_access_level   = local.updated ? "LimitedDetails" : "AvailabilityOnly"
  target_application_uri   = "outlook.com"
}

# ---------------------------------------------------------------------------
# Address book segmentation: every list selects CustomAttribute15 =
# "TF-CI-NOBODY", which no recipient has, so the lists stay empty. The filters
# are written without parentheses, which Exchange adds: the drift checks show
# that the configured text is kept.
# ---------------------------------------------------------------------------

resource "msexchangeonline_address_list" "test" {
  name         = "TF-CI-AddressList"
  display_name = local.updated ? "TF-CI Address List (updated)" : null
  recipient_filter = local.updated ? (
    "CustomAttribute15 -eq 'TF-CI-NOBODY' -and Department -eq 'TF-CI'"
  ) : "CustomAttribute15 -eq 'TF-CI-NOBODY'"
}

resource "msexchangeonline_address_list" "child" {
  name             = "TF-CI-AddressList-Child"
  container        = msexchangeonline_address_list.test.name
  recipient_filter = "CustomAttribute15 -eq 'TF-CI-NOBODY' -and City -eq 'TF-CI'"
}

resource "msexchangeonline_address_list" "rooms" {
  name             = "TF-CI-Rooms"
  recipient_filter = "RecipientDisplayType -eq 'ConferenceRoomMailbox' -and CustomAttribute15 -eq 'TF-CI-NOBODY'"
}

resource "msexchangeonline_global_address_list" "test" {
  name = "TF-CI-GAL"
  recipient_filter = local.updated ? (
    "CustomAttribute15 -eq 'TF-CI-NOBODY' -or CustomAttribute14 -eq 'TF-CI-NOBODY'"
  ) : "CustomAttribute15 -eq 'TF-CI-NOBODY'"
}

resource "msexchangeonline_offline_address_book" "test" {
  name          = "TF-CI-OAB"
  address_lists = [msexchangeonline_global_address_list.test.name]
}

resource "msexchangeonline_address_book_policy" "test" {
  name                 = "TF-CI-ABP"
  global_address_list  = msexchangeonline_global_address_list.test.name
  offline_address_book = msexchangeonline_offline_address_book.test.name
  room_list            = msexchangeonline_address_list.rooms.name
  address_lists = local.updated ? [
    msexchangeonline_address_list.test.name,
    msexchangeonline_address_list.child.name,
  ] : [msexchangeonline_address_list.test.name]
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
