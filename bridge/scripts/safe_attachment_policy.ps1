# safe_attachment_policy.ps1 — CRUD for Defender for Office 365 Safe Attachments policies + rules
# Manages SafeAttachmentPolicy (settings) and SafeAttachmentRule (conditions) as a single unit.
# Cmdlet nouns: SafeAttachmentPolicy, SafeAttachmentRule
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
    # Policy settings
    # Only the settings set in the Terraform configuration are sent.
    [string]$SafeAttachmentAction,  # Off, Allow, Block or DynamicDelivery
    [string]$Redirect,
    [string]$RedirectAddress,
    [string]$QuarantineTag,
    # Rule conditions — lists passed as comma-separated strings from Go
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


# Safe Attachments requires Defender for Office 365 Plan 1 — fail early with a clear message
if (-not (Get-Command New-SafeAttachmentPolicy -ErrorAction SilentlyContinue)) {
    [Console]::Error.WriteLine("ERROR: Safe Attachments requires Defender for Office 365 Plan 1. The cmdlet New-SafeAttachmentPolicy is not available on this tenant.")
    exit 1
}



# Merge policy and rule into a single JSON object returned to Terraform
function Get-PolicyWithRule {
    param([string]$IdentityValue)

    $policy = Get-SafeAttachmentPolicy -Identity $IdentityValue
    $rule   = Get-SafeAttachmentRule -Identity $IdentityValue -ErrorAction SilentlyContinue

    @{
        Identity          = $policy.Identity
        Name              = $policy.Name
        # Without -Enable the policy scans nothing, whatever its Action: the
        # Defender portal shows it as Off.
        SafeAttachmentAction = if ($policy.Enable) { [string]$policy.Action } else { "Off" }
        Redirect          = $policy.Redirect
        RedirectAddress   = if ($policy.RedirectAddress) { $policy.RedirectAddress } else { "" }
        QuarantineTag     = if ($policy.QuarantineTag)   { [string]$policy.QuarantineTag } else { "" }
        Priority          = if ($rule) { $rule.Priority } else { 0 }
        Enabled           = if ($rule) { $rule.State -eq "Enabled" } else { $true }
        RecipientDomainIs         = if ($rule -and $rule.RecipientDomainIs)         { [string[]]@($rule.RecipientDomainIs         | ForEach-Object { $_.ToString() }) } else { @() }
        SentToMemberOf            = if ($rule -and $rule.SentToMemberOf)            { [string[]]@($rule.SentToMemberOf            | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfRecipientDomainIs = if ($rule -and $rule.ExceptIfRecipientDomainIs) { [string[]]@($rule.ExceptIfRecipientDomainIs | ForEach-Object { $_.ToString() }) } else { @() }
    } | ConvertTo-Json -Compress
}

# Only the settings set in the Terraform configuration are sent.
$policySplat = ConvertTo-BridgeSplat $PSBoundParameters -Bool Redirect -String RedirectAddress, QuarantineTag
# $Action is taken by the bridge: the cmdlet -Action arrives as SafeAttachmentAction.
# Off turns scanning off (-Enable $false); any other value turns it on with that action.
if ($PSBoundParameters.ContainsKey("SafeAttachmentAction")) {
    if ($SafeAttachmentAction -eq "Off") {
        $policySplat["Enable"] = $false
    } else {
        $policySplat["Enable"] = $true
        $policySplat["Action"] = $SafeAttachmentAction
    }
}
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
            [Console]::Error.WriteLine("ERROR in safe_attachment_policy Get: $msg")
            exit 1
        }
    }

    "Create" {
        New-SafeAttachmentPolicy -Name $Name @policySplat | Out-Null

        # 2. Create the rule that scopes the policy, when a condition is set.
        try {
            Sync-BridgeRule -Noun "SafeAttachmentRule" -PolicyParam "SafeAttachmentPolicy" -Name $Name -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled
        } catch {
            # Roll back the policy so no orphan object is left behind.
            $ruleError = $_.Exception.Message
            Remove-SafeAttachmentPolicy -Identity $Name -ErrorAction SilentlyContinue
            throw "Failed to create '$Name' (nothing was created): $ruleError"
        }


        Get-PolicyWithRule -IdentityValue $Name
    }

    "Update" {
        if ($policySplat.Count -gt 0) { Set-SafeAttachmentPolicy -Identity $Identity @policySplat }

        # 2. Create, update or remove the rule to match the conditions.
        Sync-BridgeRule -Noun "SafeAttachmentRule" -PolicyParam "SafeAttachmentPolicy" -Name $Identity -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled


        Get-PolicyWithRule -IdentityValue $Identity
    }

    "Delete" {
        Remove-BridgeRule -Noun "SafeAttachmentRule" -Name $Identity
        Remove-SafeAttachmentPolicy -Identity $Identity
        Write-Output "{}"
    }
}
