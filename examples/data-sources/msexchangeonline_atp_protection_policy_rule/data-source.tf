data "msexchangeonline_atp_protection_policy_rule" "standard" {
  name = "Standard Preset Security Policy"
}

output "enabled" {
  description = "Whether the Defender part of the Standard preset is on."
  value       = data.msexchangeonline_atp_protection_policy_rule.standard.enabled
}
