---
page_title: "Certificate Authentication"
subcategory: "Guides"
---

# Certificate Authentication

With `auth_method = "client_certificate"` (the default), the provider proves it is the app registration with a certificate: the app holds the public key, the provider holds the private key in a `.pfx`. It works anywhere Terraform runs, including internal CI runners that Entra ID cannot reach.

Before you start, create the app registration and grant its Exchange permissions: steps 1 and 2 of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication).

## How It Works

1. The provider loads the `.pfx`: from a file, from its base64 content, or from the Windows certificate store.
2. `Connect-ExchangeOnline -Certificate` signs a request with the private key; Entra ID checks the signature against the public key uploaded to the app and issues an access token.
3. The provider runs the Exchange cmdlets with that session.

## Step 1 — Generate a Certificate

**Option A — Self-signed, with PowerShell on Windows**

```powershell
$cert = New-SelfSignedCertificate `
    -Subject "CN=terraform-msexchangeonline" `
    -CertStoreLocation "Cert:\CurrentUser\My" `
    -KeyExportPolicy Exportable `
    -KeySpec KeyExchange `
    -KeyLength 2048 `
    -KeyAlgorithm RSA `
    -HashAlgorithm SHA256 `
    -NotAfter (Get-Date).AddYears(2)

# Public key (.cer): upload it to the app registration
Export-Certificate -Cert $cert -FilePath "terraform-msexchangeonline.cer"

# Private key (.pfx): used by the provider
$password = Read-Host -AsSecureString "PFX password"
Export-PfxCertificate -Cert $cert -FilePath "terraform-msexchangeonline.pfx" -Password $password
```

**Option B — OpenSSL (Linux, macOS, CI)**

```bash
openssl req -x509 -newkey rsa:2048 -keyout terraform-msexchangeonline.key \
    -out terraform-msexchangeonline.cer -days 730 -nodes -subj "/CN=terraform-msexchangeonline"

openssl pkcs12 -export -out terraform-msexchangeonline.pfx \
    -inkey terraform-msexchangeonline.key -in terraform-msexchangeonline.cer

rm terraform-msexchangeonline.key   # the private key now lives only in the .pfx
```

A certificate issued by your internal PKI works the same way: the provider only needs the `.pfx`, Entra ID only the public key.

## Step 2 — Upload the Certificate to the App Registration

1. Open the app registration in Entra ID
2. Go to **Certificates & secrets → Certificates → Upload certificate**
3. Upload `terraform-msexchangeonline.cer` and note its expiry date: upload the next certificate before that date, or Terraform stops connecting

## Step 3 — Configure the Provider

Give the provider exactly one certificate source.

**`.pfx` file:**

```hcl
provider "msexchangeonline" {
  tenant_id            = var.tenant_id
  client_id            = var.client_id
  certificate_path     = "/path/to/terraform-msexchangeonline.pfx"
  certificate_password = var.certificate_password
}
```

**Base64 content of the `.pfx`** — for a certificate injected by a vault or a masked CI variable, without writing a file on the runner:

```hcl
provider "msexchangeonline" {
  tenant_id            = var.tenant_id
  client_id            = var.client_id
  certificate_base64   = var.certificate_base64
  certificate_password = var.certificate_password
}
```

**Thumbprint (Windows — certificate in the local store):** the private key can stay non-exportable.

```hcl
provider "msexchangeonline" {
  tenant_id              = var.tenant_id
  client_id              = var.client_id
  certificate_thumbprint = "AABBCCDDEEFF..."
}
```

**Environment variables (recommended for CI/CD):**

```bash
export EXCHANGE_TENANT_ID="contoso.onmicrosoft.com"
export EXCHANGE_CLIENT_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export EXCHANGE_CERTIFICATE_PATH="/secrets/terraform-msexchangeonline.pfx"
export EXCHANGE_CERTIFICATE_PASSWORD="your-pfx-password"
```

Replace `EXCHANGE_CERTIFICATE_PATH` with `EXCHANGE_CERTIFICATE_BASE64` to pass the content instead of a path.

## The Base64 Certificate

`certificate_base64` takes the whole `.pfx` file, private key included, encoded in standard base64:

```bash
openssl base64 -A -in terraform-msexchangeonline.pfx
```

- The value may be cut in lines (`base64` without `-A`, a value copied from a vault): the provider removes the line breaks.
- The provider decodes it in memory and passes it to PowerShell through the process environment. No file is written.
- **Base64 is not encryption.** Store the value as a secret: a masked and protected CI variable, a vault, a sensitive Terraform variable. Upload only the public certificate (`.cer`) to Entra ID.

From Azure Key Vault, read the certificate **as a secret**: the secret that Key Vault keeps under the certificate's name holds the whole `.pfx`, private key included, in base64 and without a password, when the certificate's content type is `application/x-pkcs12` (the default; with PEM the value is PEM text, which the provider does not accept). The `azurerm_key_vault_certificate` data source does not fit: its `certificate_data_base64` is the public certificate only.

```hcl
data "azurerm_key_vault_secret" "exchange" {
  name         = "terraform-msexchangeonline" # the certificate's name
  key_vault_id = var.key_vault_id
}

provider "msexchangeonline" {
  tenant_id          = var.tenant_id
  client_id          = var.client_id
  certificate_base64 = data.azurerm_key_vault_secret.exchange.value
}
```

## Security Best Practices

- **Never commit** a `.pfx` or its password. Keep them in a vault or in CI secrets scoped to the deployment environment.
- **Prefer the Windows certificate store** on Windows runners: the private key can be marked non-exportable.
- **Use separate read and write apps** in production, each with its own certificate (see the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication)).
- **Track the expiry date**: an expired certificate stops every run at once.
- **Consider OIDC** where the CI platform supports it: nothing to store and nothing to renew. See [OIDC with GitHub Actions](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc_github).

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `Missing certificate` | No certificate source | Set `certificate_path`, `certificate_base64` or `certificate_thumbprint` (or their `EXCHANGE_CERTIFICATE_*` variable) |
| `Conflicting certificate settings` | Two sources at once, possibly one from an environment variable | Keep one source; unset the leftover `EXCHANGE_CERTIFICATE_*` variable |
| `Invalid certificate_base64` | The value is not base64 | Encode the whole `.pfx`: `openssl base64 -A -in app.pfx` |
| `AADSTS700027` | The certificate is not registered on the application, or has expired | Upload the `.cer` of the same certificate as the `.pfx`, and check its end date in **Certificates & secrets** |

More errors are listed in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/troubleshooting).
