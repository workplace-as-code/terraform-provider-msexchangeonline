# quarantine_policy.ps1 — CRUD for Exchange Online quarantine policies
# Cmdlet noun: QuarantinePolicy
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

    [string]$Identity                             = "",
    [string]$Name                                 = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$EndUserQuarantinePermissionsValue,
    [string]$ESNEnabled,
    [string]$QuarantineRetentionDays
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId `
    -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword `
    -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$settings = ConvertTo-BridgeSplat $PSBoundParameters -Int EndUserQuarantinePermissionsValue, QuarantineRetentionDays -Bool ESNEnabled



# Get-QuarantinePolicy does not return EndUserQuarantinePermissionsValue: it
# only returns EndUserQuarantinePermissions, a text such as
# "[PermissionToBlockSender: False ... PermissionToDelete: True]". Rebuild the
# number from it, with the bit order documented for New-QuarantinePolicy
# (PermissionToViewHeader is the most significant bit).
$PermissionBits = [ordered]@{
    PermissionToViewHeader     = 128
    PermissionToDownload       = 64
    PermissionToAllowSender    = 32
    PermissionToBlockSender    = 16
    PermissionToRequestRelease = 8
    PermissionToRelease        = 4
    PermissionToPreview        = 2
    PermissionToDelete         = 1
}
function ConvertTo-PermissionsValue {
    param($Policy)
    $text = [string]$Policy.EndUserQuarantinePermissions
    $value = 0
    foreach ($name in $PermissionBits.Keys) {
        if ($text -match "$($name):\s*True") { $value += $PermissionBits[$name] }
    }
    $value
}

function ConvertTo-QuarantineJson {
    param($pol)
    @{
        # Identity is "<tenant>\<name>", which Get-QuarantinePolicy rejects for
        # the built-in policies: the name identifies every policy.
        Identity                          = [string]$pol.Name
        Name                              = [string]$pol.Name
        EndUserQuarantinePermissionsValue = ConvertTo-PermissionsValue $pol
        ESNEnabled                        = [bool]$pol.ESNEnabled
        QuarantineRetentionDays          = if ($pol.QuarantineRetentionDays) { [int]$pol.QuarantineRetentionDays } else { 15 }
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "Get" {
        try {
            $pol = Get-QuarantinePolicy -Identity $Identity -ErrorAction Stop
            ConvertTo-QuarantineJson -pol $pol
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") { exit 0 }
            Write-Error "Failed to get quarantine policy: $msg"; exit 1
        }
    }

    "Create" {
        New-QuarantinePolicy -Name $Name @settings | Out-Null
        $pol = Get-QuarantinePolicy -Identity $Name -ErrorAction Stop
        ConvertTo-QuarantineJson -pol $pol
    }

    "Update" {
        if ($settings.Count -gt 0) { Set-QuarantinePolicy -Identity $Identity @settings | Out-Null }
        $pol = Get-QuarantinePolicy -Identity $Identity -ErrorAction Stop
        ConvertTo-QuarantineJson -pol $pol
    }

    "Delete" {
        Remove-QuarantinePolicy -Identity $Identity
        Write-Output "{}"
    }
}
