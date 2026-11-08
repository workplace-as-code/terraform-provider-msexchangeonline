data "msexchangeonline_ome_configuration" "default" {
  name = "OME Configuration"
}

output "otp_enabled" {
  description = "Whether recipients can open the portal with a one-time passcode."
  value       = data.msexchangeonline_ome_configuration.default.otp_enabled
}
