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

# ---------------------------------------------------------------------------
# Accepted domain (read-only — managed at Entra ID level)
# ---------------------------------------------------------------------------

data "msexchangeonline_accepted_domain" "primary" {
  name = var.primary_domain
}

# ---------------------------------------------------------------------------
# Remote domains
# ---------------------------------------------------------------------------

resource "msexchangeonline_remote_domain" "internet" {
  name        = "Internet"
  domain_name = "*"

  auto_reply_enabled   = true
  auto_forward_enabled = false
  allowed_oof_type     = "External"
}

# ---------------------------------------------------------------------------
# Transport rules
# ---------------------------------------------------------------------------

resource "msexchangeonline_transport_rule" "block_executables" {
  name     = "Block executable attachments"
  priority = 1
  enabled  = true

  attachment_is_executable = true
  action                   = "Reject"
  reject_message           = "Executable attachments are not permitted by security policy."
}

resource "msexchangeonline_transport_rule" "block_spoofed_domain" {
  name     = "Block spoofed internal domain"
  priority = 2
  enabled  = true

  from_address_contains = ["@${data.msexchangeonline_accepted_domain.primary.domain_name}"]
  action                = "Reject"
  reject_message        = "External senders cannot use internal domain addresses."
}

# ---------------------------------------------------------------------------
# Anti-phishing
# ---------------------------------------------------------------------------

resource "msexchangeonline_anti_phish_policy" "standard" {
  name = "Standard-AntiPhish"

  phish_threshold_level                  = 2
  enable_spoof_intelligence              = true
  authentication_fail_action             = "MoveToJmf"
  enable_mailbox_intelligence            = true
  enable_mailbox_intelligence_protection = true
  mailbox_intelligence_protection_action = "MoveToJmf"
  enable_organization_domains_protection = true
  enable_unauthenticated_sender          = true
  enable_via_tag                         = true
  enable_first_contact_safety_tips       = true

  recipient_domain_is = [data.msexchangeonline_accepted_domain.primary.domain_name]
  priority            = 0
}

# ---------------------------------------------------------------------------
# Malware filter
# ---------------------------------------------------------------------------

resource "msexchangeonline_malware_filter_policy" "standard" {
  name = "Standard-MalwareFilter"

  zap_enabled        = true
  enable_file_filter = true
  file_types         = ["exe", "bat", "cmd", "ps1", "vbs", "js", "jar", "scr", "msi", "dll"]

  recipient_domain_is = [data.msexchangeonline_accepted_domain.primary.domain_name]
  priority            = 0
}

# ---------------------------------------------------------------------------
# Safe Links
# ---------------------------------------------------------------------------

resource "msexchangeonline_safe_links_policy" "standard" {
  name = "Standard-SafeLinks"

  enable_safe_links_for_email  = true
  enable_safe_links_for_teams  = true
  enable_safe_links_for_office = true
  scan_urls                    = true
  deliver_message_after_scan   = true
  track_clicks                 = true
  allow_click_through          = false
  enable_for_internal_senders  = true

  recipient_domain_is = [data.msexchangeonline_accepted_domain.primary.domain_name]
  priority            = 0
}

# ---------------------------------------------------------------------------
# Safe Attachments
# ---------------------------------------------------------------------------

resource "msexchangeonline_safe_attachment_policy" "standard" {
  name = "Standard-SafeAttachments"

  safe_attachment_action = "DynamicDelivery"

  recipient_domain_is = [data.msexchangeonline_accepted_domain.primary.domain_name]
  priority            = 0
}

# ---------------------------------------------------------------------------
# OWA Mailbox Policy
# ---------------------------------------------------------------------------

resource "msexchangeonline_owa_mailbox_policy" "standard" {
  name = "Standard-OWA"

  attachments_enabled             = true
  recover_deleted_items_enabled   = true
  rules_enabled                   = true
  signatures_enabled              = true
  instant_messaging_enabled       = false
  active_sync_integration_enabled = false
  public_folders_enabled          = false
  theme_selection_enabled         = false

  wac_viewing_on_private_computers_enabled = true
  wac_viewing_on_public_computers_enabled  = false

  web_parts_frame_options_type = "SameOrigin"
  outbound_charset             = "AlwaysUTF8"

  blocked_file_types = ["exe", "bat", "cmd", "ps1", "vbs", "js", "jar", "scr"]
}

# ---------------------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------------------

output "primary_domain" {
  value       = data.msexchangeonline_accepted_domain.primary.domain_name
  description = "The primary accepted domain of the tenant."
}

output "primary_domain_type" {
  value       = data.msexchangeonline_accepted_domain.primary.domain_type
  description = "The type of the primary accepted domain (Authoritative, ExternalRelay, InternalRelay)."
}
