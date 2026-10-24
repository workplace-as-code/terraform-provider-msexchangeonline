# hosted_content_filter_policy.ps1 — CRUD for Exchange Online spam filter policies + rules
# Manages HostedContentFilterPolicy (settings) and HostedContentFilterRule (conditions) as a single unit.
# Cmdlet nouns: HostedContentFilterPolicy, HostedContentFilterRule
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

    [string]$Identity                          = "",
    [string]$Name                              = "",
    # Policy settings
    [string]$SpamAction,
    [string]$HighConfidenceSpamAction,
    [string]$PhishSpamAction,
    [string]$HighConfidencePhishAction,
    [string]$BulkSpamAction,
    [string]$BulkThreshold,
    [string]$QuarantineTag,
    [string]$HighConfidenceSpamQuarantineTag,
    [string]$PhishQuarantineTag,
    [string]$HighConfidencePhishQuarantineTag,
    [string]$BulkQuarantineTag,
    [string]$MarkAsSpamBulkMail,
    [string]$MarkAsSpamEmptyMessages,
    [string]$IncreaseScoreWithImageLinks,
    # Rule conditions
    [string]$Priority = "",
    [string]$Enabled,
    [string]$RecipientDomainIs,
    [string]$SentTo,
    [string]$SentToMemberOf,
    [string]$ExceptIfRecipientDomainIs,
    [string]$ExceptIfSentTo
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"



function Get-PropSafe {
    param($Obj, [string]$Prop, $Default = $null)
    $p = $Obj.PSObject.Properties[$Prop]
    if ($null -ne $p) { return $p.Value }
    return $Default
}


function Get-PolicyWithRule {
    param([string]$IdentityValue)
    $policy = Get-HostedContentFilterPolicy -Identity $IdentityValue -ErrorAction Stop
    $rule   = Get-HostedContentFilterRule   -Identity $IdentityValue -ErrorAction SilentlyContinue

    @{
        Identity                         = $policy.Identity
        Name                             = $policy.Name
        SpamAction                       = [string](Get-PropSafe $policy 'SpamAction'                'MoveToJmf')
        HighConfidenceSpamAction         = [string](Get-PropSafe $policy 'HighConfidenceSpamAction'  'MoveToJmf')
        PhishSpamAction                  = [string](Get-PropSafe $policy 'PhishSpamAction'           'MoveToJmf')
        HighConfidencePhishAction        = [string](Get-PropSafe $policy 'HighConfidencePhishAction' 'Quarantine')
        BulkSpamAction                   = [string](Get-PropSafe $policy 'BulkSpamAction'            'MoveToJmf')
        BulkThreshold                    = [int](Get-PropSafe    $policy 'BulkThreshold'             7)
        QuarantineTag                    = [string](Get-PropSafe $policy 'QuarantineTag'             '')
        HighConfidenceSpamQuarantineTag  = [string](Get-PropSafe $policy 'HighConfidenceSpamQuarantineTag'  '')
        PhishQuarantineTag               = [string](Get-PropSafe $policy 'PhishQuarantineTag'               '')
        HighConfidencePhishQuarantineTag = [string](Get-PropSafe $policy 'HighConfidencePhishQuarantineTag' '')
        BulkQuarantineTag                = [string](Get-PropSafe $policy 'BulkQuarantineTag'                '')
        MarkAsSpamBulkMail               = [string](Get-PropSafe $policy 'MarkAsSpamBulkMail'        'On')
        MarkAsSpamEmptyMessages          = [string](Get-PropSafe $policy 'MarkAsSpamEmptyMessages'   'Off')
        IncreaseScoreWithImageLinks      = [string](Get-PropSafe $policy 'IncreaseScoreWithImageLinks' 'Off')
        Priority                         = if ($rule) { [int]$rule.Priority } else { 0 }
        Enabled                          = if ($rule) { $rule.State -eq "Enabled" } else { $true }
        RecipientDomainIs                = if ($rule -and $rule.RecipientDomainIs)         { [string[]]@($rule.RecipientDomainIs         | ForEach-Object { $_.ToString() }) } else { @() }
        SentTo                           = if ($rule -and $rule.SentTo)                    { [string[]]@($rule.SentTo                    | ForEach-Object { $_.ToString() }) } else { @() }
        SentToMemberOf                   = if ($rule -and $rule.SentToMemberOf)            { [string[]]@($rule.SentToMemberOf            | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfRecipientDomainIs        = if ($rule -and $rule.ExceptIfRecipientDomainIs) { [string[]]@($rule.ExceptIfRecipientDomainIs | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfSentTo                   = if ($rule -and $rule.ExceptIfSentTo)            { [string[]]@($rule.ExceptIfSentTo            | ForEach-Object { $_.ToString() }) } else { @() }
    } | ConvertTo-Json -Compress
}

# Only the settings set in the Terraform configuration are sent.
$policySplat = ConvertTo-BridgeSplat $PSBoundParameters -Int BulkThreshold -String SpamAction, HighConfidenceSpamAction, PhishSpamAction, HighConfidencePhishAction, BulkSpamAction, QuarantineTag, HighConfidenceSpamQuarantineTag, PhishQuarantineTag, HighConfidencePhishQuarantineTag, BulkQuarantineTag, MarkAsSpamBulkMail, MarkAsSpamEmptyMessages, IncreaseScoreWithImageLinks
$ruleEnabled = if ($PSBoundParameters.ContainsKey("Enabled")) { $Enabled -eq "true" } else { $null }

# Rule conditions. The rule exists only while an inclusion condition is set
# (see Sync-BridgeRule in common.ps1).
$RuleConditions = ConvertTo-BridgeConditions $PSBoundParameters -Csv RecipientDomainIs, SentTo, SentToMemberOf, ExceptIfRecipientDomainIs, ExceptIfSentTo

switch ($Action) {

    "Get" {
        try {
            Get-PolicyWithRule -IdentityValue $Identity
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") {
                exit 0
            }
            [Console]::Error.WriteLine("ERROR in hosted_content_filter_policy Get: $msg")
            exit 1
        }
    }

    "Create" {
        # 1. Create the policy
        New-HostedContentFilterPolicy -Name $Name @policySplat | Out-Null

        # 2. Create the rule that scopes the policy, when a condition is set.
        try {
            Sync-BridgeRule -Noun "HostedContentFilterRule" -PolicyParam "HostedContentFilterPolicy" -Name $Name -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled
        } catch {
            # Roll back the policy so no orphan object is left behind.
            $ruleError = $_.Exception.Message
            Remove-HostedContentFilterPolicy -Identity $Name -ErrorAction SilentlyContinue
            throw "Failed to create '$Name' (nothing was created): $ruleError"
        }


        Get-PolicyWithRule -IdentityValue $Name
    }

    "Update" {
        # 1. Update policy settings
        if ($policySplat.Count -gt 0) { Set-HostedContentFilterPolicy -Identity $Identity @policySplat }

        # 2. Create, update or remove the rule to match the conditions.
        Sync-BridgeRule -Noun "HostedContentFilterRule" -PolicyParam "HostedContentFilterPolicy" -Name $Identity -Conditions $RuleConditions -Priority $Priority -Enabled $ruleEnabled


        Get-PolicyWithRule -IdentityValue $Identity
    }

    "Delete" {
        # Rule must be deleted before the policy
        Remove-BridgeRule -Noun "HostedContentFilterRule" -Name $Identity
        Remove-HostedContentFilterPolicy -Identity $Identity
        Write-Output "{}"
    }
}
