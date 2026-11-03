data "msexchangeonline_address_list" "all_users" {
  name = "All Users"
}

output "all_users_recipient_filter" {
  description = "Filter of the built-in list of every user."
  value       = data.msexchangeonline_address_list.all_users.recipient_filter
}
