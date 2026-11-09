# Data sources built from resources: each reads a test object back once it
# exists and fails the run if it got another object. The protection policy
# data sources are in main.tf and defender.tf.

data "msexchangeonline_transport_rule" "test" {
  name       = msexchangeonline_transport_rule.test.name
  depends_on = [msexchangeonline_transport_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_transport_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_remote_domain" "test" {
  name       = msexchangeonline_remote_domain.test.name
  depends_on = [msexchangeonline_remote_domain.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_remote_domain.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_inbound_connector" "test" {
  name       = msexchangeonline_inbound_connector.test.name
  depends_on = [msexchangeonline_inbound_connector.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_inbound_connector.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_outbound_connector" "test" {
  name       = msexchangeonline_outbound_connector.test.name
  depends_on = [msexchangeonline_outbound_connector.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_outbound_connector.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_anti_phish_rule" "test" {
  name       = msexchangeonline_anti_phish_rule.test.name
  depends_on = [msexchangeonline_anti_phish_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_anti_phish_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_malware_filter_rule" "test" {
  name       = msexchangeonline_malware_filter_rule.test.name
  depends_on = [msexchangeonline_malware_filter_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_malware_filter_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_hosted_content_filter_rule" "test" {
  name       = msexchangeonline_hosted_content_filter_rule.test.name
  depends_on = [msexchangeonline_hosted_content_filter_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_hosted_content_filter_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_hosted_outbound_spam_filter_rule" "test" {
  name       = msexchangeonline_hosted_outbound_spam_filter_rule.test.name
  depends_on = [msexchangeonline_hosted_outbound_spam_filter_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_hosted_outbound_spam_filter_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_journal_rule" "test" {
  name       = msexchangeonline_journal_rule.test.name
  depends_on = [msexchangeonline_journal_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_journal_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_owa_mailbox_policy" "test" {
  name       = msexchangeonline_owa_mailbox_policy.test.name
  depends_on = [msexchangeonline_owa_mailbox_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_owa_mailbox_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_mobile_device_mailbox_policy" "test" {
  name       = msexchangeonline_mobile_device_mailbox_policy.test.name
  depends_on = [msexchangeonline_mobile_device_mailbox_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_mobile_device_mailbox_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_email_address_policy" "test" {
  name       = msexchangeonline_email_address_policy.test.name
  depends_on = [msexchangeonline_email_address_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_email_address_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_retention_policy_tag" "test" {
  name       = msexchangeonline_retention_policy_tag.test.name
  depends_on = [msexchangeonline_retention_policy_tag.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_retention_policy_tag.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_retention_policy" "test" {
  name       = msexchangeonline_retention_policy.test.name
  depends_on = [msexchangeonline_retention_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_retention_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_dkim_signing_config" "test" {
  count       = var.test_dkim_domain != "" ? 1 : 0
  domain_name = msexchangeonline_dkim_signing_config.test[0].domain_name
  depends_on  = [msexchangeonline_dkim_signing_config.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_dkim_signing_config.test[0].id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_eop_protection_policy_rule" "standard" {
  count      = var.test_preset ? 1 : 0
  name       = msexchangeonline_eop_protection_policy_rule.standard[0].name
  depends_on = [msexchangeonline_eop_protection_policy_rule.standard]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_eop_protection_policy_rule.standard[0].id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_safe_links_rule" "test" {
  count      = local.defender
  name       = msexchangeonline_safe_links_rule.test[0].name
  depends_on = [msexchangeonline_safe_links_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_safe_links_rule.test[0].id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_safe_attachment_rule" "test" {
  count      = local.defender
  name       = msexchangeonline_safe_attachment_rule.test[0].name
  depends_on = [msexchangeonline_safe_attachment_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_safe_attachment_rule.test[0].id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_atp_protection_policy_rule" "standard" {
  count      = var.test_preset && var.test_defender ? 1 : 0
  name       = msexchangeonline_atp_protection_policy_rule.standard[0].name
  depends_on = [msexchangeonline_atp_protection_policy_rule.standard]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_atp_protection_policy_rule.standard[0].id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_sharing_policy" "test" {
  name       = msexchangeonline_sharing_policy.test.name
  depends_on = [msexchangeonline_sharing_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_sharing_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_organization_relationship" "test" {
  name       = msexchangeonline_organization_relationship.test.name
  depends_on = [msexchangeonline_organization_relationship.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_organization_relationship.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_address_book_policy" "test" {
  name       = msexchangeonline_address_book_policy.test.name
  depends_on = [msexchangeonline_address_book_policy.test]

  lifecycle {
    postcondition {
      condition     = self.global_address_list == msexchangeonline_global_address_list.test.name
      error_message = "The data source read another policy than the test one, or without its GAL."
    }
  }
}

# ---------------------------------------------------------------------------
# Hybrid: read-only objects of the Hybrid Configuration Wizard. The test tenant
# is not hybrid: an administrator created TF-CI-OnPrem and TF-CI-IOC once, with
# contoso values. test/run.sh sets test_hybrid when TF_VAR_test_hybrid is set.
# ---------------------------------------------------------------------------

variable "test_hybrid" {
  description = "Read the TF-CI-OnPrem and TF-CI-IOC hybrid objects."
  type        = bool
  default     = false
}

data "msexchangeonline_onpremises_organization" "test" {
  count = var.test_hybrid ? 1 : 0
  name  = "TF-CI-OnPrem"
}

data "msexchangeonline_intra_organization_connector" "test" {
  count = var.test_hybrid ? 1 : 0
  name  = "TF-CI-IOC"
}

check "hybrid_objects" {
  assert {
    condition = !var.test_hybrid || (
      length(one(data.msexchangeonline_onpremises_organization.test[*].hybrid_domains)) > 0 &&
      one(data.msexchangeonline_intra_organization_connector.test[*].discovery_endpoint) != ""
    )
    error_message = "The hybrid test objects could not be read."
  }
}

data "msexchangeonline_tenant_allow_block_list_spoof_item" "test" {
  id         = msexchangeonline_tenant_allow_block_list_spoof_item.test.id
  depends_on = [msexchangeonline_tenant_allow_block_list_spoof_item.test]

  lifecycle {
    postcondition {
      condition     = self.spoofed_user == "tf-ci-spoof.example.com"
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeonline_quarantine_global_settings" "this" {
  depends_on = [msexchangeonline_quarantine_global_settings.this]

  lifecycle {
    postcondition {
      condition     = self.id == "default"
      error_message = "The data source read another object than the tenant-wide settings."
    }
  }
}

data "msexchangeonline_report_submission_policy" "this" {
  depends_on = [msexchangeonline_report_submission_policy.this]

  lifecycle {
    postcondition {
      condition     = self.id == "default"
      error_message = "The data source read another object than the tenant-wide settings."
    }
  }
}
