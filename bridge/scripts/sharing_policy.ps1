# sharing_policy.ps1 - exo_object.ps1 for sharing policies, called with the
# same parameters. Exchange returns the actions of a domain separated by ", "
# ("contoso.com:CalendarSharingFreeBusyDetail, ContactsSharing"): the domains
# go back to Terraform without the spaces, as the configuration writes them.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("SharingPolicy")] [string]$Noun = "SharingPolicy",
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$PSBoundParameters["Noun"] = $Noun
$global:LASTEXITCODE = 0
$output = @(& "$PSScriptRoot/exo_object.ps1" @PSBoundParameters)
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$json = ($output -join "`n").Trim()
# Nothing (not found) or "{}" (deleted): pass it on as is.
if (-not $json -or $json -eq "{}") { $json; exit 0 }

$policy = ConvertFrom-Json -InputObject $json
if ($null -ne $policy.PSObject.Properties["Domains"]) {
    $policy.Domains = @($policy.Domains | Where-Object { $null -ne $_ } | ForEach-Object { "$_" -replace ',\s+', ',' })
}
ConvertTo-Json -Compress -Depth 4 -InputObject $policy
