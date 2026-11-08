<#
.SYNOPSIS
    Checks, before the integration tests, that the app can run every cmdlet
    of the bridge scripts.

.DESCRIPTION
    Reads the cmdlets of the provider from tools/new_role_group.ps1, which
    tools/tools_test.go keeps in step with the bridge scripts, connects with
    the credentials of test/run.sh and runs Get-Command on each of them.

    A cmdlet that no management role of the organization contains does not
    exist in Exchange Online: the provider calls a cmdlet Microsoft removed or
    never had. That fails the run, unless the cmdlet belongs to an optional
    test that is off (a cmdlet can also be missing for want of a license). A
    cmdlet that exists but that the app cannot run is a warning: its role is
    missing from the app's role group.

    Run by test/run.sh with the certificate of the app: EXCHANGE_TENANT_ID,
    EXCHANGE_CLIENT_ID, EXCHANGE_CERTIFICATE_PATH or
    EXCHANGE_CERTIFICATE_BASE64, EXCHANGE_CERTIFICATE_PASSWORD, and the
    EXCHANGE_TEST_* variables of the optional tests.
    PREFLIGHT_LIST_ONLY=1 prints the cmdlets and stops (used by the tests).
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# The cmdlet lists of the role group script, read without running it: the
# first assignment of each list is the literal one.
$ast = [System.Management.Automation.Language.Parser]::ParseFile("$root/tools/new_role_group.ps1", [ref]$null, [ref]$null)
$lists = @{}
$assignments = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.AssignmentStatementAst] }, $false)
foreach ($a in $assignments) {
    $name = "$($a.Left)".TrimStart('$')
    if ($name -notin "ProviderCmdlets", "RoleManagementCmdlets" -or $lists.ContainsKey($name)) { continue }
    $lists[$name] = @($a.Right.Expression.SafeGetValue())
}
foreach ($name in "ProviderCmdlets", "RoleManagementCmdlets") {
    if (-not $lists.ContainsKey($name)) { throw "tools/new_role_group.ps1 has no `$$name list" }
}
$cmdlets = @(@($lists.ProviderCmdlets) + @($lists.RoleManagementCmdlets) | Sort-Object -Unique)
if ($env:PREFLIGHT_LIST_ONLY) { $cmdlets; return }

# The cmdlets of the optional tests: missing ones only fail the run when the
# test is on.
$optional = [ordered]@{
    EXCHANGE_TEST_DEFENDER   = 'SafeLinks|SafeAttachment|ATPProtectionPolicyRule'
    EXCHANGE_TEST_PRESET     = 'EOPProtectionPolicyRule|ATPProtectionPolicyRule'
    EXCHANGE_TEST_ENCRYPTION = 'OMEConfiguration|IRMConfiguration|RMSTemplate'
    EXCHANGE_TEST_RBAC       = 'ManagementScope|ManagementRole|RoleGroup|RoleAssignmentPolicy|ServicePrincipal'
    TF_VAR_test_hybrid       = 'OnPremisesOrganization|IntraOrganizationConnector'
}
function Test-Optional([string]$Cmdlet) {
    foreach ($flag in $optional.Keys) {
        $value = [Environment]::GetEnvironmentVariable($flag)
        if ($Cmdlet -match "-($($optional[$flag]))" -and (-not $value -or $value -eq "false")) { return $true }
    }
    $false
}

# Connect like the provider does: the bridge reads the certificate secrets
# from these variables.
if ($env:EXCHANGE_CERTIFICATE_BASE64) { $env:EXCHANGE_BRIDGE_CERTIFICATE_BASE64 = $env:EXCHANGE_CERTIFICATE_BASE64 }
if ($env:EXCHANGE_CERTIFICATE_PASSWORD) { $env:EXCHANGE_BRIDGE_CERTIFICATE_PASSWORD = $env:EXCHANGE_CERTIFICATE_PASSWORD }
$auth = @{
    TenantId        = $env:EXCHANGE_TENANT_ID
    ClientId        = $env:EXCHANGE_CLIENT_ID
    CertificatePath = "$env:EXCHANGE_CERTIFICATE_PATH"
    Environment     = if ($env:EXCHANGE_ENVIRONMENT) { $env:EXCHANGE_ENVIRONMENT } else { "global" }
}
. "$root/bridge/scripts/auth.ps1" @auth

$missing = @($cmdlets | Where-Object { -not (Get-Command $_ -ErrorAction SilentlyContinue) })
$canReadRoles = [bool](Get-Command Get-ManagementRoleEntry -ErrorAction SilentlyContinue)
$errors = 0
foreach ($cmdlet in $missing) {
    $roles = @()
    if ($canReadRoles) {
        $roles = @(Get-ManagementRoleEntry "*\$cmdlet" -ErrorAction SilentlyContinue | ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
    }
    if ($roles.Count -gt 0) {
        Write-Host "  ! $cmdlet is not available to the app (roles: $($roles -join ', '))" -ForegroundColor Yellow
    } elseif (-not $canReadRoles) {
        Write-Host "  ! $cmdlet is not available to the app (it cannot run Get-ManagementRoleEntry to say why)" -ForegroundColor Yellow
    } elseif (Test-Optional $cmdlet) {
        Write-Host "  ! $cmdlet is in no role of the organization (optional test off)" -ForegroundColor Yellow
    } else {
        Write-Host "  x $cmdlet is in no role of the organization: Exchange Online has no such cmdlet" -ForegroundColor Red
        $errors++
    }
}
Disconnect-ExchangeOnline -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
if ($errors -gt 0) { exit 1 }
Write-Host "  $($cmdlets.Count - $missing.Count) of the $($cmdlets.Count) cmdlets of the bridge scripts are available to the app"
