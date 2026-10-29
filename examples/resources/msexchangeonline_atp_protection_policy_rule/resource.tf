# Safe Links and Safe Attachments of the Standard preset for the same group.
resource "msexchangeonline_atp_protection_policy_rule" "standard" {
  name              = "Standard Preset Security Policy"
  enabled           = true
  sent_to_member_of = ["Standard protection pilot"]
}
