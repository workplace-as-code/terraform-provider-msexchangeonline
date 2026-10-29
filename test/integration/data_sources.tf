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
  count       = length(msexchangeonline_dkim_signing_config.test)
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
  count      = length(msexchangeonline_eop_protection_policy_rule.standard)
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
  count      = length(msexchangeonline_atp_protection_policy_rule.standard)
  name       = msexchangeonline_atp_protection_policy_rule.standard[0].name
  depends_on = [msexchangeonline_atp_protection_policy_rule.standard]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeonline_atp_protection_policy_rule.standard[0].id
      error_message = "The data source read another object than the test one."
    }
  }
}
