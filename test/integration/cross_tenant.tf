# Microsoft 365 cross-tenant access policy of a partner tenant. Microsoft Graph
# refuses these capabilities to an app without the Entra role Exchange
# Administrator: they go through a second app that has it, so the main suite
# keeps proving that the role group alone is enough for every other resource.
# test/run.sh enables it when TF_VAR_test_partner_tenant_id and
# TF_VAR_xtap_client_id are set.

variable "test_partner_tenant_id" {
  description = "Tenant ID of a partner whose Exchange capabilities the test may own (it leaves none)."
  type        = string
  default     = ""
}

variable "test_group_object_id" {
  description = "Object ID of a group the MailTips capability is limited to in the update phase (optional)."
  type        = string
  default     = ""
}

variable "xtap_client_id" {
  description = "Client ID of the app with the Entra role Exchange Administrator. null uses the main app."
  type        = string
  default     = null
}

variable "xtap_certificate_path" {
  description = "Path to the .pfx certificate of that app."
  type        = string
  default     = null
}

variable "xtap_certificate_password" {
  description = "Password of that certificate."
  type        = string
  default     = null
  sensitive   = true
}

locals {
  cross_tenant = var.test_partner_tenant_id != "" && var.xtap_client_id != null ? 1 : 0
}

provider "msexchangeonline" {
  alias                = "xtap"
  client_id            = var.xtap_client_id
  certificate_path     = var.xtap_certificate_path
  certificate_password = var.xtap_certificate_password
}

resource "msexchangeonline_cross_tenant_partner" "test" {
  count             = local.cross_tenant
  provider          = msexchangeonline.xtap
  partner_tenant_id = var.test_partner_tenant_id

  capabilities = concat(
    [{ name = "crossTenantCalendarAvailabilityBasic", allowed = true }],
    local.updated ? [{
      name            = "crossTenantMailTipsLimited"
      allowed         = true
      included_groups = var.test_group_object_id != "" ? [var.test_group_object_id] : null
    }] : [],
  )
}

data "msexchangeonline_cross_tenant_partner" "test" {
  count             = local.cross_tenant
  provider          = msexchangeonline.xtap
  partner_tenant_id = msexchangeonline_cross_tenant_partner.test[0].partner_tenant_id
  depends_on        = [msexchangeonline_cross_tenant_partner.test]

  lifecycle {
    postcondition {
      condition     = length(self.capabilities) == length(msexchangeonline_cross_tenant_partner.test[0].capabilities)
      error_message = "The data source read other capabilities than the resource."
    }
  }
}
