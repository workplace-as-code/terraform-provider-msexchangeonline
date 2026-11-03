data "msexchangeonline_global_address_list" "default" {
  name = "Default Global Address List"
}

output "default_recipient_filter" {
  description = "Filter of the default GAL."
  value       = data.msexchangeonline_global_address_list.default.recipient_filter
}
