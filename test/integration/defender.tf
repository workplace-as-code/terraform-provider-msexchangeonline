# Defender for Office 365 resources. They need a Defender for Office 365 Plan 1
# license: test/run.sh sets test_defender when EXCHANGE_TEST_DEFENDER is set.

variable "test_defender" {
  description = "Set by test/run.sh to test the Defender for Office 365 resources."
  type        = bool
  default     = false
}

locals {
  defender = var.test_defender ? 1 : 0
}

# ---------------------------------------------------------------------------
# Safe Links: a policy scoped by its own rule, and a policy scoped by a
# standalone rule resource.
# ---------------------------------------------------------------------------

resource "msexchangeonline_safe_links_policy" "test" {
  count = local.defender
  name  = "TF-CI-SafeLinks"

  enable_safe_links_for_email = true
  scan_urls                   = true
  track_clicks                = !local.updated
  allow_click_through         = false
  do_not_rewrite_urls         = local.updated ? ["https://tf-ci-test.example/*", "https://intranet.tf-ci-test.example/*"] : ["https://tf-ci-test.example/*"]

  recipient_domain_is = [var.primary_domain]
  priority            = 0
  enabled             = false
}

resource "msexchangeonline_safe_links_policy" "unscoped" {
  count                       = local.defender
  name                        = "TF-CI-SafeLinks-Unscoped"
  enable_safe_links_for_teams = local.updated
}

resource "msexchangeonline_safe_links_rule" "test" {
  count             = local.defender
  name              = "TF-CI-SafeLinksRule"
  safe_links_policy = msexchangeonline_safe_links_policy.unscoped[0].name
  enabled           = false

  recipient_domain_is           = [var.primary_domain]
  except_if_recipient_domain_is = local.updated ? ["tf-ci-test.example"] : []
}

# ---------------------------------------------------------------------------
# Safe Attachments
# ---------------------------------------------------------------------------

resource "msexchangeonline_safe_attachment_policy" "test" {
  count                  = local.defender
  name                   = "TF-CI-SafeAttachments"
  safe_attachment_action = local.updated ? "Block" : "DynamicDelivery"

  recipient_domain_is = [var.primary_domain]
  priority            = 0
  enabled             = false
}

resource "msexchangeonline_safe_attachment_policy" "unscoped" {
  count                  = local.defender
  name                   = "TF-CI-SafeAttachments-Unscoped"
  safe_attachment_action = local.updated ? "Allow" : "Off"
}

resource "msexchangeonline_safe_attachment_rule" "test" {
  count                  = local.defender
  name                   = "TF-CI-SafeAttachmentRule"
  safe_attachment_policy = msexchangeonline_safe_attachment_policy.unscoped[0].name
  enabled                = false

  recipient_domain_is = local.updated ? [var.primary_domain, "tf-ci-test.example"] : [var.primary_domain]
}

# ---------------------------------------------------------------------------
# Anti-phishing settings that need Defender for Office 365
# ---------------------------------------------------------------------------

resource "msexchangeonline_anti_phish_policy" "defender" {
  count = local.defender
  name  = "TF-CI-AntiPhish-Defender"

  phish_threshold_level                  = local.updated ? 2 : 1
  enable_mailbox_intelligence            = true
  enable_mailbox_intelligence_protection = local.updated
  mailbox_intelligence_protection_action = "MoveToJmf"
  enable_organization_domains_protection = !local.updated
}
