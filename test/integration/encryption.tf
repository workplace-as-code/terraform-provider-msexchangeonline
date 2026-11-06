# Message encryption. It needs Microsoft Purview Message Encryption (custom
# branding templates need Advanced Message Encryption, E5): test/run.sh sets
# test_encryption when EXCHANGE_TEST_ENCRYPTION is set.

variable "test_encryption" {
  description = "Set by test/run.sh to test message encryption."
  type        = bool
  default     = false
}

locals {
  encryption = var.test_encryption ? 1 : 0
}

# Encrypts mail to an external test domain; disabled, so no message is affected.
resource "msexchangeonline_transport_rule" "encrypt" {
  count   = local.encryption
  name    = "TF-CI-Rule-Encrypt"
  enabled = false

  sent_to_scope                                  = "NotInOrganization"
  recipient_domain_is                            = ["tf-ci-test.example"]
  apply_rights_protection_template               = local.updated ? "Do Not Forward" : "Encrypt"
  apply_rights_protection_customization_template = "OME Configuration"
}

variable "irm_enable_portal_tracking_logs" {
  description = "Set by test/run.sh to test a write to the IRM configuration, then to restore it. null leaves it unmanaged."
  type        = bool
  default     = null
}

# Tenant-wide: imported, written and restored by test/run.sh, never destroyed.
import {
  to = msexchangeonline_irm_configuration.this
  id = "default"
}

resource "msexchangeonline_irm_configuration" "this" {
  enable_portal_tracking_logs = var.irm_enable_portal_tracking_logs
}
