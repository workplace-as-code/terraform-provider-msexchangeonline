# organization_config.ps1 — Read/write Exchange Online organization configuration
# Singleton resource — there is exactly one organization config per tenant.
# Cmdlet noun: OrganizationConfig → Get-OrganizationConfig / Set-OrganizationConfig
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Set")] [string]$Action,

    # Writable settings
    [string]$AuditDisabled                      = "",
    [string]$ReadTrackingEnabled                = "",
    [string]$FocusedInboxOn                     = "",
    [string]$MailTipsAllTipsEnabled             = "",
    [string]$MailTipsExternalRecipientsTipsEnabled = "",
    [string]$MailTipsLargeAudienceThreshold     = "",
    [string]$MessageRecallEnabled               = "",
    [string]$OAuth2ClientProfileEnabled         = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment

function Get-PropSafe {
    param($Obj, [string]$Prop, $Default = $null)
    $p = $Obj.PSObject.Properties[$Prop]
    if ($null -ne $p) { return $p.Value }
    return $Default
}

function ConvertTo-Bytes {
    param($Value)
    if ($null -eq $Value -or $Value.ToString() -eq "Unlimited") { return -1 }
    return [int64]$Value.ToBytes()
}

function Read-Config {
    $config = Get-OrganizationConfig -ErrorAction Stop
    $maxSendRaw = Get-PropSafe $config 'MaxSendSize'
    $maxRecvRaw = Get-PropSafe $config 'MaxReceiveSize'

    @{
        Identity                              = $config.Identity
        DisplayName                           = if ($config.DisplayName) { $config.DisplayName } else { "" }
        SCLJunkThreshold                      = if ($null -ne (Get-PropSafe $config 'SCLJunkThreshold')) { [int](Get-PropSafe $config 'SCLJunkThreshold') } else { -1 }
        MaxSendSize                           = ConvertTo-Bytes $maxSendRaw
        MaxReceiveSize                        = ConvertTo-Bytes $maxRecvRaw
        AuditDisabled                         = [bool](Get-PropSafe $config 'AuditDisabled' $false)
        OutboundSpamFilteringEnabled          = [bool](Get-PropSafe $config 'OutboundSpamFilteringEnabled' $true)
        ReadTrackingEnabled                   = [bool](Get-PropSafe $config 'ReadTrackingEnabled' $false)
        FocusedInboxOn                        = if ($null -ne (Get-PropSafe $config 'FocusedInboxOn')) { [bool](Get-PropSafe $config 'FocusedInboxOn') } else { $false }
        MailTipsAllTipsEnabled                = [bool](Get-PropSafe $config 'MailTipsAllTipsEnabled' $true)
        MailTipsExternalRecipientsTipsEnabled = [bool](Get-PropSafe $config 'MailTipsExternalRecipientsTipsEnabled' $false)
        MailTipsLargeAudienceThreshold        = [int](Get-PropSafe $config 'MailTipsLargeAudienceThreshold' 25)
        MessageRecallEnabled                  = [bool](Get-PropSafe $config 'MessageRecallEnabled' $true)
        OAuth2ClientProfileEnabled            = [bool](Get-PropSafe $config 'OAuth2ClientProfileEnabled' $true)
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "Get" {
        Read-Config
    }

    "Set" {
        $params = @{}

        if ($AuditDisabled -ne "")          { $params["AuditDisabled"]          = ($AuditDisabled -eq "true") }
        if ($ReadTrackingEnabled -ne "")    { $params["ReadTrackingEnabled"]    = ($ReadTrackingEnabled -eq "true") }
        if ($MailTipsAllTipsEnabled -ne "") { $params["MailTipsAllTipsEnabled"] = ($MailTipsAllTipsEnabled -eq "true") }
        if ($MailTipsExternalRecipientsTipsEnabled -ne "") {
            $params["MailTipsExternalRecipientsTipsEnabled"] = ($MailTipsExternalRecipientsTipsEnabled -eq "true")
        }
        if ($MailTipsLargeAudienceThreshold -ne "") {
            $params["MailTipsLargeAudienceThreshold"] = [int]$MailTipsLargeAudienceThreshold
        }
        if ($MessageRecallEnabled -ne "")   { $params["MessageRecallEnabled"]   = ($MessageRecallEnabled -eq "true") }
        if ($OAuth2ClientProfileEnabled -ne "") { $params["OAuth2ClientProfileEnabled"] = ($OAuth2ClientProfileEnabled -eq "true") }

        # FocusedInboxOn is a tenant-wide override — only set if explicitly provided
        if ($FocusedInboxOn -ne "") { $params["FocusedInboxOn"] = ($FocusedInboxOn -eq "true") }

        if ($params.Count -gt 0) {
            Set-OrganizationConfig @params -ErrorAction Stop
        }

        Read-Config
    }
}
