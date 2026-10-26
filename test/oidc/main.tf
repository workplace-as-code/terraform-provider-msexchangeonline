# Read-only check of OIDC authentication, run by .github/workflows/oidc.yml.
# The plan reads data sources, then terraform query lists several types at
# once, so several PowerShell processes each connect with the access token.
# Nothing is created or changed in the tenant.
terraform {
  required_providers {
    msexchangeonline = {
      source = "workplace-as-code/msexchangeonline"
    }
  }
}

# Authentication comes from the EXCHANGE_* environment variables of the job.
provider "msexchangeonline" {}

data "msexchangeonline_organization_config" "this" {}

data "msexchangeonline_accepted_domains" "all" {}

output "accepted_domain_count" {
  value = length(data.msexchangeonline_accepted_domains.all.domains)
}
