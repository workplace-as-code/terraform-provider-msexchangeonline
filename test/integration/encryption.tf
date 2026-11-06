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
