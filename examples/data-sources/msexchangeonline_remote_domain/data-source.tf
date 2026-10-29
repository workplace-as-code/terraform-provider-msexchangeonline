data "msexchangeonline_remote_domain" "default" {
  name = "Default"
}

output "auto_forward_enabled" {
  description = "Whether mail can be forwarded automatically to external domains."
  value       = data.msexchangeonline_remote_domain.default.auto_forward_enabled
}
