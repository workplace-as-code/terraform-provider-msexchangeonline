data "msexchangeonline_anti_phish_rule" "executives" {
  name = "Anti-Phish Rule - Executives"
}

output "enabled" {
  description = "Whether the anti-phishing rule for executives applies."
  value       = data.msexchangeonline_anti_phish_rule.executives.enabled
}
