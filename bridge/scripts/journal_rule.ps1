# journal_rule.ps1 — CRUD for journal rules (New/Get/Set/Remove-JournalRule),
# called by the generic resource code (object.go) with the same parameters as
# exo_object.ps1. Journal rule specifics:
#   - "Enabled" is set with New-JournalRule -Enabled, then Enable-JournalRule /
#     Disable-JournalRule, and read from the Enabled property
#   - an empty Recipient clears it: every recipient is journaled

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("JournalRule")] [string]$Noun = "JournalRule",
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$splat = ConvertFrom-BridgeValues $Values
$props = @(ConvertFrom-BridgeJson $Properties)

# New-JournalRule and Set-JournalRule reject "" for an address: $null clears it.
foreach ($k in @($splat.Keys)) { if ($splat[$k] -is [string] -and $splat[$k] -eq "") { $splat[$k] = $null } }
$enabled = $null
if ($splat.ContainsKey("Enabled")) { $enabled = [bool]$splat["Enabled"]; $splat.Remove("Enabled") }

function Read-BridgeJournalRule {
    $rule = Invoke-BridgeRetry { Get-JournalRule -Identity $Identity }
    $out = [ordered]@{ Name = [string]$rule.Name }
    foreach ($p in $props) { $out[$p] = ConvertTo-BridgeValue (Get-BridgeProp $rule $p) }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try {
                Read-BridgeJournalRule
            } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $new = @{}
            foreach ($k in $splat.Keys) { if ($null -ne $splat[$k]) { $new[$k] = $splat[$k] } }
            if ($null -ne $enabled) { $new["Enabled"] = $enabled }
            New-JournalRule -Name $Identity @new | Out-Null
            Read-BridgeJournalRule
        }

        "Update" {
            if ($splat.Count -gt 0) { Set-JournalRule -Identity $Identity @splat }
            if ($enabled -eq $true)  { Enable-JournalRule -Identity $Identity }
            if ($enabled -eq $false) { Disable-JournalRule -Identity $Identity }
            Read-BridgeJournalRule
        }

        "Delete" {
            try {
                Remove-JournalRule -Identity $Identity
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("JournalRule $Action failed: $($_.Exception.Message)")
    exit 1
}
