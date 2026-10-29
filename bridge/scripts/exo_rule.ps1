# exo_rule.ps1 — Generic CRUD for the rules that scope a protection policy to
# senders or recipients (HostedOutboundSpamFilterRule, ...). Same contract as
# exo_object.ps1, with the rule specifics:
#   - "Enabled" is set with New-<noun> -Enabled, then Enable-<noun> /
#     Disable-<noun>, and read from the State property
#   - "Priority" is set after the rule exists: New-<noun> rejects a value
#     outside 0..(rule count - 1)
#   - an empty list clears the condition

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidatePattern('^[A-Za-z]+Rule$')] [string]$Noun,
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

# Enabled and Priority are not plain Set-<noun> parameters.
$enabled = $null
if ($splat.ContainsKey("Enabled")) { $enabled = [bool]$splat["Enabled"]; $splat.Remove("Enabled") }
$priority = $null
if ($splat.ContainsKey("Priority")) { $priority = [int]$splat["Priority"]; $splat.Remove("Priority") }

function Read-BridgeRule {
    $rule = Invoke-BridgeRetry { & "Get-$Noun" -Identity $Identity }
    $out = [ordered]@{ Name = [string]$rule.Name }
    foreach ($p in $props) {
        $out[$p] = if ($p -eq "Enabled") { [string](Get-BridgeProp $rule "State") -eq "Enabled" }
                   else { ConvertTo-BridgeValue (Get-BridgeProp $rule $p) }
    }
    # A condition that was never set comes back as null: report an empty list.
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

function Set-BridgeRuleState {
    if ($null -ne $priority) { & "Set-$Noun" -Identity $Identity -Priority $priority }
    if ($enabled -eq $true)  { & "Enable-$Noun" -Identity $Identity }
    if ($enabled -eq $false) { & "Disable-$Noun" -Identity $Identity }
}

try {
    switch ($Action) {
        "Get" {
            try {
                Read-BridgeRule
            } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            # New-<noun> rejects $null conditions: only send the ones with values.
            $new = @{}
            foreach ($k in $splat.Keys) { if ($null -ne $splat[$k]) { $new[$k] = $splat[$k] } }
            if ($null -ne $enabled) { $new["Enabled"] = $enabled }
            & "New-$Noun" -Name $Identity @new | Out-Null
            try {
                if ($null -ne $priority) { & "Set-$Noun" -Identity $Identity -Priority $priority }
            } catch {
                $priorityError = $_.Exception.Message
                & "Remove-$Noun" -Identity $Identity -ErrorAction SilentlyContinue
                throw "Invalid priority $priority for rule '$Identity' (nothing was created): $priorityError"
            }
            Read-BridgeRule
        }

        "Update" {
            # Set-<noun> rejects the policy parameter when it names the policy
            # the rule already applies ("already has rule"): only send a change.
            $current = Invoke-BridgeRetry { & "Get-$Noun" -Identity $Identity }
            foreach ($k in @($splat.Keys)) {
                if ($k -like "*Policy" -and [string](Get-BridgeProp $current $k) -eq [string]$splat[$k]) { $splat.Remove($k) }
            }
            if ($splat.Count -gt 0) { & "Set-$Noun" -Identity $Identity @splat }
            Set-BridgeRuleState
            Read-BridgeRule
        }

        "Delete" {
            try {
                & "Remove-$Noun" -Identity $Identity
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
