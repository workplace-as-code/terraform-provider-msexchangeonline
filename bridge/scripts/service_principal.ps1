# service_principal.ps1 - Exchange service principals of Microsoft Entra
# applications (RBAC for Applications), found by app ID.
#
#   -Identity  application (client) ID
#   -Values    base64 JSON: ObjectId (enterprise application), DisplayName
#
# Output: AppId, ObjectId, DisplayName.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$wanted = if ($Values) { ConvertFrom-Json -InputObject ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Values))) } else { $null }

# Find-BridgeServicePrincipal returns the service principal of the app, or $null.
function Find-BridgeServicePrincipal {
    @(Invoke-BridgeRetry { Get-ServicePrincipal }) | Where-Object { [string]$_.AppId -eq $Identity } | Select-Object -First 1
}

function ConvertTo-BridgeServicePrincipal($Sp) {
    ConvertTo-Json -Compress -InputObject ([ordered]@{
        AppId       = [string]$Sp.AppId
        ObjectId    = [string](Get-BridgeProp $Sp "ObjectId" "")
        DisplayName = [string](Get-BridgeProp $Sp "DisplayName" "")
    })
}

try {
    switch ($Action) {
        "Get" {
            $sp = Find-BridgeServicePrincipal
            if ($null -eq $sp) { exit 0 }
            ConvertTo-BridgeServicePrincipal $sp
        }

        "Create" {
            $new = @{ AppId = $Identity; ObjectId = $wanted.ObjectId }
            if ($null -ne $wanted.PSObject.Properties["DisplayName"]) { $new.DisplayName = $wanted.DisplayName }
            Invoke-BridgeRetry -Create { New-ServicePrincipal @new | Out-Null }
            $sp = Wait-BridgeConsistent -Read { Find-BridgeServicePrincipal } -Check { param($o) $null -ne $o }
            ConvertTo-BridgeServicePrincipal $sp
        }

        "Update" {
            $sp = Find-BridgeServicePrincipal
            if ($null -eq $sp) { throw "The service principal of app $Identity couldn't be found." }
            if ($null -ne $wanted.PSObject.Properties["DisplayName"] -and [string]$sp.DisplayName -ne $wanted.DisplayName) {
                Invoke-BridgeRetry { Set-ServicePrincipal -Identity $sp.Identity -DisplayName $wanted.DisplayName }
                $sp = Find-BridgeServicePrincipal
            }
            ConvertTo-BridgeServicePrincipal $sp
        }

        "Delete" {
            $sp = Find-BridgeServicePrincipal
            if ($null -ne $sp) { Remove-ServicePrincipal -Identity $sp.Identity }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("ServicePrincipal $Action failed: $($_.Exception.Message)")
    exit 1
}
