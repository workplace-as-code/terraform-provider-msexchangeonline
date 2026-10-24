# Exchange Online: email address policies only apply to Microsoft 365 Groups.
# New groups get an address in the groups subdomain.
resource "msexchangeonline_email_address_policy" "groups" {
  name                            = "Microsoft 365 Groups"
  enabled_email_address_templates = ["SMTP:@groups.contoso.com"]
  priority                        = 1
}
