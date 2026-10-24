# safe_links_rule.ps1 — CRUD for Safe Links rules (standalone rule management)
# Use this when the policy is managed separately via msexchangeonline_safe_links_policy.
# Cmdlet noun: SafeLinksRule

param(
    [string]$TenantId = "", [string]$ClientId = "",
    [string]$CertificateThumbprint = "", [string]$CertificatePath = "",
    [string]$CertificatePassword = "",
    [string]$Environment = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity = "", [string]$Name = "",
    [string]$SafeLinksPolicy = "",
    [string]$Priority = "", [string]$Enabled,
    [string]$RecipientDomainIs, [string]$SentToMemberOf,
    [string]$ExceptIfRecipientDomainIs
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId `
    -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword `
    -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"


function ConvertTo-RuleJson { param($rule)
    @{
        Identity                  = [string]$rule.Identity
        Name                      = [string]$rule.Name
        SafeLinksPolicy           = [string]$rule.SafeLinksPolicy
        Priority                  = [int]$rule.Priority
        Enabled                   = ($rule.State -eq "Enabled")
        RecipientDomainIs         = if ($rule.RecipientDomainIs)         { [string[]]@($rule.RecipientDomainIs         | ForEach-Object { $_.ToString() }) } else { @() }
        SentToMemberOf            = if ($rule.SentToMemberOf)            { [string[]]@($rule.SentToMemberOf            | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfRecipientDomainIs = if ($rule.ExceptIfRecipientDomainIs) { [string[]]@($rule.ExceptIfRecipientDomainIs | ForEach-Object { $_.ToString() }) } else { @() }
    } | ConvertTo-Json -Compress
}

# Only the conditions set in the Terraform configuration are sent; an empty
# list clears the condition.
$conditions = ConvertTo-BridgeSplat $PSBoundParameters -Csv RecipientDomainIs, SentToMemberOf, ExceptIfRecipientDomainIs

switch ($Action) {
    "Get" {
        try { ConvertTo-RuleJson (Get-SafeLinksRule -Identity $Identity -ErrorAction Stop) }
        catch { $m = $_.Exception.Message; if ($m -match "couldn't be found|not found|ObjectNotFound") { exit 0 }; Write-Error $m; exit 1 }
    }
    "Create" {
        $params = @{ Name = $Name; SafeLinksPolicy = $SafeLinksPolicy }
        foreach ($k in $conditions.Keys) { if ($null -ne $conditions[$k]) { $params[$k] = $conditions[$k] } }
        New-SafeLinksRule @params | Out-Null
        # Priority is set afterwards: New-SafeLinksRule rejects a value outside
        # 0..(rule count - 1). An invalid value removes the new rule.
        if ($Priority -ne "") {
            try { Set-SafeLinksRule -Identity $Name -Priority ([int]$Priority) -ErrorAction Stop }
            catch {
                $priorityError = $_.Exception.Message
                Remove-SafeLinksRule -Identity $Name -ErrorAction SilentlyContinue
                throw "Invalid priority $Priority for '$Name' (nothing was created): $priorityError"
            }
        }
        if ($Enabled -eq "false") { Disable-SafeLinksRule -Identity $Name }
        ConvertTo-RuleJson (Get-SafeLinksRule -Identity $Name -ErrorAction Stop)
    }

    "Update" {
        $params = @{} + $conditions
        if ($Priority -ne "") { $params["Priority"] = [int]$Priority }
        if ($params.Count -gt 0) { Set-SafeLinksRule -Identity $Identity @params }
        if ($PSBoundParameters.ContainsKey("Enabled")) {
            if ($Enabled -eq "true") { Enable-SafeLinksRule -Identity $Identity }
            else                     { Disable-SafeLinksRule -Identity $Identity }
        }
        ConvertTo-RuleJson (Get-SafeLinksRule -Identity $Identity -ErrorAction Stop)
    }

    "Delete" { Remove-SafeLinksRule -Identity $Identity; Write-Output "{}" }
}
