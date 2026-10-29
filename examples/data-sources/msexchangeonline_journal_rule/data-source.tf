data "msexchangeonline_journal_rule" "archive" {
  name = "Archive all mail"
}

output "journal_email_address" {
  description = "The address that receives the journal reports."
  value       = data.msexchangeonline_journal_rule.archive.journal_email_address
}
