# anti_phish_rule.ps1 — CRUD for standalone Exchange Online anti-phishing rules
# Use this resource when you need to link an existing anti-phish policy to a
# specific set of recipients independently of the policy settings.
# Cmdlet noun: AntiPhishRule
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

    [string]$Identity                  = "",
    [string]$Name                      = "",
    [string]$AntiPhishPolicy           = "",
    [string]$Priority                  = "",
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



function ConvertTo-BridgeRule {
    param($Rule)
    @{
        Identity                  = $Rule.Identity
        Name                      = $Rule.Name
        AntiPhishPolicy           = [string]$Rule.AntiPhishPolicy
        Priority                  = [int]$Rule.Priority
        Enabled                   = ($Rule.State -eq "Enabled")
        RecipientDomainIs         = if ($Rule.RecipientDomainIs)         { [string[]]@($Rule.RecipientDomainIs         | ForEach-Object { $_.ToString() }) } else { @() }
        SentTo                    = if ($Rule.SentTo)                    { [string[]]@($Rule.SentTo                    | ForEach-Object { $_.ToString() }) } else { @() }
        SentToMemberOf            = if ($Rule.SentToMemberOf)            { [string[]]@($Rule.SentToMemberOf            | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfRecipientDomainIs = if ($Rule.ExceptIfRecipientDomainIs) { [string[]]@($Rule.ExceptIfRecipientDomainIs | ForEach-Object { $_.ToString() }) } else { @() }
        ExceptIfSentTo            = if ($Rule.ExceptIfSentTo)            { [string[]]@($Rule.ExceptIfSentTo            | ForEach-Object { $_.ToString() }) } else { @() }
    } | ConvertTo-Json -Compress
}

# Only the conditions set in the Terraform configuration are sent; an empty
# list clears the condition.
$conditions = ConvertTo-BridgeSplat $PSBoundParameters -Csv RecipientDomainIs, SentTo, SentToMemberOf, ExceptIfRecipientDomainIs, ExceptIfSentTo

switch ($Action) {

    "Get" {
        try {
            $rule = Get-AntiPhishRule -Identity $Identity -ErrorAction Stop
            ConvertTo-BridgeRule -Rule $rule
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") {
                exit 0
            }
            [Console]::Error.WriteLine("ERROR in anti_phish_rule Get: $msg")
            exit 1
        }
    }

    "Create" {
        $params = @{ Name = $Name; AntiPhishPolicy = $AntiPhishPolicy }
        foreach ($k in $conditions.Keys) { if ($null -ne $conditions[$k]) { $params[$k] = $conditions[$k] } }
        New-AntiPhishRule @params | Out-Null
        # Priority is set afterwards: New-AntiPhishRule rejects a value outside
        # 0..(rule count - 1). An invalid value removes the new rule.
        if ($Priority -ne "") {
            try { Set-AntiPhishRule -Identity $Name -Priority ([int]$Priority) -ErrorAction Stop }
            catch {
                $priorityError = $_.Exception.Message
                Remove-AntiPhishRule -Identity $Name -ErrorAction SilentlyContinue
                throw "Invalid priority $Priority for '$Name' (nothing was created): $priorityError"
            }
        }
        if ($Enabled -eq "false") { Disable-AntiPhishRule -Identity $Name }
        $rule = Get-AntiPhishRule -Identity $Name -ErrorAction Stop
        ConvertTo-BridgeRule -Rule $rule
    }

    "Update" {
        $params = @{} + $conditions
        if ($Priority -ne "") { $params["Priority"] = [int]$Priority }
        if ($params.Count -gt 0) { Set-AntiPhishRule -Identity $Identity @params }
        if ($PSBoundParameters.ContainsKey("Enabled")) {
            if ($Enabled -eq "true") { Enable-AntiPhishRule -Identity $Identity }
            else                     { Disable-AntiPhishRule -Identity $Identity }
        }
        $rule = Get-AntiPhishRule -Identity $Identity -ErrorAction Stop
        ConvertTo-BridgeRule -Rule $rule
    }

    "Delete" {
        Remove-AntiPhishRule -Identity $Identity
        Write-Output "{}"
    }
}
