# Standard protection for a pilot group. The preset must have been turned on
# once in the Microsoft Defender portal, which creates this rule.
resource "msexchangeonline_eop_protection_policy_rule" "standard" {
  name              = "Standard Preset Security Policy"
  enabled           = true
  sent_to_member_of = ["Standard protection pilot"]
}
