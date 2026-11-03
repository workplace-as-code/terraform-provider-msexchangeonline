data "msexchangeonline_offline_address_book" "default" {
  name = "Default Offline Address Book"
}

output "default_address_lists" {
  description = "Address lists of the default offline address book."
  value       = data.msexchangeonline_offline_address_book.default.address_lists
}
