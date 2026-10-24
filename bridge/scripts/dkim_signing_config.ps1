# dkim_signing_config.ps1 — CRUD for Exchange Online DKIM signing configuration
# Manages per-domain DKIM signing (New/Set-DkimSigningConfig).
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

    [string]$Identity = "",
    [string]$DomainName = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$Enabled,
    [string]$KeySize,
    [string]$HeaderCanonicalization,
    [string]$BodyCanonicalization
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId `
    -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword `
    -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

# Only the settings set in the Terraform configuration are sent.
$settings = ConvertTo-BridgeSplat $PSBoundParameters -Bool Enabled -Int KeySize -String HeaderCanonicalization, BodyCanonicalization




# Get-DkimSigningConfig has no KeySize property (StrictMode throws on it), but
# one per selector: report the key size of the selector used after the last
# rotation, 0 when Exchange reports none.
function Get-DkimKeySize {
    param($cfg)
    $selector = [string](Get-BridgeProp $cfg "SelectorAfterRotateOnDate" "Selector1")
    if ($selector -notmatch '^Selector[12]$') { $selector = "Selector1" }
    $size = Get-BridgeProp $cfg "${selector}KeySize"
    if ($size) { [int]$size } else { 0 }
}

function ConvertTo-DkimJson {
    param($cfg)
    @{
        Identity               = [string]$cfg.Identity
        DomainName             = [string]$cfg.Domain
        Enabled                = [bool]$cfg.Enabled
        KeySize                = Get-DkimKeySize $cfg
        HeaderCanonicalization = [string]$cfg.HeaderCanonicalization
        BodyCanonicalization   = [string]$cfg.BodyCanonicalization
        Selector1CNAME         = [string](Get-BridgeProp $cfg "Selector1CNAME" "")
        Selector2CNAME         = [string](Get-BridgeProp $cfg "Selector2CNAME" "")
    } | ConvertTo-Json -Compress
}

# Set-DkimSigningConfig cannot change the key size: fail clearly rather than
# leave a different key size than the configuration asks for.
function Assert-DkimKeySize {
    param($cfg)
    $current = Get-DkimKeySize $cfg
    if ($settings.ContainsKey("KeySize") -and $current -and $current -ne [int]$settings["KeySize"]) {
        throw "The DKIM key of $($cfg.Domain) is $current bits, and Set-DkimSigningConfig cannot change it. Rotate the key with Rotate-DkimSigningConfig -KeySize $($settings['KeySize']), or remove key_size from the configuration."
    }
}

switch ($Action) {

    "Get" {
        try {
            $cfg = Get-DkimSigningConfig -Identity $Identity -ErrorAction Stop
            ConvertTo-DkimJson -cfg $cfg
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") {
                exit 0
            }
            Write-Error "Failed to get DKIM signing config: $msg"
            exit 1
        }
    }

    "Create" {
        $existing = $null
        try { $existing = Get-DkimSigningConfig -Identity $DomainName -ErrorAction Stop }
        catch { if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw } }
        if ($existing) {
            # A DKIM configuration cannot be deleted, so the domain may already
            # have one (created in the portal, or kept by an earlier destroy):
            # adopt it.
            Assert-DkimKeySize $existing
            $settings.Remove("KeySize")
            if ($settings.Count -gt 0) { Set-DkimSigningConfig -Identity $DomainName @settings | Out-Null }
        } else {
            if (-not $settings.ContainsKey("Enabled")) {
                throw "Set enabled to create a DKIM signing configuration: New-DkimSigningConfig requires it."
            }
            New-DkimSigningConfig -DomainName $DomainName @settings | Out-Null
        }
        $cfg = Get-DkimSigningConfig -Identity $DomainName -ErrorAction Stop
        ConvertTo-DkimJson -cfg $cfg
    }

    "Update" {
        Assert-DkimKeySize (Get-DkimSigningConfig -Identity $Identity -ErrorAction Stop)
        # The key size only applies when the configuration is created.
        $settings.Remove("KeySize")
        if ($settings.Count -gt 0) { Set-DkimSigningConfig -Identity $Identity @settings | Out-Null }
        $cfg = Get-DkimSigningConfig -Identity $Identity -ErrorAction Stop
        ConvertTo-DkimJson -cfg $cfg
    }

    "Delete" {
        # DKIM configs are domain-bound and cannot be deleted, only disabled
        Set-DkimSigningConfig -Identity $Identity -Enabled $false | Out-Null
        Write-Output "{}"
    }
}
