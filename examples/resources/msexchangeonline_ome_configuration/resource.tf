# The default template, with the company logo
resource "msexchangeonline_ome_configuration" "default" {
  name             = "OME Configuration"
  image            = filebase64("${path.module}/logo.png")
  background_color = "#ffffff"
  disclaimer_text  = "This message is confidential."
}

# A custom template whose messages expire after 30 days
resource "msexchangeonline_ome_configuration" "finance" {
  name                         = "Finance"
  image                        = filebase64("${path.module}/logo.png")
  introduction_text            = "has sent you a secure message from Contoso Finance."
  external_mail_expiry_in_days = 30
}
