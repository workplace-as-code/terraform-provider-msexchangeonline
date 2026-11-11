# quarantine_global_settings.ps1 — Reads and writes the global settings of
# quarantine notifications. A tenant never configured only has a virtual object
# (zero Guid) that Set-QuarantinePolicy cannot find: create it first. The object
# is always read by type: -Identity DefaultGlobalTag does not find it, even once
# it exists.
#   -Values     base64 JSON object: cmdlet parameter -> value
#   -Languages  base64 JSON array of {Language, SenderName, Subject, Disclaimer};
#               absent when the configuration does not manage the languages

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Update")] [string]$Action,

    [string]$Values = "",
    [string]$Languages
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

function Get-BridgeGlobalPolicy {
    Invoke-BridgeRetry { @(Get-QuarantinePolicy -QuarantinePolicyType GlobalQuarantinePolicy) | Select-Object -First 1 }
}

function Get-BridgeListItem {
    param($List, [int]$Index)
    $items = @($List)
    if ($Index -lt $items.Count -and $null -ne $items[$Index]) { [string]$items[$Index] } else { "" }
}

function Write-BridgeGlobalPolicy {
    param($Policy)
    # Piping $null runs ForEach-Object once: a policy without languages would
    # read back as one language with an empty name.
    $names = @(if ($null -ne $Policy.MultiLanguageSetting) { $Policy.MultiLanguageSetting | ForEach-Object { [string]$_ } })
    $languages = [object[]]@(for ($i = 0; $i -lt $names.Count; $i++) {
        [ordered]@{
            Language   = $names[$i]
            SenderName = Get-BridgeListItem $Policy.MultiLanguageSenderName $i
            Subject    = Get-BridgeListItem $Policy.EsnCustomSubject $i
            Disclaimer = Get-BridgeListItem $Policy.MultiLanguageCustomDisclaimer $i
        }
    })
    ConvertTo-Json -Compress -Depth 4 -InputObject ([ordered]@{
        EndUserSpamNotificationFrequency         = [string]$Policy.EndUserSpamNotificationFrequency
        EndUserSpamNotificationCustomFromAddress = [string]$Policy.EndUserSpamNotificationCustomFromAddress
        OrganizationBrandingEnabled              = [bool]$Policy.OrganizationBrandingEnabled
        Languages                                = $languages
    })
}

try {
    switch ($Action) {
        "Get" { Write-BridgeGlobalPolicy (Get-BridgeGlobalPolicy) }

        "Update" {
            $splat = ConvertFrom-BridgeValues $Values
            if ($splat.ContainsKey("EndUserSpamNotificationCustomFromAddress") -and -not $splat["EndUserSpamNotificationCustomFromAddress"]) {
                # $null puts back the Microsoft sender.
                $splat["EndUserSpamNotificationCustomFromAddress"] = $null
            }
            if ($PSBoundParameters.ContainsKey("Languages")) {
                # Every list is written whole, in the same order: positions match.
                $wanted = @(ConvertFrom-BridgeJson $Languages | Where-Object { $null -ne $_ } | Sort-Object Language)
                $splat["MultiLanguageSetting"]          = [string[]]@($wanted | ForEach-Object { $_.Language })
                $splat["MultiLanguageSenderName"]       = [string[]]@($wanted | ForEach-Object { $_.SenderName })
                $splat["EsnCustomSubject"]              = [string[]]@($wanted | ForEach-Object { $_.Subject })
                $splat["MultiLanguageCustomDisclaimer"] = [string[]]@($wanted | ForEach-Object { $_.Disclaimer })
            }
            $policy = Get-BridgeGlobalPolicy
            if ([string](Get-BridgeProp $policy "Guid" "") -in "", "00000000-0000-0000-0000-000000000000") {
                Invoke-BridgeRetry -Create { New-QuarantinePolicy -Name DefaultGlobalTag -QuarantinePolicyType GlobalQuarantinePolicy | Out-Null }
                $policy = Get-BridgeGlobalPolicy
            }
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { $policy | Set-QuarantinePolicy @splat | Out-Null } }
            Write-BridgeGlobalPolicy (Get-BridgeGlobalPolicy)
        }
    }
} catch {
    [Console]::Error.WriteLine("quarantine_global_settings $Action failed: $($_.Exception.Message)")
    exit 1
}
