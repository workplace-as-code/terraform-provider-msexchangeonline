data "msexchangeonline_mobile_device_mailbox_policy" "default" {
  name = "Default"
}

output "min_password_length" {
  description = "The minimum device password length in the default policy."
  value       = data.msexchangeonline_mobile_device_mailbox_policy.default.min_password_length
}
