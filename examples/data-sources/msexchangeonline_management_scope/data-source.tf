data "msexchangeonline_management_scope" "france" {
  name = "France"
}

output "france_filter" {
  value = data.msexchangeonline_management_scope.france.recipient_restriction_filter
}
