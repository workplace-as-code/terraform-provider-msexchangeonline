---
page_title: "Authentication Guide"
subcategory: "Guides"
---

# Authentication Guide

The Microsoft Exchange Online provider uses **certificate-based app-only authentication**. Client secrets are not supported for app-only access (`Connect-ExchangeOnline` requires a certificate).


---

## Exchange Online — Certificate-Based Auth (CBA)

### Why a certificate?

Microsoft requires app-only (unattended) connections to Exchange Online PowerShell to use a certificate rather than a client secret. This is enforced by the `ExchangeOnlineManagement` PowerShell module.

The names below (`terraform-msexchangeonline`) are suggestions: any name works.

### Step 1 — Create an App Registration in Entra ID

1. Go to **Entra admin center → App registrations → New registration**
2. Name: `terraform-msexchangeonline`, supported account types: **Single tenant**, then **Register**
3. Note the **Application (client) ID** and the tenant's primary `.onmicrosoft.com` domain (**Entra admin center → Identity → Settings → Domain names**). The provider's `tenant_id` must be that domain: `Connect-ExchangeOnline` does not accept a tenant GUID.
4. Under **API permissions → Add a permission → APIs my organization uses**, select **Office 365 Exchange Online → Application permissions → Exchange.ManageAsApp**
5. Click **Grant admin consent**, then remove the default delegated `User.Read` permission

`Exchange.ManageAsApp` grants no rights by itself: it only lets the app open an Exchange Online PowerShell session. What the session can do comes from the roles assigned in the next step.

Do not add `Exchange.ManageAsAppV2` or `Exchange.AdminAPI.ManageAsApp`: they are for the Exchange Online Admin REST API, which the provider does not use.

### Step 2 — Grant Exchange Permissions

Choose one option.

**Option A — Least-privilege role group (recommended)**

