# Free/busy for every user, and MailTips for the members of one group, for the
# users of the Fabrikam tenant.
resource "msexchangeonline_cross_tenant_partner" "fabrikam" {
  partner_tenant_id = "00000000-0000-0000-0000-000000000000"

  capabilities = [
    {
      name    = "crossTenantCalendarAvailabilityBasic"
      allowed = true
    },
    {
      name            = "crossTenantMailTipsLimited"
      allowed         = true
      included_groups = ["11111111-1111-1111-1111-111111111111"]
    },
  ]
}
