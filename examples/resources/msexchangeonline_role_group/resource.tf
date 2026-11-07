# A help desk role group; its members come from the identity management process
resource "msexchangeonline_role_group" "helpdesk_france" {
  name        = "Helpdesk France"
  description = "First-level support for the French subsidiary"
}
