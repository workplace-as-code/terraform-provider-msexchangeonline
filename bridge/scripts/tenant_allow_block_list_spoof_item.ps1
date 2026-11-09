# tenant_allow_block_list_spoof_item.ps1 — CRUD for one spoofed sender pair of
# the Tenant Allow/Block List, identified by its spoof type, spoofed user and
# sending infrastructure. Get- must be called without -Identity (it then lists
# the entries, with it they come back empty); the other cmdlets require
# -Identity Default. Set- and Remove- take the entry's GUID in -Ids.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("Internal","External")] [string]$SpoofType,
    [Parameter(Mandatory)] [string]$SpoofedUser,
    [Parameter(Mandatory)] [string]$SendingInfrastructure,
    [ValidateSet("", "Allow", "Block")] [string]$ListAction = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

function Find-BridgeSpoofItem {
    @(Invoke-BridgeRetry { @(Get-TenantAllowBlockListSpoofItems) }) |
        Where-Object { $_.SpoofType -eq $SpoofType -and $_.SpoofedUser -eq $SpoofedUser -and $_.SendingInfrastructure -eq $SendingInfrastructure } |
        Select-Object -First 1
}

function ConvertTo-BridgeSpoofItem {
    param($Item)
    ConvertTo-Json -Compress -InputObject ([ordered]@{
        SpoofType             = [string]$Item.SpoofType
        SpoofedUser           = [string]$Item.SpoofedUser
        SendingInfrastructure = [string]$Item.SendingInfrastructure
        Action                = [string]$Item.Action
    })
}

try {
    switch ($Action) {
        "Get" {
            $item = Find-BridgeSpoofItem
            if ($null -eq $item) { exit 0 }
            ConvertTo-BridgeSpoofItem $item
        }

        "Create" {
            # New- accepts a pair that already exists and says nothing.
            if ($null -ne (Find-BridgeSpoofItem)) {
                throw "The spoofed sender pair already exists: import it."
            }
            $created = Invoke-BridgeRetry -Create {
                New-TenantAllowBlockListSpoofItems -Identity Default -Action $ListAction -SendingInfrastructure $SendingInfrastructure -SpoofedUser $SpoofedUser -SpoofType $SpoofType
            }
            ConvertTo-BridgeSpoofItem $created
        }

        "Update" {
            $item = Find-BridgeSpoofItem
            if ($null -eq $item) { throw "The spoofed sender pair couldn't be found." }
            Invoke-BridgeRetry { Set-TenantAllowBlockListSpoofItems -Identity Default -Ids ([string]$item.Identity) -Action $ListAction | Out-Null }
            $updated = Wait-BridgeConsistent -Read { Find-BridgeSpoofItem } -Check { param($o) $null -ne $o -and $o.Action -eq $ListAction }
            ConvertTo-BridgeSpoofItem $updated
        }

        "Delete" {
            $item = Find-BridgeSpoofItem
            if ($null -ne $item) {
                try {
                    Invoke-BridgeRetry { Remove-TenantAllowBlockListSpoofItems -Identity Default -Ids ([string]$item.Identity) | Out-Null }
                } catch {
                    if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
                }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("tenant_allow_block_list_spoof_item $Action failed: $($_.Exception.Message)")
    exit 1
}
