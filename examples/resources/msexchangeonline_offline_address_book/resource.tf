resource "msexchangeonline_global_address_list" "contoso" {
  name             = "Contoso - GAL"
  recipient_filter = "CustomAttribute1 -eq 'Contoso'"
}

# The offline address book of the Contoso entity: its GAL.
resource "msexchangeonline_offline_address_book" "contoso" {
  name          = "Contoso - OAB"
  address_lists = [msexchangeonline_global_address_list.contoso.name]
}
