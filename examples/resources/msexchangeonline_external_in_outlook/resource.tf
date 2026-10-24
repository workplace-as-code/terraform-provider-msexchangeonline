# Show the External tag, except for a partner domain.
resource "msexchangeonline_external_in_outlook" "this" {
  enabled    = true
  allow_list = ["partner.example"]
}
