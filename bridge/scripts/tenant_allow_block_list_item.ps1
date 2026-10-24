# tenant_allow_block_list_item.ps1 — CRUD for one Tenant Allow/Block List entry
# (New/Get/Set/Remove-TenantAllowBlockListItems), identified by its list type
# and value. Exchange Online only.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("Sender","Url","FileHash","IP")] [string]$ListType,
    [Parameter(Mandatory)] [string]$Entry,
    [ValidateSet("", "Block", "Allow")] [string]$ListAction = "",
    [string]$Notes,
    [string]$ExpirationDate = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

function Get-Item_ {
    try {
        @(Get-TenantAllowBlockListItems -ListType $ListType -Entry $Entry -ErrorAction Stop) | Select-Object -First 1
    } catch {
        if (Test-BridgeNotFound $_.Exception.Message) { return $null }
        throw
    }
}

# ConvertTo-Rfc3339 formats a date in UTC, the format of the Terraform
# attribute. Exchange returns the UTC time without marking it as UTC, so the
# value is formatted as is: converting it would shift it by the local offset.
function ConvertTo-Rfc3339 {
    param($Value)
    if (-not $Value) { return "" }
    ([datetime]$Value).ToString("yyyy-MM-ddTHH:mm:ssZ", [cultureinfo]::InvariantCulture)
}

function ConvertTo-BridgeItem {
    param($Item)
    $expiration = Get-BridgeProp $Item "ExpirationDate"
    [ordered]@{
        Value          = [string](Get-BridgeProp $Item "Value" $Entry)
        Action         = [string](Get-BridgeProp $Item "Action" "")
        Notes          = [string](Get-BridgeProp $Item "Notes" "")
        ExpirationDate = ConvertTo-Rfc3339 $expiration
    } | ConvertTo-Json -Compress
}

# Get-ExpirationParams returns the expiration parameters: a date when one is
# configured, otherwise no expiration for a block entry and removal 45 days
# after the last use for an allow entry (Exchange does not accept
# NoExpiration on most allow entries).
function Get-ExpirationParams {
    param([string]$ItemAction)
    if ($ExpirationDate) { return @{ ExpirationDate = [DateTimeOffset]::Parse($ExpirationDate, [cultureinfo]::InvariantCulture).UtcDateTime } }
    if ($ItemAction -eq "Allow") { return @{ RemoveAfter = 45 } }
    @{ NoExpiration = $true }
}

try {
    switch ($Action) {
        "Get" {
            $item = Get-Item_
            if ($null -eq $item) { exit 0 }
            ConvertTo-BridgeItem $item
        }

        "Create" {
            $p = @{ ListType = $ListType; Entries = @($Entry) } + (Get-ExpirationParams $ListAction)
            $p[$ListAction] = $true
            if ($PSBoundParameters.ContainsKey("Notes")) { $p["Notes"] = $Notes }
            New-TenantAllowBlockListItems @p | Out-Null
            $item = Get-Item_
            if ($null -eq $item) { throw "The $ListType entry '$Entry' was created but cannot be read back." }
            ConvertTo-BridgeItem $item
        }

        "Update" {
            $p = @{ ListType = $ListType; Entries = @($Entry) } + (Get-ExpirationParams $ListAction)
            if ($PSBoundParameters.ContainsKey("Notes")) { $p["Notes"] = $Notes }
            Set-TenantAllowBlockListItems @p | Out-Null
            ConvertTo-BridgeItem (Get-Item_)
        }

        "Delete" {
            try {
                Remove-TenantAllowBlockListItems -ListType $ListType -Entries @($Entry) -ErrorAction Stop | Out-Null
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("tenant_allow_block_list_item $Action failed: $($_.Exception.Message)")
    exit 1
}
