# auth_connect.ps1 - runs auth.ps1 against fake Exchange cmdlets and prints, as
# JSON, the parameters it passed to Connect-ExchangeOnline (run by
# auth_connect_test.go).
#   -Case token           : EXCHANGE_BRIDGE_ACCESS_TOKEN is set
#   -Case token-noclientid : the same, without a client ID (system-assigned managed identity)
#   -Case base64          : EXCHANGE_BRIDGE_CERTIFICATE_BASE64 holds a password-protected .pfx
#   -Case base64-wrapped  : the same, cut in lines of 76 characters like `base64` does
#   -Case base64-nopassword : a .pfx exported without a password
param([string]$Script, [string]$Case)

$ErrorActionPreference = "Stop"

function Get-ConnectionInformation { $null }
function Connect-ExchangeOnline {
    param($AccessToken, $Organization, $AppID, $Certificate, $CertificateThumbprint, $ExchangeEnvironmentName, [switch]$ShowBanner)
    $global:Connected = [ordered]@{
        AccessToken = $AccessToken
        Organization = $Organization
        AppID = $AppID
        Thumbprint = if ($Certificate) { $Certificate.Thumbprint } else { $null }
    }
}

$expected = $null
switch ($Case) {
    { $_ -like "token*" } {
        $env:EXCHANGE_BRIDGE_ACCESS_TOKEN = "at-1"
    }
    { $_ -like "base64*" } {
        $rsa  = [System.Security.Cryptography.RSA]::Create(2048)
        $req  = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
            "CN=auth-connect-test", $rsa,
            [System.Security.Cryptography.HashAlgorithmName]::SHA256,
            [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)
        $cert = $req.CreateSelfSigned([DateTimeOffset]::Now, [DateTimeOffset]::Now.AddDays(1))
        $password = if ($Case -eq "base64-nopassword") { $null } else { "pw" }
        $b64  = [Convert]::ToBase64String($cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, $password))
        if ($Case -eq "base64-wrapped") {
            $b64 = (($b64 -split '(.{76})') | Where-Object { $_ }) -join "`n"
        }
        $env:EXCHANGE_BRIDGE_CERTIFICATE_BASE64   = $b64
        $env:EXCHANGE_BRIDGE_CERTIFICATE_PASSWORD = $password
        $expected = $cert.Thumbprint
    }
    default { throw "unknown case $Case" }
}

$clientId = if ($Case -eq "token-noclientid") { "" } else { "00000000-0000-0000-0000-000000000001" }
. $Script -TenantId contoso -ClientId $clientId

ConvertTo-Json -Compress -InputObject @{ connected = $global:Connected; expectedThumbprint = $expected }
