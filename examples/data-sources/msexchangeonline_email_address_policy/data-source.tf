data "msexchangeonline_email_address_policy" "default" {
  name = "Default Policy"
}

output "enabled_email_address_templates" {
  description = "The address templates of the default email address policy."
  value       = data.msexchangeonline_email_address_policy.default.enabled_email_address_templates
}
