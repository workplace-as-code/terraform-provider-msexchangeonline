# role_group.ps1 - exo_object.ps1 for role groups, called with the same
# parameters. The provider's app is not a manager of the role groups it
# creates: Set-RoleGroup and Remove-RoleGroup need
# -BypassSecurityGroupManagerCheck, which Role Management allows. Members are
# never read nor written.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("RoleGroup")] [string]$Noun,
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$global:PSDefaultParameterValues["Set-RoleGroup:BypassSecurityGroupManagerCheck"] = $true
$global:PSDefaultParameterValues["Remove-RoleGroup:BypassSecurityGroupManagerCheck"] = $true
$global:LASTEXITCODE = 0
& "$PSScriptRoot/exo_object.ps1" @PSBoundParameters
exit $LASTEXITCODE
