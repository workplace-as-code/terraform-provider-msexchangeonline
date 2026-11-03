# Partners see free/busy times only; no anonymous calendar links.
resource "msexchangeonline_sharing_policy" "partners" {
  name = "Partners"
  domains = [
    "contoso.com:CalendarSharingFreeBusySimple",
    "fabrikam.com:CalendarSharingFreeBusyDetail,ContactsSharing",
  ]
}

# The policy of every mailbox without one of its own.
resource "msexchangeonline_sharing_policy" "default" {
  name    = "Default Sharing Policy"
  domains = ["*:CalendarSharingFreeBusySimple"]
}

import {
  to = msexchangeonline_sharing_policy.default
  id = "Default Sharing Policy"
}
