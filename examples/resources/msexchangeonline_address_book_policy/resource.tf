# Address book segmentation: the mailboxes of the Contoso entity only see the
# recipients of Contoso. Assign the policy to the mailboxes outside Terraform
# (Set-Mailbox -AddressBookPolicy).
resource "msexchangeonline_global_address_list" "contoso" {
  name             = "Contoso - GAL"
  recipient_filter = "CustomAttribute1 -eq 'Contoso'"
}

resource "msexchangeonline_address_list" "contoso_users" {
  name             = "Contoso - Users"
  recipient_filter = "CustomAttribute1 -eq 'Contoso' -and RecipientType -eq 'UserMailbox'"
}

resource "msexchangeonline_address_list" "contoso_rooms" {
  name             = "Contoso - Rooms"
  recipient_filter = "CustomAttribute1 -eq 'Contoso' -and RecipientTypeDetails -eq 'RoomMailbox'"
}

resource "msexchangeonline_offline_address_book" "contoso" {
  name          = "Contoso - OAB"
  address_lists = [msexchangeonline_global_address_list.contoso.name]
}

resource "msexchangeonline_address_book_policy" "contoso" {
  name                 = "Contoso"
  global_address_list  = msexchangeonline_global_address_list.contoso.name
  offline_address_book = msexchangeonline_offline_address_book.contoso.name
  room_list            = msexchangeonline_address_list.contoso_rooms.name
  address_lists        = [msexchangeonline_address_list.contoso_users.name]
}
