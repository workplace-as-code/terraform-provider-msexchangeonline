data "msexchangeonline_report_submission_policy" "this" {}

output "reporting_mailbox" {
  description = "Mailbox that receives user reported messages."
  value       = data.msexchangeonline_report_submission_policy.this.reporting_mailbox
}
