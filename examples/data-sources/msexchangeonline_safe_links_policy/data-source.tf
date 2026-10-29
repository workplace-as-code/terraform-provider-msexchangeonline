data "msexchangeonline_safe_links_policy" "default" {
  name = "Built-In Protection Policy"
}

output "scan_urls" {
  description = "Whether the built-in Safe Links policy scans URLs in real time."
  value       = data.msexchangeonline_safe_links_policy.default.scan_urls
}
