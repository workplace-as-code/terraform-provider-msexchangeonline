# mailbox.ps1 — Read-only data source for Exchange Online mailboxes
# Cmdlet: Get-EXOMailbox (REST, faster than Get-Mailbox)
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment = "global",

    [Parameter(Mandatory)] [ValidateSet("Get")] [string]$Action,
    [Parameter(Mandatory)] [string]$Identity
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment

# Get-EXOMailbox only returns a minimal property set by default: the other
# properties must be requested, or they are missing (and StrictMode throws).
$mailbox = Get-EXOMailbox -Identity $Identity -ErrorAction Stop `
    -Properties DisplayName, EmailAddresses, HiddenFromAddressListsEnabled, LitigationHoldEnabled, RetentionPolicy

# Extract only SMTP proxy addresses (strip the "smtp:" prefix)
$smtpAddresses = @(
    $mailbox.EmailAddresses |
    Where-Object { $_.ToString() -like "smtp:*" -or $_.ToString() -like "SMTP:*" } |
    ForEach-Object { $_.ToString() -replace '^[Ss][Mm][Tt][Pp]:', '' }
)

@{
    Identity                      = [string]$mailbox.Identity
    DisplayName                   = $mailbox.DisplayName
    PrimarySmtpAddress            = $mailbox.PrimarySmtpAddress.ToString()
    RecipientTypeDetails          = [string]$mailbox.RecipientTypeDetails
    HiddenFromAddressListsEnabled = $mailbox.HiddenFromAddressListsEnabled
    LitigationHoldEnabled         = $mailbox.LitigationHoldEnabled
    RetentionPolicy               = if ($mailbox.RetentionPolicy) { [string]$mailbox.RetentionPolicy } else { "" }
    EmailAddresses                = $smtpAddresses
} | ConvertTo-Json -Compress
