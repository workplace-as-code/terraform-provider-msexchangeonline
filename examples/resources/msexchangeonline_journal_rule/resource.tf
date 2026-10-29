# Journal reports that cannot be delivered go to this address, required first.
resource "msexchangeonline_transport_config" "this" {
  journaling_report_ndr_to = "journal-ndr@archive.contoso.com"
}

# Journal the messages of every recipient to an external archiving service.
resource "msexchangeonline_journal_rule" "archive" {
  name                  = "Archive all mail"
  journal_email_address = "journal@archive.contoso.com"
  scope                 = "Global"
}
