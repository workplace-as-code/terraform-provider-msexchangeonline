# Read recipients and change their phone numbers, nothing else
resource "msexchangeonline_management_role" "helpdesk_phone" {
  name        = "Helpdesk Phone"
  parent      = "Mail Recipients"
  description = "First-level support: phone numbers only"

  cmdlets = ["Get-Mailbox", "Get-Recipient", "Get-User", "Set-User"]
  cmdlet_parameters = {
    "Set-User" = ["Identity", "Phone", "MobilePhone"]
  }
}
