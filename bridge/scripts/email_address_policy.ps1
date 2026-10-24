# email_address_policy.ps1 — CRUD for Exchange email address policies
# Cmdlet noun: EmailAddressPolicy
# In Exchange Online, email address policies only apply to Microsoft 365 Groups
# (-IncludeUnifiedGroupRecipients).

param(
    [string]$TenantId = "", [string]$ClientId = "",
    [string]$CertificateThumbprint = "", [string]$CertificatePath = "",
    [string]$CertificatePassword = "",
    [string]$Environment = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity     = "",
    [string]$Name         = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$EnabledEmailAddressTemplates,
    [string]$Priority
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId `
    -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword `
    -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$settings = ConvertTo-BridgeSplat $PSBoundParameters -Int Priority -Csv EnabledEmailAddressTemplates
function ConvertTo-PolicyJson { param($pol)
    @{
        Identity                      = [string]$pol.Identity
        Name                          = [string]$pol.Name
        EnabledEmailAddressTemplates  = if ($pol.EnabledEmailAddressTemplates) { [string[]]@($pol.EnabledEmailAddressTemplates | ForEach-Object { $_.ToString() }) } else { @() }
        # The default policy always comes last: its priority is "Lowest".
        Priority                      = if ([string]$pol.Priority -eq "Lowest") { 0 } else { [int][string]$pol.Priority }
    } | ConvertTo-Json -Compress
}

function Get-PolicyParams {
    param([bool]$IsCreate = $false)
    $p = @{} + $settings
    # A new policy cannot be created with an empty list.
    if ($IsCreate -and $p.ContainsKey("EnabledEmailAddressTemplates") -and $null -eq $p["EnabledEmailAddressTemplates"]) {
        $p.Remove("EnabledEmailAddressTemplates")
    }
    # Only New-EmailAddressPolicy has this switch.
    if ($IsCreate) { $p["IncludeUnifiedGroupRecipients"] = $true }
    $p
}

switch ($Action) {
    "Get" {
        try { ConvertTo-PolicyJson (Get-EmailAddressPolicy -Identity $Identity -ErrorAction Stop) }
        catch { $m = $_.Exception.Message; if ($m -match "couldn't be found|not found|ObjectNotFound") { exit 0 }; Write-Error $m; exit 1 }
    }
    "Create" {
        $p = Get-PolicyParams -IsCreate $true
        $p["Name"] = $Name
        New-EmailAddressPolicy @p | Out-Null
        ConvertTo-PolicyJson (Get-EmailAddressPolicy -Identity $Name -ErrorAction Stop)
    }
    "Update" {
        $p = Get-PolicyParams
        if ($p.Count -gt 0) { Set-EmailAddressPolicy -Identity $Identity @p | Out-Null }
        ConvertTo-PolicyJson (Get-EmailAddressPolicy -Identity $Identity -ErrorAction Stop)
    }
    "Delete" { Remove-EmailAddressPolicy -Identity $Identity; Write-Output "{}" }
}
