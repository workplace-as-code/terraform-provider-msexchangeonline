# The recipients of the French subsidiary
resource "msexchangeonline_management_scope" "france" {
  name                         = "France"
  recipient_restriction_filter = "CustomAttribute1 -eq 'France'"
}
