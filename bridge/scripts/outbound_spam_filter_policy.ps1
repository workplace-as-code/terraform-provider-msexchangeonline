# outbound_spam_filter_policy.ps1 — CRUD for Exchange Online outbound spam filter policies + rules
# Cmdlet nouns: HostedOutboundSpamFilterPolicy, HostedOutboundSpamFilterRule
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity                    = "",
    [string]$Name                        = "",
    [string]$RecipientLimitExternalPerHour,
    [string]$RecipientLimitInternalPerHour,
    [string]$RecipientLimitPerDay,
    [string]$ActionWhenThresholdReached,
    [string]$AutoForwardingMode,
    [string]$BccSuspiciousOutboundMail,
    [string]$BccSuspiciousOutboundAdditionalRecipients,
    [string]$NotifyOutboundSpam,
    [string]$NotifyOutboundSpamRecipients,
    [string]$Priority                    = "",
    [string]$Enabled,
    # Rule conditions: the rule matches on the sender
    [string]$SenderDomainIs,
    [string]$From,
    [string]$FromMemberOf
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId `
    -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword `
    -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"


function Get-PolicyWithRule {
    param([string]$IdentityValue)
    $policy = Get-HostedOutboundSpamFilterPolicy -Identity $IdentityValue
    $rule   = Get-HostedOutboundSpamFilterRule -Identity $IdentityValue -ErrorAction SilentlyContinue
    @{
        Identity                                     = [string]$policy.Identity
        Name                                         = [string]$policy.Name
        RecipientLimitExternalPerHour                = [int]$policy.RecipientLimitExternalPerHour
        RecipientLimitInternalPerHour                = [int]$policy.RecipientLimitInternalPerHour
        RecipientLimitPerDay                         = [int]$policy.RecipientLimitPerDay
        ActionWhenThresholdReached                   = [string]$policy.ActionWhenThresholdReached
        AutoForwardingMode                           = [string]$policy.AutoForwardingMode
        BccSuspiciousOutboundMail                    = [bool]$policy.BccSuspiciousOutboundMail
        BccSuspiciousOutboundAdditionalRecipients    = if ($policy.BccSuspiciousOutboundAdditionalRecipients) { [string[]]@($policy.BccSuspiciousOutboundAdditionalRecipients | ForEach-Object { $_.ToString() }) } else { @() }
        NotifyOutboundSpam                           = [bool]$policy.NotifyOutboundSpam
        NotifyOutboundSpamRecipients                 = if ($policy.NotifyOutboundSpamRecipients) { [string[]]@($policy.NotifyOutboundSpamRecipients | ForEach-Object { $_.ToString() }) } else { @() }
        Priority                                     = if ($rule) { [int]$rule.Priority } else { 0 }
        Enabled                                      = if ($rule) { $rule.State -eq "Enabled" } else { $true }
        SenderDomainIs                               = if ($rule) { @(ConvertTo-BridgeStrings $rule.SenderDomainIs) } else { @() }
        From                                         = if ($rule) { @(ConvertTo-BridgeStrings $rule.From) } else { @() }
        FromMemberOf                                 = if ($rule) { @(ConvertTo-BridgeStrings $rule.FromMemberOf) } else { @() }
    } | ConvertTo-Json -Compress
}

# Rule conditions. The rule exists only while an inclusion condition is set
# (see Sync-BridgeRule in common.ps1).
$RuleConditions = ConvertTo-BridgeConditions $PSBoundParameters -Csv SenderDomainIs, From, FromMemberOf

# Only the settings set in the Terraform configuration are sent.
$policySplat = ConvertTo-BridgeSplat $PSBoundParameters `
    -Int RecipientLimitExternalPerHour, RecipientLimitInternalPerHour, RecipientLimitPerDay `
    -String ActionWhenThresholdReached, AutoForwardingMode `
    -Bool BccSuspiciousOutboundMail, NotifyOutboundSpam `
    -Csv BccSuspiciousOutboundAdditionalRecipients, NotifyOutboundSpamRecipients
$ruleEnabled = if ($PSBoundParameters.ContainsKey("Enabled")) { $Enabled -eq "true" } else { $null }

switch ($Action) {

    "Get" {
        try {
            Get-PolicyWithRule -IdentityValue $Identity
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") { exit 0 }
            Write-Error "Failed to get outbound spam filter policy: $msg"; exit 1
        }
    }

    "Create" {
        New-HostedOutboundSpamFilterPolicy -Name $Name @policySplat | Out-Null

        # 2. Create the rule that scopes the policy, when a condition is set.
        try {
            Sync-BridgeRule -Noun "HostedOutboundSpamFilterRule" -PolicyParam "HostedOutboundSpamFilterPolicy" -Name $Name -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled
        } catch {
            # Roll back the policy so no orphan object is left behind.
            $ruleError = $_.Exception.Message
            Remove-HostedOutboundSpamFilterPolicy -Identity $Name -ErrorAction SilentlyContinue
            throw "Failed to create '$Name' (nothing was created): $ruleError"
        }


        Get-PolicyWithRule -IdentityValue $Name
    }

    "Update" {
        if ($policySplat.Count -gt 0) { Set-HostedOutboundSpamFilterPolicy -Identity $Identity @policySplat | Out-Null }

        # 2. Create, update or remove the rule to match the conditions.
        Sync-BridgeRule -Noun "HostedOutboundSpamFilterRule" -PolicyParam "HostedOutboundSpamFilterPolicy" -Name $Identity -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled


        Get-PolicyWithRule -IdentityValue $Identity
    }

    "Delete" {
        Remove-BridgeRule -Noun "HostedOutboundSpamFilterRule" -Name $Identity
        Remove-HostedOutboundSpamFilterPolicy -Identity $Identity
        Write-Output "{}"
    }
}