The provider only needs the management roles that contain the cmdlets it runs. [`tools/new_role_group.ps1`](https://github.com/workplace-as-code/terraform-provider-msexchangeonline/blob/main/tools/new_role_group.ps1) asks Exchange Online which roles contain those cmdlets, creates a role group with just those roles and adds the app to it. Unlike Exchange Administrator, the group contains nothing the provider does not use. Review the list printed by the first run before applying it.

You need the Object ID of the app's **service principal**: **Entra admin center → Enterprise applications → terraform-msexchangeonline → Object ID**. It differs from the Object ID shown under App registrations.

```powershell
Connect-ExchangeOnline   # as an Exchange or Global administrator

# 1. Show the roles the provider needs (changes nothing)
./tools/new_role_group.ps1

# 2. Create the "terraform-msexchangeonline" role group and add the app
./tools/new_role_group.ps1 -AppId "<client-id>" -ServicePrincipalObjectId "<enterprise-app-object-id>" -Apply
```

Run the script again after upgrading the provider: new resources can need new roles, and the script adds the missing ones to the existing group.

**Option B — Exchange Administrator role**

Simpler, but broader: the app can run every Exchange Online cmdlet, including mailbox and recipient management.

1. **Entra admin center → Roles & admins → Exchange Administrator → Add assignments**
2. Select the app registration and assign

Role assignments can take up to 30 minutes to apply.

### Step 3 — Generate a Certificate

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

### Step 4 — Upload the Certificate to the App Registration

1. Open the app registration in Entra ID
2. Go to **Certificates & secrets → Certificates → Upload certificate**
3. Upload `terraform-msexchangeonline.cer` and note its expiry date

### Step 5 — Configure the Provider

**Using certificate file (.pfx):**

```hcl
provider "msexchangeonline" {
  tenant_id            = var.tenant_id
  client_id            = var.client_id
  certificate_path     = "/path/to/terraform-msexchangeonline.pfx"
  certificate_password = var.certificate_password
}
```

**Using thumbprint (Windows — certificate in local store):**

```hcl
provider "msexchangeonline" {
  tenant_id              = var.tenant_id
  client_id              = var.client_id
  certificate_thumbprint = "AABBCCDDEEFF..."
}
```

**Via environment variables (recommended for CI/CD):**

```bash
export EXCHANGE_TENANT_ID="contoso.onmicrosoft.com"
export EXCHANGE_CLIENT_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export EXCHANGE_CERTIFICATE_PATH="/secrets/terraform-msexchangeonline.pfx"
export EXCHANGE_CERTIFICATE_PASSWORD="your-pfx-password"
```

Values set in the `provider` block take precedence over environment variables. Environment variables are only used for attributes left unset in HCL.

The provider never passes secrets on the PowerShell command line. The certificate password and the on-premises password reach the bridge scripts through the child process environment.

### Cloud Environments

For sovereign clouds, set the `environment` parameter:

```hcl
provider "msexchangeonline" {
  environment = "gcchigh" # Valid: global (default), gcc, gcchigh, dod, china
  # ... other fields
}
```

Or via env var: `export EXCHANGE_ENVIRONMENT="gcchigh"`

---

---

## Production: Separate Read and Write Apps

`terraform plan` only reads; `terraform apply` changes the tenant. When they run in different contexts, give them different apps:

| App | Used by | Permissions |
|---|---|---|
| `terraform-msexchangeonline-read` | `terraform plan` on pull requests, scheduled drift detection | `Exchange.ManageAsApp` + the **View-Only Organization Management** role group, assigned like Option A (`New-ServicePrincipal`, then `Add-RoleGroupMember`). The **Global Reader** Entra role also works but reads the whole tenant, not only Exchange. |
| `terraform-msexchangeonline` | `terraform apply`, only from the protected branch after approval | `Exchange.ManageAsApp` + the least-privilege role group (Option A) |

If the read certificate leaks from a CI job, the configuration can be read but not changed. Keep the write certificate in a secret scoped to the deployment environment (for example a GitHub Actions environment with required reviewers).

For a single administrator testing on one tenant, one write app is enough.

---

## Storing Secrets Safely

Never commit credentials to version control. Recommended approaches:

- **HashiCorp Vault** — inject secrets as environment variables in CI
- **GitHub Actions secrets** — store the `.pfx` base64-encoded (`base64 -w0 terraform-msexchangeonline.pfx`), write it to a temporary file in the job, and pass its password via `${{ secrets.EXCHANGE_CERTIFICATE_PASSWORD }}`
- **Azure Key Vault** — retrieve the PFX at pipeline runtime
- **Terraform Cloud** — store sensitive variables in workspace settings

---

## Runtime tuning

These optional environment variables control how the provider runs PowerShell:

| Variable | Default | Purpose |
|---|---|---|
| `EXCHANGE_SCRIPT_TIMEOUT` | `10m` | Maximum duration of one bridge script run, as a Go duration. |
| `EXCHANGE_MAX_CONCURRENCY` | `4` | Maximum number of PowerShell processes running at once. Lower it if Exchange Online throttles you. |
| `EXCHANGE_BRIDGE_DIR` | unset | Run the bridge scripts from this folder instead of the copy embedded in the provider. For script development only. |
| `EXCHANGE_BRIDGE_MODE` | `persistent` | `persistent` keeps up to `EXCHANGE_MAX_CONCURRENCY` PowerShell processes alive and reuses their Exchange connection. `oneshot` starts a new process, and a new connection, for every operation. Use it only for troubleshooting. |
| `EXCHANGE_WORKER_MAX_REQUESTS` | `200` | Number of operations a persistent PowerShell process serves before it is replaced. |

In persistent mode, a process that hits an error, a timeout or a crash is discarded and the next operation starts a fresh one. The processes disconnect from Exchange when Terraform finishes.

---

## Troubleshooting

Connection errors (`AADSTS` codes), certificate, role and license problems are covered in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/troubleshooting).
