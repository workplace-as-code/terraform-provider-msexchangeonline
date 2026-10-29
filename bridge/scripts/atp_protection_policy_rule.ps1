# atp_protection_policy_rule.ps1 — Get/Set/Enable/Disable-ATPProtectionPolicyRule: the rule of a
# preset security policy, adopted and never created or removed. The logic is
# shared with the other preset rule script: Invoke-BridgePresetRule (common.ps1).

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("ATPProtectionPolicyRule")] [string]$Noun = "ATPProtectionPolicyRule",
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

try {
    Invoke-BridgePresetRule -Noun "ATPProtectionPolicyRule" -Action $Action -Identity $Identity -Values $Values -Properties $Properties
} catch {
    [Console]::Error.WriteLine("ATPProtectionPolicyRule $Action failed: $($_.Exception.Message)")
    exit 1
}
