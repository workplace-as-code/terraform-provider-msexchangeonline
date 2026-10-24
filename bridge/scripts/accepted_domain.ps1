# accepted_domains.ps1 — CRUD for Exchange Online accepted domains
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.
# Note: the domain must be verified in Entra ID/M365 before it can be added.
# Note: the default accepted domain cannot be deleted.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","List","Create","Update","Delete")] [string]$Action,

    [string]$Identity,
    [string]$Name,
    [string]$DomainName,
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$DomainType,       # Authoritative, ExternalRelay, InternalRelay
    [string]$MatchSubDomains
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

# Only the settings set in the Terraform configuration are sent.
$settings = ConvertTo-BridgeSplat $PSBoundParameters -String DomainType -Bool MatchSubDomains

function ConvertTo-BridgeAcceptedDomain {
    param($Domain)
    @{
        Identity        = [string]$Domain.Identity
        Name            = $Domain.Name
        DomainName      = $Domain.DomainName.ToString()
        DomainType      = $Domain.DomainType.ToString()
        Default         = [bool]$Domain.Default
        MatchSubDomains = [bool]$Domain.MatchSubDomains
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "List" {
        # Return all accepted domains as a JSON array
        $domains = Get-AcceptedDomain
        $result = @($domains | ForEach-Object {
            @{
                Identity        = [string]$_.Identity
                Name            = $_.Name
                DomainName      = $_.DomainName.ToString()
                DomainType      = $_.DomainType.ToString()
                Default         = [bool]$_.Default
                MatchSubDomains = [bool]$_.MatchSubDomains
            }
        })
        # -InputObject keeps a one-element array an array (piping would unwrap it).
        ConvertTo-Json -Compress -InputObject $result
    }

    "Get" {
        try {
            $domain = Get-AcceptedDomain -Identity $Identity -ErrorAction Stop
            ConvertTo-BridgeAcceptedDomain -Domain $domain
        } catch {
            # Only "object not found" means the resource is gone; any other error (auth,
            # throttling, network) must fail the run instead of dropping it from state.
            if ($_.Exception.Message -match "couldn't be found|not found|ObjectNotFound") { exit 0 }
            [Console]::Error.WriteLine("ERROR in accepted_domain Get: $($_.Exception.Message)")
            exit 1
        }
    }

    "Create" {
        # Exchange Online has no New-AcceptedDomain: a domain added and verified
        # in the Microsoft 365 admin center becomes an accepted domain, which
        # Create adopts.
        $domain = @(Get-AcceptedDomain | Where-Object { [string]$_.DomainName -eq $DomainName }) | Select-Object -First 1
        if ($null -eq $domain) {
            throw "Exchange Online has no accepted domain '$DomainName'. Add and verify the domain in the Microsoft 365 admin center first: it then becomes an accepted domain that this resource adopts."
        }
        if ([string]$domain.Name -cne $Name) {
            throw "The accepted domain '$DomainName' is named '$($domain.Name)' and Exchange Online cannot rename it: set name = `"$($domain.Name)`"."
        }
        if ($settings.Count -gt 0) { Set-AcceptedDomain -Identity ([string]$domain.Identity) @settings }
        ConvertTo-BridgeAcceptedDomain -Domain (Get-AcceptedDomain -Identity ([string]$domain.Identity))
    }

    "Update" {
        # DomainName is immutable: only DomainType and MatchSubDomains can change.
        if ($settings.Count -gt 0) { Set-AcceptedDomain -Identity $Identity @settings }

        $domain = Get-AcceptedDomain -Identity $Identity
        ConvertTo-BridgeAcceptedDomain -Domain $domain
    }

    "Delete" {
        # Exchange Online has no Remove-AcceptedDomain: the domain is removed in the
        # Microsoft 365 admin center. Terraform only forgets it.
        Write-Output "{}"
    }
}
