<#
.SYNOPSIS
    Removes the report submission rule and policy of the test tenant, so the
    tenant goes back to its state before the tests. Run by test/run.sh, with
    the certificate of the app.
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

if ($env:EXCHANGE_CERTIFICATE_BASE64) { $env:EXCHANGE_BRIDGE_CERTIFICATE_BASE64 = $env:EXCHANGE_CERTIFICATE_BASE64 }
if ($env:EXCHANGE_CERTIFICATE_PASSWORD) { $env:EXCHANGE_BRIDGE_CERTIFICATE_PASSWORD = $env:EXCHANGE_CERTIFICATE_PASSWORD }
$auth = @{
    TenantId        = $env:EXCHANGE_TENANT_ID
    ClientId        = $env:EXCHANGE_CLIENT_ID
    CertificatePath = "$env:EXCHANGE_CERTIFICATE_PATH"
    Environment     = if ($env:EXCHANGE_ENVIRONMENT) { $env:EXCHANGE_ENVIRONMENT } else { "global" }
}
. "$root/bridge/scripts/auth.ps1" @auth

foreach ($rule in @(Get-ReportSubmissionRule)) { Remove-ReportSubmissionRule -Identity $rule.Identity }
$zero = "00000000-0000-0000-0000-000000000000"
if ([string](Get-ReportSubmissionPolicy).Guid -ne $zero) { Remove-ReportSubmissionPolicy -Identity DefaultReportSubmissionPolicy }
$guid = [string](Get-ReportSubmissionPolicy).Guid
Disconnect-ExchangeOnline -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
if ($guid -ne $zero) { throw "The report submission policy still exists (Guid $guid)." }
