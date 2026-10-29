data "msexchangeonline_transport_rule" "executables" {
  name = "Block executable attachments"
}

output "enabled" {
  description = "Whether the executable attachments rule applies."
  value       = data.msexchangeonline_transport_rule.executables.enabled
}
