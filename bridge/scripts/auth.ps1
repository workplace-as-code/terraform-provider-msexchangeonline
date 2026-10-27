# auth.ps1 — Exchange Online authentication
# Dot-sourced by all bridge/scripts scripts.
#
# Runs in PowerShell 7+ (pwsh) with the ExchangeOnlineManagement v3 module.
# App-only authentication, in this order:
#   - an access token obtained by the provider with OIDC or a managed identity
#     (EXCHANGE_BRIDGE_ACCESS_TOKEN);
#   - a certificate: Windows store thumbprint, base64 content of a .pfx
#     (EXCHANGE_BRIDGE_CERTIFICATE_BASE64), or .pfx file.
# The app needs Exchange.ManageAsApp and the management roles of the provider's
# role group (or Exchange Administrator).

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$CertificateThumbprint = "",
    [string]$Environment           = "global"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Exchange Online answers in the language of the machine. The scripts recognize
# errors by their English text ("couldn't be found", ...): on a French machine a
# missing object would be reported as a failure. Ask for English, whatever the OS.
[System.Globalization.CultureInfo]::CurrentCulture   = [System.Globalization.CultureInfo]::GetCultureInfo("en-US")
[System.Globalization.CultureInfo]::CurrentUICulture = [System.Globalization.CultureInfo]::GetCultureInfo("en-US")

# The connection runs in a child scope. This file is dot-sourced into every
# bridge script: a variable assigned here would overwrite the script's own
# parameter of the same name, without regard to case (msexchangeserver once
# created a transport rule named after the Exchange snap-in this way).
& {
    param([string]$TenantId, [string]$ClientId, [string]$CertificatePath, [string]$CertificatePassword, [string]$CertificateThumbprint, [string]$Environment)

    # Never prompt: pass to every cmdlet that has the parameter.
    # Some Exchange Online cmdlets (Remove-QuarantinePolicy, ...) do not have it,
    # so it cannot be passed explicitly.
    $global:PSDefaultParameterValues["*:Confirm"] = $false

    # Secrets are passed by the Go client through environment variables, never on the
    # command line (command lines are visible to every local user and are logged by
    # process-creation auditing). Explicit parameters still win, for manual testing.
    if (-not $CertificatePassword -and $env:EXCHANGE_BRIDGE_CERTIFICATE_PASSWORD) { $CertificatePassword = $env:EXCHANGE_BRIDGE_CERTIFICATE_PASSWORD }
    $accessToken       = $env:EXCHANGE_BRIDGE_ACCESS_TOKEN
    $certificateBase64 = $env:EXCHANGE_BRIDGE_CERTIFICATE_BASE64

    $exchangeEnvironments = @{
        "global"  = "O365Default"
        "gcc"     = "O365Default"
        "gcchigh" = "O365USGovGCCHigh"
        "dod"     = "O365USGovDoD"
        "china"   = "O365China"
    }

    if (-not $exchangeEnvironments.ContainsKey($Environment)) {
        throw "Invalid environment: '$Environment'. Valid values: global, gcc, gcchigh, dod, china"
    }

    # Skip if already connected
    $existing = Get-ConnectionInformation -ErrorAction SilentlyContinue
    if ($existing) { return }

    # A certificate belongs to an app registration, whose client ID Exchange
    # needs. An access token already names its identity: a system-assigned
    # managed identity has no client ID at all.
    if (-not $accessToken -and -not $ClientId) { throw "client_id is required" }

    # Connect-ExchangeOnline -Organization needs the primary .onmicrosoft.com
    # domain. Accept the short name ("contoso") too, but not a tenant GUID.
    if ($TenantId -match '^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$') {
        throw "tenant_id must be the tenant's .onmicrosoft.com domain (for example contoso.onmicrosoft.com), not a tenant GUID"
    }
    $organization = if ($TenantId -match '\.') { $TenantId } else { "$TenantId.onmicrosoft.com" }
    $envName      = $exchangeEnvironments[$Environment]

    # EphemeralKeySet keeps the private key out of the key store, but macOS does
    # not support it: there the default loads the key into a temporary keychain.
    $certFlags = if ($IsMacOS) {
        [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::DefaultKeySet
    } else {
        [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::EphemeralKeySet
    }

    try {
        if ($accessToken) {
            # OIDC or managed identity: the provider obtained this access token
            # and starts a new process before it expires.
            Connect-ExchangeOnline `
                -AccessToken $accessToken `
                -Organization $organization `
                -ExchangeEnvironmentName $envName `
                -ShowBanner:$false
        } elseif ($CertificateThumbprint) {
            # Windows certificate store — thumbprint lookup
            Connect-ExchangeOnline `
                -CertificateThumbprint $CertificateThumbprint `
                -AppID $ClientId `
                -Organization $organization `
                -ExchangeEnvironmentName $envName `
                -ShowBanner:$false
        } else {
            if ($certificateBase64) {
                # Content of the .pfx, decoded in memory: no file on disk. A
                # value cut in lines (base64 without -A, a vault) is accepted.
                $bytes = [Convert]::FromBase64String(($certificateBase64 -replace '\s', ''))
                $cert  = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new(
                    [byte[]]$bytes,
                    $CertificatePassword,
                    $certFlags
                )
            } elseif ($CertificatePath) {
                $cert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new(
                    $CertificatePath,
                    $CertificatePassword,
                    $certFlags
                )
            } else {
                throw "Either certificate_path, certificate_base64 or certificate_thumbprint is required"
            }
            Connect-ExchangeOnline `
                -Certificate $cert `
                -AppID $ClientId `
                -Organization $organization `
                -ExchangeEnvironmentName $envName `
                -ShowBanner:$false
        }
    } catch {
        throw "Failed to connect to Exchange Online: $_"
    }
} $TenantId $ClientId $CertificatePath $CertificatePassword $CertificateThumbprint $Environment
