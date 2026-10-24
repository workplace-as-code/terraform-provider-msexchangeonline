# anti_phish_policy.ps1 — CRUD for Exchange Online anti-phishing policies + rules
# Manages AntiPhishPolicy (settings) and AntiPhishRule (conditions) as a single unit.
# Cmdlet nouns: AntiPhishPolicy, AntiPhishRule
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
    # Settings: only the ones set in the Terraform configuration are sent.
    # Booleans arrive as "true"/"false", lists as comma-separated strings.
    [string]$PhishThresholdLevel,
    [string]$EnableSpoofIntelligence,
    [string]$AuthenticationFailAction,
    [string]$EnableMailboxIntelligence,
    [string]$EnableMailboxIntelligenceProtection,
    [string]$MailboxIntelligenceProtectionAction,
    [string]$EnableOrganizationDomainsProtection,
    [string]$EnableUnauthenticatedSender,
    [string]$EnableViaTag,
    [string]$EnableFirstContactSafetyTips,
    # Rule
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


$policySplat = ConvertTo-BridgeSplat $PSBoundParameters -Int PhishThresholdLevel -String AuthenticationFailAction, MailboxIntelligenceProtectionAction `
    -Bool EnableSpoofIntelligence, EnableMailboxIntelligence, EnableMailboxIntelligenceProtection, EnableOrganizationDomainsProtection, EnableUnauthenticatedSender, EnableViaTag, EnableFirstContactSafetyTips
# Some settings only exist with Microsoft Defender for Office 365.
Assert-BridgeParameters -Splat $policySplat -Cmdlet New-AntiPhishPolicy

$ruleEnabled = if ($PSBoundParameters.ContainsKey("Enabled")) { $Enabled -eq "true" } else { $null }

# Safe property getter — avoids StrictMode errors on built-in policies that omit some properties
function Get-PropSafe {
    param($Obj, [string]$Prop, $Default = $null)
    $p = $Obj.PSObject.Properties[$Prop]
    if ($null -ne $p) { return $p.Value }
    return $Default
}

# Merge policy and rule into a single JSON object returned to Terraform
function Get-PolicyWithRule {
    param([string]$IdentityValue)

    $policy = Get-AntiPhishPolicy -Identity $IdentityValue
    # Rule has the same name as the policy (enforced on Create); built-in default has no rule
    $rule = Get-AntiPhishRule -Identity $IdentityValue -ErrorAction SilentlyContinue

    @{
        Identity                            = $policy.Identity
        Name                                = $policy.Name
        PhishThresholdLevel                 = Get-PropSafe $policy 'PhishThresholdLevel' 1
        EnableSpoofIntelligence             = Get-PropSafe $policy 'EnableSpoofIntelligence' $true
        AuthenticationFailAction            = [string](Get-PropSafe $policy 'AuthenticationFailAction' 'MoveToJmf')
        EnableMailboxIntelligence           = Get-PropSafe $policy 'EnableMailboxIntelligence' $true
        EnableMailboxIntelligenceProtection = Get-PropSafe $policy 'EnableMailboxIntelligenceProtection' $false
        MailboxIntelligenceProtectionAction = [string](Get-PropSafe $policy 'MailboxIntelligenceProtectionAction' 'NoAction')
        EnableOrganizationDomainsProtection = Get-PropSafe $policy 'EnableOrganizationDomainsProtection' $false
        EnableUnauthenticatedSender         = Get-PropSafe $policy 'EnableUnauthenticatedSender' $true
        EnableViaTag                        = Get-PropSafe $policy 'EnableViaTag' $true
        EnableFirstContactSafetyTips        = Get-PropSafe $policy 'EnableFirstContactSafetyTips' $false
        Priority                            = if ($rule) { $rule.Priority } else { 0 }
        Enabled                             = if ($rule) { $rule.State -eq "Enabled" } else { $true }
        RecipientDomainIs                   = if ($rule -and $rule.RecipientDomainIs)         { [string[]]@($rule.RecipientDomainIs         | ForEach-Object { $_.ToString() }) } else { @() }
        SentToMemberOf                      = if ($rule -and $rule.SentToMemberOf)            { [string[]]@($rule.SentToMemberOf            | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfRecipientDomainIs           = if ($rule -and $rule.ExceptIfRecipientDomainIs) { [string[]]@($rule.ExceptIfRecipientDomainIs | ForEach-Object { $_.ToString() }) } else { @() }
    } | ConvertTo-Json -Compress
}

# Rule conditions. The rule exists only while an inclusion condition is set
# (see Sync-BridgeRule in common.ps1).
$RuleConditions = ConvertTo-BridgeConditions $PSBoundParameters -Csv RecipientDomainIs, SentToMemberOf, ExceptIfRecipientDomainIs

# Assert-DefenderSettings fails when a Defender for Office 365 setting was not
# applied: without the license, Exchange accepts these parameters but ignores
# them, which would leave Terraform with a value that never sticks.
function Assert-DefenderSettings {
    param([string]$IdentityValue)
    $policy = Get-AntiPhishPolicy -Identity $IdentityValue
    $defender = [ordered]@{
        PhishThresholdLevel                 = 'phish_threshold_level'
        EnableMailboxIntelligenceProtection = 'enable_mailbox_intelligence_protection'
        MailboxIntelligenceProtectionAction = 'mailbox_intelligence_protection_action'
        EnableOrganizationDomainsProtection = 'enable_organization_domains_protection'
    }
    $ignored = @(foreach ($prop in $defender.Keys) {
        if (-not $policySplat.ContainsKey($prop)) { continue }
        $actual = Get-PropSafe $policy $prop
        if ($null -ne $actual -and "$actual" -ne "$($policySplat[$prop])") { $defender[$prop] }
    })
    if ($ignored.Count -gt 0) {
        throw "Exchange did not apply $($ignored -join ', '): these settings need a Microsoft Defender for Office 365 license. Remove them from the configuration."
    }
}

switch ($Action) {

    "Get" {
        try {
            Get-PolicyWithRule -IdentityValue $Identity
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") {
                exit 0  # policy deleted externally — Terraform removes it from state
            }
            [Console]::Error.WriteLine("ERROR in anti_phish_policy Get: $msg")
            exit 1
        }
    }

    "Create" {
        # 1. Create the policy (settings)
        New-AntiPhishPolicy -Name $Name @policySplat | Out-Null

        # 2. Create the rule that scopes the policy, when a condition is set.
        try {
            Sync-BridgeRule -Noun "AntiPhishRule" -PolicyParam "AntiPhishPolicy" -Name $Name -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled
        } catch {
            # Roll back the policy so no orphan object is left behind.
            $ruleError = $_.Exception.Message
            Remove-AntiPhishPolicy -Identity $Name -ErrorAction SilentlyContinue
            throw "Failed to create '$Name' (nothing was created): $ruleError"
        }

        try { Assert-DefenderSettings -IdentityValue $Name }
        catch {
            $licenseError = $_.Exception.Message
            Remove-BridgeRule -Noun "AntiPhishRule" -Name $Name
            Remove-AntiPhishPolicy -Identity $Name -ErrorAction SilentlyContinue
            throw "Failed to create '$Name' (nothing was created): $licenseError"
        }

        Get-PolicyWithRule -IdentityValue $Name
    }

    "Update" {
        # 1. Update policy settings
        if ($policySplat.Count -gt 0) { Set-AntiPhishPolicy -Identity $Identity @policySplat }

        # 2. Create, update or remove the rule to match the conditions.
        Sync-BridgeRule -Noun "AntiPhishRule" -PolicyParam "AntiPhishPolicy" -Name $Identity -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled
        Assert-DefenderSettings -IdentityValue $Identity

        Get-PolicyWithRule -IdentityValue $Identity
    }

    "Delete" {
        # Rule must be deleted before the policy
        Remove-BridgeRule -Noun "AntiPhishRule" -Name $Identity
        Remove-AntiPhishPolicy -Identity $Identity
        Write-Output "{}"
    }
}
