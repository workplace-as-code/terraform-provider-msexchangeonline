# The sales team of the Contoso entity, set in CustomAttribute1 and Department.
resource "msexchangeonline_address_list" "sales" {
  name             = "Contoso - Sales"
  recipient_filter = "CustomAttribute1 -eq 'Contoso' -and Department -eq 'Sales'"
}

# A list nested under the sales list.
resource "msexchangeonline_address_list" "sales_paris" {
  name             = "Contoso - Sales - Paris"
  container        = msexchangeonline_address_list.sales.name
  recipient_filter = "CustomAttribute1 -eq 'Contoso' -and City -eq 'Paris'"
}
