---
page_title: "Authentication Guide"
subcategory: "Guides"
---

# Authentication Guide

The provider connects to Exchange Online with **app-only authentication**: an Entra ID app registration or an Azure managed identity, not a user account. This page helps you choose how the provider proves who it is, then covers the setup every method shares: the app registration and its Exchange permissions.

## Choose a Method

| Method | `auth_method` | Works from | Secret stored where Terraform runs | Renewal |
|---|---|---|---|---|
| Certificate: `.pfx` file, base64 content or Windows certificate store | `client_certificate` (default) | Anywhere: workstations, internal CI runners, servers | The `.pfx` and its password | Upload a new certificate before the old one expires (1 to 2 years) |
| OIDC from GitHub Actions | `oidc_github` | GitHub Actions jobs | Nothing | None: GitHub issues a token for each job |
| OIDC from another platform | `oidc` | A CI platform or cluster whose OIDC issuer Entra ID can reach over the internet | Nothing | None: the platform issues the token |
| Managed identity | `managed_identity` | Azure VMs and other Azure compute, Azure Arc-enabled servers | Nothing | None: Azure manages the identity's credential |

- **Internal CI (an on-premises GitLab or Jenkins), workstations, servers**: use a certificate. OIDC needs Entra ID to download the signing keys of the platform that issues the token, over public HTTPS: an internal platform cannot be reached. See [Certificate authentication](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_client_certificate).
- **GitHub Actions**: use `oidc_github`. Nothing to store in GitHub, nothing to renew, and Entra ID only accepts tokens from the repository and environment you declare. See [OIDC with GitHub Actions](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc_github).
- **gitlab.com, Kubernetes, another platform with a public OIDC issuer**: use `oidc`. See [OIDC with any platform](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc).
- **Terraform runs on an Azure VM, a runner hosted in Azure or an Azure Arc-enabled server**: use `managed_identity`. No app registration and nothing to store: the machine's identity connects. See [Managed identity](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_managed_identity).

The provider does not offer client secrets: a certificate, OIDC or a managed identity avoids storing a password-like credential.

---

## Step 1 — Create an App Registration in Entra ID

Every method except `managed_identity` uses the same app registration. With a managed identity, the identity replaces the app registration: follow the [Managed identity guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_managed_identity), which grants `Exchange.ManageAsApp` with Microsoft Graph, then come back to step 2 with the identity's client ID and Object ID. The names below (`terraform-msexchangeonline`) are suggestions: any name works.

1. Go to **Entra admin center → App registrations → New registration**
2. Name: `terraform-msexchangeonline`, supported account types: **Single tenant**, then **Register**
3. Note the **Application (client) ID** and the tenant's primary `.onmicrosoft.com` domain (**Entra admin center → Identity → Settings → Domain names**). The provider's `tenant_id` must be that domain: `Connect-ExchangeOnline` does not accept a tenant GUID.
4. Under **API permissions → Add a permission → APIs my organization uses**, select **Office 365 Exchange Online → Application permissions → Exchange.ManageAsApp**
5. Click **Grant admin consent**, then remove the default delegated `User.Read` permission

`Exchange.ManageAsApp` grants no rights by itself: it only lets the app open an Exchange Online PowerShell session. What the session can do comes from the roles assigned in the next step.

Do not add `Exchange.ManageAsAppV2` or `Exchange.AdminAPI.ManageAsApp`: they are for the Exchange Online Admin REST API, which the provider does not use.

## Step 2 — Grant Exchange Permissions

Choose one option.

**Option A — Least-privilege role group (recommended)**

