# Free/busy times with a subsidiary on another Microsoft 365 tenant. The target
# values come from Get-FederationInformation -DomainName fabrikam.com.
locals {
  autodiscover = "https://autodiscover-s.outlook.com/autodiscover/autodiscover.svc/WSSecurity"
}

resource "msexchangeonline_organization_relationship" "fabrikam" {
  name                     = "Fabrikam"
  domain_names             = ["fabrikam.com", "fabrikam.mail.onmicrosoft.com"]
  free_busy_access_enabled = true
  free_busy_access_level   = "AvailabilityOnly"
  target_application_uri   = "outlook.com"
  target_autodiscover_epr  = local.autodiscover
}
