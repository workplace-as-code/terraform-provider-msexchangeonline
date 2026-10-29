data "msexchangeonline_anti_phish_policy" "default" {
  name = "Office365 AntiPhish Default"
}

output "enable_spoof_intelligence" {
  description = "Whether spoof intelligence is on in the default anti-phishing policy."
  value       = data.msexchangeonline_anti_phish_policy.default.enable_spoof_intelligence
}
