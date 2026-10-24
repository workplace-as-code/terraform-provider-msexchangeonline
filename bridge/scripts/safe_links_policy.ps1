# safe_links_policy.ps1 — CRUD for Defender for Office 365 Safe Links policies + rules
# Manages SafeLinksPolicy (settings) and SafeLinksRule (conditions) as a single unit.
# Cmdlet nouns: SafeLinksPolicy, SafeLinksRule
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity = "",
    [string]$Name = "",
    # Policy settings — booleans received as strings from Go ("true"/"false")
    [string]$EnableSafeLinksForEmail,
    [string]$EnableSafeLinksForTeams,
    [string]$EnableSafeLinksForOffice,
    [string]$TrackClicks,
    [string]$AllowClickThrough,
    [string]$ScanUrls,
    [string]$EnableForInternalSenders,
    [string]$DeliverMessageAfterScan,
    [string]$DisableUrlRewrite,
    # Lists passed as comma-separated strings from Go
    [string]$DoNotRewriteUrls,
    # Rule conditions
    [string]$Priority = "",
    [string]$Enabled,
    [string]$RecipientDomainIs,
    [string]$SentToMemberOf,
    [string]$ExceptIfRecipientDomainIs
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"


# Safe Links requires Defender for Office 365 Plan 1 — fail early with a clear message
if (-not (Get-Command New-SafeLinksPolicy -ErrorAction SilentlyContinue)) {
    [Console]::Error.WriteLine("ERROR: Safe Links requires Defender for Office 365 Plan 1. The cmdlet New-SafeLinksPolicy is not available on this tenant.")
    exit 1
}



# Merge policy and rule into a single JSON object returned to Terraform
function Get-PolicyWithRule {
    param([string]$IdentityValue)

    $policy = Get-SafeLinksPolicy -Identity $IdentityValue
    $rule   = Get-SafeLinksRule -Identity $IdentityValue -ErrorAction SilentlyContinue

    @{
        Identity                 = $policy.Identity
        Name                     = $policy.Name
        EnableSafeLinksForEmail  = $policy.EnableSafeLinksForEmail
        EnableSafeLinksForTeams  = $policy.EnableSafeLinksForTeams
        EnableSafeLinksForOffice = $policy.EnableSafeLinksForOffice
        TrackClicks              = $policy.TrackClicks
        AllowClickThrough        = $policy.AllowClickThrough
        ScanUrls                 = $policy.ScanUrls
        EnableForInternalSenders = $policy.EnableForInternalSenders
        DeliverMessageAfterScan  = $policy.DeliverMessageAfterScan
        DisableUrlRewrite        = $policy.DisableUrlRewrite
        DoNotRewriteUrls         = if ($policy.DoNotRewriteUrls) { [string[]]@($policy.DoNotRewriteUrls | ForEach-Object { $_.ToString() }) } else { @() }
        Priority                  = if ($rule) { $rule.Priority } else { 0 }
        Enabled                   = if ($rule) { $rule.State -eq "Enabled" } else { $true }
        RecipientDomainIs         = if ($rule -and $rule.RecipientDomainIs)         { [string[]]@($rule.RecipientDomainIs         | ForEach-Object { $_.ToString() }) } else { @() }
        SentToMemberOf            = if ($rule -and $rule.SentToMemberOf)            { [string[]]@($rule.SentToMemberOf            | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfRecipientDomainIs = if ($rule -and $rule.ExceptIfRecipientDomainIs) { [string[]]@($rule.ExceptIfRecipientDomainIs | ForEach-Object { $_.ToString() }) } else { @() }
    } | ConvertTo-Json -Compress
}

# Only the settings set in the Terraform configuration are sent.
$policySplat = ConvertTo-BridgeSplat $PSBoundParameters -Bool EnableSafeLinksForEmail, EnableSafeLinksForTeams, EnableSafeLinksForOffice, TrackClicks, AllowClickThrough, ScanUrls, EnableForInternalSenders, DeliverMessageAfterScan, DisableUrlRewrite -Csv DoNotRewriteUrls
$ruleEnabled = if ($PSBoundParameters.ContainsKey("Enabled")) { $Enabled -eq "true" } else { $null }

# Rule conditions. The rule exists only while an inclusion condition is set
# (see Sync-BridgeRule in common.ps1).
$RuleConditions = ConvertTo-BridgeConditions $PSBoundParameters -Csv RecipientDomainIs, SentToMemberOf, ExceptIfRecipientDomainIs

switch ($Action) {

    "Get" {
        try {
            Get-PolicyWithRule -IdentityValue $Identity
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") {
                exit 0  # policy deleted externally — Terraform removes it from state
            }
            [Console]::Error.WriteLine("ERROR in safe_links_policy Get: $msg")
            exit 1
        }
    }

    "Create" {
        New-SafeLinksPolicy -Name $Name @policySplat | Out-Null

        # 2. Create the rule that scopes the policy, when a condition is set.
        try {
            Sync-BridgeRule -Noun "SafeLinksRule" -PolicyParam "SafeLinksPolicy" -Name $Name -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled
        } catch {
            # Roll back the policy so no orphan object is left behind.
            $ruleError = $_.Exception.Message
            Remove-SafeLinksPolicy -Identity $Name -ErrorAction SilentlyContinue
            throw "Failed to create '$Name' (nothing was created): $ruleError"
        }


        Get-PolicyWithRule -IdentityValue $Name
    }

    "Update" {
        if ($policySplat.Count -gt 0) { Set-SafeLinksPolicy -Identity $Identity @policySplat }

        # 2. Create, update or remove the rule to match the conditions.
        Sync-BridgeRule -Noun "SafeLinksRule" -PolicyParam "SafeLinksPolicy" -Name $Identity -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled


        Get-PolicyWithRule -IdentityValue $Identity
    }

    "Delete" {
        Remove-BridgeRule -Noun "SafeLinksRule" -Name $Identity
        Remove-SafeLinksPolicy -Identity $Identity
        Write-Output "{}"
    }
}
