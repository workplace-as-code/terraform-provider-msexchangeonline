data "msexchangeonline_service_principal" "mail_sender" {
  app_id = "00000000-0000-0000-0000-000000000000"
}

output "mail_sender_display_name" {
  value = data.msexchangeonline_service_principal.mail_sender.display_name
}
