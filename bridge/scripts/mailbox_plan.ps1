# mailbox_plan.ps1 — Get/Set-MailboxPlan and Get/Set-CASMailboxPlan. The plans
# come with the licenses: they are adopted, never created or removed.
#
# Same parameters as exo_object.ps1. -Identity is the display name of the plan
# (ExchangeOnlineEnterprise), the same in every tenant, and is returned as its
# name: the real name ends with a GUID that differs between tenants. Sizes are
# exchanged in megabytes.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("MailboxPlan", "CASMailboxPlan")] [string]$Noun,
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$SizeParams = @("IssueWarningQuota", "MaxReceiveSize", "MaxSendSize", "ProhibitSendQuota", "ProhibitSendReceiveQuota")

$splat = ConvertFrom-BridgeValues $Values
$props = @(ConvertFrom-BridgeJson $Properties)
foreach ($p in @($splat.Keys)) {
    if ($SizeParams -contains $p -and $null -ne $splat[$p]) { $splat[$p] = "$($splat[$p]) MB" }
}

# ConvertTo-Megabytes reads "35 MB (36,700,160 bytes)" as 35. Any other form,
# such as Unlimited, is returned as 0.
function ConvertTo-Megabytes {
    param($Value)
    if ([string]$Value -match '\(([\d,]+) bytes\)') {
        return [long]([decimal]($Matches[1] -replace ',', '') / 1MB)
    }
    0
}

function Read-BridgePlan {
    $plan = Invoke-BridgeRetry { & "Get-$Noun" -Identity $Identity }
    $out = [ordered]@{ Name = [string](Get-BridgeProp $plan "DisplayName" $Identity) }
    foreach ($p in $props) {
        $value = Get-BridgeProp $plan $p
        $out[$p] = if ($SizeParams -contains $p) { ConvertTo-Megabytes $value } else { ConvertTo-BridgeValue $value }
    }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgePlan } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }
        { $_ -in "Create", "Update" } {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { & "Set-$Noun" -Identity $Identity @splat | Out-Null } }
            Read-BridgePlan
        }
        "Delete" { "{}" }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
