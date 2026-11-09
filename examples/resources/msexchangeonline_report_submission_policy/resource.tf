# Users report with the Microsoft tools; reports go to Microsoft and to a mailbox.
resource "msexchangeonline_report_submission_policy" "this" {
  reporting_mailbox          = "reported@contoso.com"
  enable_report_to_microsoft = true

  languages = [
    {
      language              = "en"
      pre_submit_junk_title = "Report this message?"
    },
    {
      language              = "fr"
      pre_submit_junk_title = "Signaler ce message ?"
    },
  ]
}
