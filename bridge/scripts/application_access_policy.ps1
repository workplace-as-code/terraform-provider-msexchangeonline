# application_access_policy.ps1 — CRUD for ApplicationAccessPolicy (Exchange
# Online). Set-ApplicationAccessPolicy only changes the description: the Go
# resource replaces the policy for any other change.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity           = "",
    [string]$AppId              = "",
    [string]$PolicyScopeGroupId = "",
    [string]$AccessRight        = "",
    [string]$Description,
    # "true" on the read that follows an import: also return the primary SMTP
    # address of the scope group, which the state does not hold yet.
    [string]$ResolveScope = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

function ConvertTo-BridgePolicy {
    param($Policy)
    $out = [ordered]@{
        Identity    = [string]$Policy.Identity
        AppId       = [string](Get-BridgeProp $Policy "AppId" "")
        AccessRight = [string](Get-BridgeProp $Policy "AccessRight" "")
        Description = [string](Get-BridgeProp $Policy "Description" "")
    }
    if ($ResolveScope -eq "true") {
        # The policy identity ends with the object ID of the scope group:
        # <tenant id>\<app id>:<group SID>;<group object ID>.
        $out["ScopeGroupEmail"] = ""
        if ([string]$Policy.Identity -match ';([0-9a-fA-F-]{36})$') {
            # REST cmdlet: faster, and PrimarySmtpAddress is in its default property set.
            $group = Get-EXORecipient -Identity $Matches[1] -ErrorAction SilentlyContinue
            if ($group) { $out["ScopeGroupEmail"] = [string]$group.PrimarySmtpAddress }
        }
    }
    $out | ConvertTo-Json -Compress
}

try {
    switch ($Action) {
        "Get" {
            try {
                ConvertTo-BridgePolicy (Get-ApplicationAccessPolicy -Identity $Identity -ErrorAction Stop)
            } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $p = @{ AppId = $AppId; PolicyScopeGroupId = $PolicyScopeGroupId; AccessRight = $AccessRight }
            if ($PSBoundParameters.ContainsKey("Description")) { $p["Description"] = $Description }
            $policy = New-ApplicationAccessPolicy @p
            ConvertTo-BridgePolicy (@($policy)[0])
        }

        "Update" {
            if ($PSBoundParameters.ContainsKey("Description")) {
                Set-ApplicationAccessPolicy -Identity $Identity -Description $Description
            }
            ConvertTo-BridgePolicy (Get-ApplicationAccessPolicy -Identity $Identity -ErrorAction Stop)
        }

        "Delete" {
            try {
                Remove-ApplicationAccessPolicy -Identity $Identity -ErrorAction Stop
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("application_access_policy $Action failed: $($_.Exception.Message)")
    exit 1
}
