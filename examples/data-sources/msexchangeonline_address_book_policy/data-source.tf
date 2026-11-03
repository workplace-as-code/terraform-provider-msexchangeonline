data "msexchangeonline_address_book_policy" "contoso" {
  name = "Contoso"
}

output "contoso_global_address_list" {
  description = "GAL of the Contoso address book policy."
  value       = data.msexchangeonline_address_book_policy.contoso.global_address_list
}
