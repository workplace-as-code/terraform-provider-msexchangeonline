# Register an app of Microsoft Entra in Exchange Online, to give it scoped roles
resource "msexchangeonline_service_principal" "mail_sender" {
  app_id       = "00000000-0000-0000-0000-000000000000"
  object_id    = "11111111-1111-1111-1111-111111111111"
  display_name = "Mail sender"
}
