data "msexchangeonline_quarantine_global_settings" "this" {}

output "quarantine_notification_frequency" {
  description = "How often users receive quarantine notifications."
  value       = data.msexchangeonline_quarantine_global_settings.this.notification_frequency
}
