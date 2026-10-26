---
page_title: "OIDC with Any Platform"
subcategory: "Guides"
---

# OIDC with Any Platform

With `auth_method = "oidc"`, the provider exchanges an OIDC token issued by your CI platform or cluster for an Exchange Online access token. Nothing secret is stored: the platform issues the token, and Entra ID accepts it if its issuer and subject match a federated credential of the app registration.

For GitHub Actions, use [`oidc_github`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc_github) instead: the provider requests the token itself.

Before you start, create the app registration and grant its Exchange permissions: steps 1 and 2 of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication). The machine running Terraform needs the ExchangeOnlineManagement module **3.1.0 or later**.

## The Limit to Check First

Entra ID verifies the token's signature by downloading the issuer's OpenID configuration and public keys (`<issuer>/.well-known/openid-configuration`) **over the internet, with a publicly trusted certificate**. An internal GitLab, Jenkins or Kubernetes API server that Entra ID cannot reach cannot issue tokens it accepts. In that case, use [certificate authentication](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_client_certificate).

gitlab.com, Azure Kubernetes Service (AKS) and other platforms with a public issuer work.

## How It Works

```text
1. The platform issues a JWT for the job or the pod
     iss = the platform's issuer URL   sub = who runs (project, branch, service account)
     aud = api://AzureADTokenExchange  exp = a few minutes to the length of the job
2. The provider sends it to Entra ID as a client assertion for the app
     -> an Exchange Online access token, valid about an hour
3. PowerShell connects with Connect-ExchangeOnline -AccessToken
```

The token must carry these claims, and the federated credential must match `iss`, `sub` and `aud` exactly:

| Claim | Value |
|---|---|
| `iss` | The issuer URL declared in the federated credential |
| `sub` | The subject declared in the federated credential |
| `aud` | `api://AzureADTokenExchange` |
| `exp`, `iat` | Expiry and issue time |

## Step 1 — Declare the Federated Credential

In Entra ID, open the app registration, then **Certificates & secrets → Federated credentials → Add credential → Other issuer**, and enter the issuer and the subject. With Microsoft Graph PowerShell:

```powershell
Connect-MgGraph -Scopes "Application.ReadWrite.All"
$app = Get-MgApplication -Filter "appId eq '<client-id>'"
New-MgApplicationFederatedIdentityCredential -ApplicationId $app.Id -BodyParameter @{
    name      = "gitlab-main"
    issuer    = "https://gitlab.com"
    subject   = "project_path:contoso/exchange-config:ref_type:branch:ref:main"
    audiences = @("api://AzureADTokenExchange")
}
```

## Step 2 — Give the Token to the Provider

Use exactly one of the two sources:

| Attribute | Environment variable | Use it when |
|---|---|---|
| `oidc_token` | `EXCHANGE_OIDC_TOKEN` | The platform puts the token in a variable |
| `oidc_token_file_path` | `EXCHANGE_OIDC_TOKEN_FILE_PATH` | The platform writes the token to a file |

The file is read again each time the provider needs a new access token, so a platform that renews the file (Kubernetes) keeps a long run working. A token passed as a value cannot be renewed: if the run outlasts it, the provider fails with a message that says so. Prefer the file when you have the choice.

### GitLab CI (gitlab.com)

```yaml
apply:
  image: mcr.microsoft.com/powershell:latest
  id_tokens:
    EXCHANGE_OIDC_TOKEN:
      aud: api://AzureADTokenExchange
  variables:
    EXCHANGE_AUTH_METHOD: oidc
    EXCHANGE_TENANT_ID: contoso.onmicrosoft.com
    EXCHANGE_CLIENT_ID: $EXCHANGE_CLIENT_ID
  script:
    - pwsh -c "Install-PSResource -Name ExchangeOnlineManagement -Scope CurrentUser -TrustRepository -Quiet"
    - terraform init
    - terraform apply -auto-approve
```

The GitLab subject has the form `project_path:<group>/<project>:ref_type:branch:ref:<branch>`. Restrict the job to protected branches.

GitLab ID tokens live as long as the job's timeout: for a long run, write the token to a file and use `EXCHANGE_OIDC_TOKEN_FILE_PATH`, or keep the job timeout above the run's duration.

### Kubernetes (AKS workload identity)

A pod with workload identity receives a projected token file, renewed by Kubernetes. Point the provider at it:

```bash
export EXCHANGE_AUTH_METHOD=oidc
export EXCHANGE_TENANT_ID=contoso.onmicrosoft.com
export EXCHANGE_CLIENT_ID=<client-id>
export EXCHANGE_OIDC_TOKEN_FILE_PATH="$AZURE_FEDERATED_TOKEN_FILE"
```

The federated credential uses the cluster's OIDC issuer URL and the subject `system:serviceaccount:<namespace>:<service-account>`.

### Jenkins and Other Platforms

Any tool that can write a valid token to a file works: write it, then set `EXCHANGE_OIDC_TOKEN_FILE_PATH`. Delete the file at the end of the job.

```bash
export EXCHANGE_AUTH_METHOD=oidc
export EXCHANGE_OIDC_TOKEN_FILE_PATH="$WORKSPACE/oidc-token.jwt"
terraform apply -auto-approve
rm -f "$EXCHANGE_OIDC_TOKEN_FILE_PATH"
```

## Step 3 — Configure the Provider

```hcl
provider "msexchangeonline" {
  auth_method          = "oidc"
  tenant_id            = "contoso.onmicrosoft.com"
  client_id            = var.client_id
  oidc_token_file_path = "/var/run/secrets/tokens/exchange.jwt"
}
```

## Security Best Practices

- **Narrow subjects**: trust a protected branch or a dedicated service account, never a subject any job can obtain.
- **One app per role**: a read app for plans, a write app for applies (see the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication)).
- **Treat the token as a secret** while it is valid: never print it, delete token files after the job.

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `Missing OIDC token` | Neither `oidc_token` nor `oidc_token_file_path` is set | Set one of them, or its environment variable |
| `Conflicting OIDC settings` | Both are set | Keep one |
| `Invalid oidc_token_file_path` | The file does not exist or cannot be read | Check the path and the permissions of the user running Terraform |
| `AADSTS70021` or `AADSTS700213` | No federated credential matches the token's subject | Decode the token's payload (the middle part, base64) and compare `sub` with the federated credential |
| `AADSTS700211` | The issuer does not match | Compare `iss` with the federated credential, character for character |
| `AADSTS700212` | The audience does not match | Request the token with the audience `api://AzureADTokenExchange` |
| `AADSTS700024` | The token has expired | Use a token file that the platform renews, or a shorter run |
| Entra ID cannot validate the signature | The issuer is not reachable from the internet, or its certificate is not publicly trusted | Use certificate authentication for internal platforms |

More errors are listed in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/troubleshooting).