[`tools/new_role_group.ps1`](https://github.com/workplace-as-code/terraform-provider-msexchangeonline/blob/main/tools/new_role_group.ps1) creates a role group with only the management roles that contain the cmdlets the provider runs, and adds the app to it. Unlike Exchange Administrator, the group cannot manage mailboxes or other recipients. Its roles:

- **E-Mail Address Policies**
- **Journaling**
- **Organization Configuration**
- **Organization Transport Settings**
- **Recipient Policies**
- **Remote and Accepted Domains**
- **Retention Management**
- **Transport Hygiene**
- **Transport Rules**

The script first checks with Exchange Online that these roles contain every cmdlet of the provider, and stops if Microsoft has moved one.

You need the Object ID of the app's **service principal**: **Entra admin center → Enterprise applications → terraform-msexchangeonline → Object ID**. It differs from the Object ID shown under App registrations.

```powershell
Connect-ExchangeOnline   # as an Exchange or Global administrator

# 1. Show the roles the provider needs (changes nothing)
./tools/new_role_group.ps1

# 2. Create the "terraform-msexchangeonline" role group and add the app
./tools/new_role_group.ps1 -AppId "<client-id>" -ServicePrincipalObjectId "<enterprise-app-object-id>" -Apply
```

Run the script again after upgrading the provider: it adds the roles a new version needs to the existing group, and lists the roles of the group that are not in the list above: `-Apply -RemoveExtraRoles` removes them. Versions before 0.3.1 granted Mail Recipient Creation, which can create and delete mailboxes: run the script with `-Apply -RemoveExtraRoles` to remove it.

**Option B — Exchange Administrator role**

Simpler, but broader: the app can run every Exchange Online cmdlet, including mailbox and recipient management.

1. **Entra admin center → Roles & admins → Exchange Administrator → Add assignments**
2. Select the app registration and assign

Role assignments can take up to 30 minutes to apply.

## Step 3 — Give the App Its Credential

Follow the guide of the method you chose:

- [Certificate authentication](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_client_certificate): generate a certificate, upload its public key to the app, give the `.pfx` to the provider.
- [OIDC with GitHub Actions](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc_github): declare which repository and environment the app trusts.
- [OIDC with any platform](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc): declare which issuer and subject the app trusts.
- [Managed identity](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_managed_identity): nothing to give, the identity has no credential to store. Configure the provider as step 4 of that guide shows.

---

## Provider Settings

Values set in the `provider` block take precedence over environment variables. Environment variables are only used for attributes left unset in HCL.

| Attribute | Environment variable | Methods |
|---|---|---|
| `tenant_id` | `EXCHANGE_TENANT_ID` | All (required) |
| `client_id` | `EXCHANGE_CLIENT_ID` | All, required except with `managed_identity`, where it selects a user-assigned identity |
| `auth_method` | `EXCHANGE_AUTH_METHOD` | All (default `client_certificate`) |
| `certificate_path`, `certificate_base64`, `certificate_thumbprint`, `certificate_password` | `EXCHANGE_CERTIFICATE_PATH`, `EXCHANGE_CERTIFICATE_BASE64`, `EXCHANGE_CERTIFICATE_THUMBPRINT`, `EXCHANGE_CERTIFICATE_PASSWORD` | `client_certificate` |
| `oidc_token`, `oidc_token_file_path` | `EXCHANGE_OIDC_TOKEN`, `EXCHANGE_OIDC_TOKEN_FILE_PATH` | `oidc` |
| `environment` | `EXCHANGE_ENVIRONMENT` | All |

The provider checks these settings at plan time: an attribute that does not belong to the chosen method, or two certificate sources at once, fails with a message that says what to change. Without `auth_method`, `EXCHANGE_OIDC_*` variables left in the environment are ignored, so an existing certificate configuration keeps working.

The provider never passes secrets on the PowerShell command line. The certificate, its password and the access token reach the bridge scripts through the child process environment.

### Cloud Environments

For sovereign clouds, set the `environment` parameter:

```hcl
provider "msexchangeonline" {
  environment = "gcchigh" # Valid: global (default), gcc, gcchigh, dod, china
  # ... other fields
}
```

Or via env var: `export EXCHANGE_ENVIRONMENT="gcchigh"`

OIDC and managed identities are only supported in the `global` and `gcc` environments for now. Use a certificate in the other clouds.

---

## Production: Separate Read and Write Apps

`terraform plan` only reads; `terraform apply` changes the tenant. When they run in different contexts, give them different apps:

| App | Used by | Permissions |
|---|---|---|
| `terraform-msexchangeonline-read` | `terraform plan` on pull requests, scheduled drift detection | `Exchange.ManageAsApp` + the **View-Only Organization Management** role group, assigned like Option A (`New-ServicePrincipal`, then `Add-RoleGroupMember`). The **Global Reader** Entra role also works but reads the whole tenant, not only Exchange. |
| `terraform-msexchangeonline` | `terraform apply`, only from the protected branch after approval | `Exchange.ManageAsApp` + the least-privilege role group (Option A) |

If the read credential leaks from a CI job, the configuration can be read but not changed. With a certificate, keep the write certificate in a secret scoped to the deployment environment (for example a GitHub Actions environment with required reviewers). With OIDC, make the write app trust only that protected environment.

For a single administrator testing on one tenant, one write app is enough.

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

In persistent mode, a process that hits an error, a timeout or a crash is discarded and the next operation starts a fresh one. With OIDC, a process is also replaced a few minutes before its access token expires, so runs longer than an hour keep working. The processes disconnect from Exchange when Terraform finishes.

---

## Troubleshooting

Connection errors (`AADSTS` codes), certificate, OIDC, role and license problems are covered in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/troubleshooting).
