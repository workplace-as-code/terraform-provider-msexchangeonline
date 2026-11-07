# management_scope.ps1 - exo_object.ps1 for management scopes, called with the
# same parameters. Creating an exclusive scope asks for confirmation: -Force
# answers it, since the configuration already says the scope is exclusive.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("ManagementScope")] [string]$Noun,
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$global:PSDefaultParameterValues["New-ManagementScope:Force"] = $true
$global:PSDefaultParameterValues["Set-ManagementScope:Force"] = $true
$global:PSDefaultParameterValues["Remove-ManagementScope:Force"] = $true
$global:LASTEXITCODE = 0
& "$PSScriptRoot/exo_object.ps1" @PSBoundParameters
exit $LASTEXITCODE
