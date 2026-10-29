data "msexchangeonline_eop_protection_policy_rule" "standard" {
  name = "Standard Preset Security Policy"
}

output "enabled" {
  description = "Whether the Standard preset security policy is on."
  value       = data.msexchangeonline_eop_protection_policy_rule.standard.enabled
}
