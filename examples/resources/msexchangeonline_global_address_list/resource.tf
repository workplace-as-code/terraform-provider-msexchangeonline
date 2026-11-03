# The GAL of the Contoso entity: its users, groups, rooms and contacts.
resource "msexchangeonline_global_address_list" "contoso" {
  name             = "Contoso - GAL"
  recipient_filter = "CustomAttribute1 -eq 'Contoso'"
}
