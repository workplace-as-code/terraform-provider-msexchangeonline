# management_role_info.ps1 — Reads a management role, built-in or custom, and
# the cmdlets it contains. Used by the data source: management_role.ps1 refuses
# built-in roles. Empty stdout when the role does not exist.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [string]$Name
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment
. "$PSScriptRoot/common.ps1"

try {
    try {
        $role = Invoke-BridgeRetry { Get-ManagementRole -Identity $Name }
    } catch {
        if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
        throw
    }
    $cmdlets = @(Invoke-BridgeRetry { Get-ManagementRoleEntry -Identity "$($role.Name)\*" } |
        ForEach-Object { [string]$_.Name } | Sort-Object -Unique)
    # Exchange Online returns the parent as "<tenant>\<role>".
    $parent = ([string]$role.Parent -split '\\')[-1]
    ConvertTo-Json -Compress -InputObject ([ordered]@{
        Name        = [string]$role.Name
        Description = [string]$role.Description
        Parent      = $parent
        BuiltIn     = [bool]$role.IsRootRole
        Cmdlets     = [string[]]$cmdlets
    })
} catch {
    [Console]::Error.WriteLine("management_role_info failed: $($_.Exception.Message)")
    exit 1
}
