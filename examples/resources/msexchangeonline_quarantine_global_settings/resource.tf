resource "msexchangeonline_quarantine_global_settings" "this" {
  notification_frequency        = "1.00:00:00"
  sender_address                = "quarantine@contoso.com"
  organization_branding_enabled = true

  languages = [
    {
      language    = "English"
      sender_name = "Contoso IT"
      subject     = "You have quarantined messages"
      disclaimer  = "Contact the Help Desk for more information."
    },
    {
      language    = "French"
      sender_name = "Service informatique Contoso"
      subject     = "Vous avez des messages en quarantaine"
      disclaimer  = "Contactez le service d'assistance pour plus d'informations."
    },
  ]
}
