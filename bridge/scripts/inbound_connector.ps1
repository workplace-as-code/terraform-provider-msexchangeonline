# inbound_connector.ps1 — CRUD for Exchange Online inbound connectors
# Manages mail flow connectors that receive email from external systems.
# Cmdlet noun: InboundConnector
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

    [string]$Identity                    = "",
    [string]$Name                        = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$Enabled,
    [string]$ConnectorType,
    [string]$SenderDomains,
    [string]$SenderIPAddresses,
    [string]$RequireTls,
    [string]$RestrictDomainsToCertificate,
    [string]$TlsSenderCertificateName,
    [string]$Comment
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

$settings = ConvertTo-BridgeSplat $PSBoundParameters -Bool Enabled, RequireTls, RestrictDomainsToCertificate -String ConnectorType, TlsSenderCertificateName, Comment -Csv SenderDomains, SenderIPAddresses




# ConvertTo-DomainList returns plain domain names. Exchange returns sender
# domains as address spaces, for example "smtp:contoso.com;1".
function ConvertTo-DomainList {
    param($Values)
    @($Values | Where-Object { $null -ne $_ } | ForEach-Object { ($_.ToString() -replace '^smtp:', '') -replace ';\d+$', '' })
}

function Get-PropSafe {
    param($Obj, [string]$Prop, $Default = $null)
    $p = $Obj.PSObject.Properties[$Prop]
    if ($null -ne $p) { return $p.Value }
    return $Default
}

function ConvertTo-BridgeConnector {
    param($Connector)
    @{
        Identity                  = $Connector.Identity
        Name                      = $Connector.Name
        Enabled                   = [bool](Get-PropSafe $Connector 'Enabled' $true)
        ConnectorType             = [string](Get-PropSafe $Connector 'ConnectorType' 'OnPremises')
        ConnectorSource           = [string](Get-PropSafe $Connector 'ConnectorSource' 'Default')
        SenderDomains             = @(ConvertTo-DomainList $Connector.SenderDomains)
        SenderIPAddresses         = if ($Connector.SenderIPAddresses) { [string[]]@($Connector.SenderIPAddresses | ForEach-Object { $_.ToString() }) } else { @() }
        RequireTls                = [bool](Get-PropSafe $Connector 'RequireTls' $false)
        RestrictDomainsToCertificate    = [bool](Get-PropSafe $Connector 'RestrictDomainsToCertificate' $false)
        TlsSenderCertificateName = [string](Get-PropSafe $Connector 'TlsSenderCertificateName' '')
        Comment                   = [string](Get-PropSafe $Connector 'Comment' '')
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "Get" {
        try {
            $connector = Get-InboundConnector -Identity $Identity -ErrorAction Stop
            ConvertTo-BridgeConnector -Connector $connector
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "couldn't be found" -or $msg -match "not found" -or $msg -match "ObjectNotFoundException") {
                exit 0
            }
            [Console]::Error.WriteLine("ERROR in inbound_connector Get: $msg")
            exit 1
        }
    }

    "Create" {
        $params = @{ Name = $Name }
        foreach ($k in $settings.Keys) { if ($null -ne $settings[$k]) { $params[$k] = $settings[$k] } }
        $connector = New-InboundConnector @params
        ConvertTo-BridgeConnector -Connector $connector
    }

    "Update" {
        if ($settings.Count -gt 0) { Set-InboundConnector -Identity $Identity @settings | Out-Null }
        $connector = Get-InboundConnector -Identity $Identity -ErrorAction Stop
        ConvertTo-BridgeConnector -Connector $connector
    }

    "Delete" {
        Remove-InboundConnector -Identity $Identity
        Write-Output "{}"
    }
}
