# ome_configuration.ps1 - branding templates of Microsoft Purview Message
# Encryption, with the parameters of exo_object.ps1. Its own script because:
#   - the templates are named by -Identity (New-OMEConfiguration has no -Name);
#   - the default template, "OME Configuration", cannot be created nor removed:
#     Create sets it, Delete leaves it as it is;
#   - the logo (Image) travels as base64 and Exchange takes bytes;
#   - the expiry is set in days and read back as a time span.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("OMEConfiguration")] [string]$Noun,
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$DefaultTemplate = "OME Configuration"
$splat = ConvertFrom-BridgeValues $Values
$props = @(ConvertFrom-BridgeJson $Properties)
if ($splat.ContainsKey("Image")) {
    $splat.Image = if ($splat.Image) { [byte[]][Convert]::FromBase64String($splat.Image) } else { $null }
}
if ($splat.ContainsKey("ExternalMailExpiryInDays")) { $splat.ExternalMailExpiryInDays = [int]$splat.ExternalMailExpiryInDays }

function Read-BridgeTemplate {
    $t = Invoke-BridgeRetry { Get-OMEConfiguration -Identity $Identity }
    $out = [ordered]@{ Name = [string](Get-BridgeProp $t "Identity" $Identity) }
    foreach ($p in $props) {
        $out[$p] = switch ($p) {
            "Image" {
                $bytes = Get-BridgeProp $t "Image" $null
                if ($bytes) { [Convert]::ToBase64String([byte[]]$bytes) } else { "" }
            }
            "ExternalMailExpiryInDays" {
                $span = Get-BridgeProp $t "ExternalMailExpiryInterval" $null
                if ($span) { [long]([timespan]"$span").TotalDays } else { [long]0 }
            }
            default { ConvertTo-BridgeValue (Get-BridgeProp $t $p) }
        }
    }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgeTemplate } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            if ($Identity -eq $DefaultTemplate) {
                if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-OMEConfiguration -Identity $Identity @splat | Out-Null } }
            } else {
                Invoke-BridgeRetry -Create { New-OMEConfiguration -Identity $Identity @splat | Out-Null }
            }
            Read-BridgeTemplate
        }

        "Update" {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-OMEConfiguration -Identity $Identity @splat | Out-Null } }
            Read-BridgeTemplate
        }

        "Delete" {
            if ($Identity -ne $DefaultTemplate) {
                try {
                    Remove-OMEConfiguration -Identity $Identity
                } catch {
                    if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
                }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
