---
page_title: "OIDC with GitHub Actions"
subcategory: "Guides"
---

# OIDC with GitHub Actions

With `auth_method = "oidc_github"`, a GitHub Actions job connects to Exchange Online **without any stored credential**: GitHub issues a short-lived OIDC token for the job, and Entra ID exchanges it for an Exchange Online access token, if the token comes from the repository and environment the app registration trusts.

Before you start, create the app registration and grant its Exchange permissions: steps 1 and 2 of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication). The app needs no certificate for this method.

## How It Works

```text
GitHub Actions job (permissions: id-token: write)
  1. The provider asks GitHub for the job's OIDC token
       GET $ACTIONS_ID_TOKEN_REQUEST_URL&audience=api://AzureADTokenExchange
       -> a JWT signed by token.actions.githubusercontent.com, valid a few minutes,
          whose subject names the repository and the environment (or branch)
  2. The provider sends it to Entra ID as a client assertion for the app
       Entra ID checks the signature with GitHub's public keys, the issuer,
       the audience and the subject against the app's federated credentials
       -> an Exchange Online access token, valid about an hour
  3. PowerShell connects with Connect-ExchangeOnline -AccessToken
       A few minutes before the token expires, the provider gets a new one
       and starts a new PowerShell process with it
```

The machine running Terraform needs the ExchangeOnlineManagement module **3.1.0 or later** (the version that added `-AccessToken`).

## Step 1 — Declare the Federated Credential

The federated credential tells Entra ID which GitHub tokens the app accepts. Trust a **protected environment**: only the branches you allow can use it, and each run can wait for a reviewer's approval.

1. In GitHub, create the environment: **Settings → Environments → New environment**, for example `production`. Set **Deployment branches** to your protected branch and add **Required reviewers**.
2. In Entra ID, open the app registration, then **Certificates & secrets → Federated credentials → Add credential**:
   - **Federated credential scenario**: GitHub Actions deploying Azure resources
   - **Organization** and **Repository**: your repository
   - **Entity type**: Environment, with the environment name
   - **Name**: for example `github-production`

The portal fills in the issuer `https://token.actions.githubusercontent.com`, the subject and the audience `api://AzureADTokenExchange`. When it asks for the **Organization ID** and the **Repository ID**, your repository uses immutable subjects (see below): get both IDs with `gh api repos/<organization>/<repository> --jq '"\(.owner.id) \(.id)"'`.

With Microsoft Graph PowerShell:

```powershell
Connect-MgGraph -Scopes "Application.ReadWrite.All"
$app = Get-MgApplication -Filter "appId eq '<client-id>'"
New-MgApplicationFederatedIdentityCredential -ApplicationId $app.Id -BodyParameter @{
    name      = "github-production"
    issuer    = "https://token.actions.githubusercontent.com"
    subject   = "repo:contoso/exchange-config:environment:production"
    audiences = @("api://AzureADTokenExchange")
}
```

With the Azure CLI:

```bash
az ad app federated-credential create --id "<client-id>" --parameters '{
  "name": "github-production",
  "issuer": "https://token.actions.githubusercontent.com",
  "subject": "repo:contoso/exchange-config:environment:production",
  "audiences": ["api://AzureADTokenExchange"]
}'
```

### Subject Formats

Entra ID compares the subject **exactly**: no wildcard. Declare one federated credential per context (an app accepts up to 20).

GitHub writes the subject in one of two forms. Check which one your repository uses:

```bash
gh api repos/<org>/<repo>/actions/oidc/customization/sub
```

- `"use_immutable_subject": true`: the subject starts with the organization and repository **IDs** as well as their names, `repo:<org>@<org-id>/<repo>@<repo-id>`. The answer gives this prefix in `sub_claim_prefix`. The IDs do not change when the organization or the repository is renamed.
- Otherwise: the subject starts with `repo:<org>/<repo>`.

Then add the context:

| The job runs | Subject (with `repo:<org>/<repo>`, or the immutable prefix) |
|---|---|
| In an environment | `repo:<org>/<repo>:environment:<environment>` |
| On a branch, without environment | `repo:<org>/<repo>:ref:refs/heads/<branch>` |
| On a tag, without environment | `repo:<org>/<repo>:ref:refs/tags/<tag>` |
| For a pull request, without environment | `repo:<org>/<repo>:pull_request` |

For example, with immutable subjects: `repo:contoso@123456/exchange-config@987654:environment:production`.

A job that declares an environment always gets the environment subject, whatever its branch. An organization or a repository can also customize the claims of the subject: the same API shows it.

## Step 2 — Configure the Workflow

The job needs `permissions: id-token: write`: without it GitHub does not let the job request a token.

```yaml
name: Terraform

on:
  push:
    branches: [main]

permissions:
  contents: read
  id-token: write

jobs:
  apply:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v7
      - uses: hashicorp/setup-terraform@v4

      - name: Install the ExchangeOnlineManagement module
        shell: pwsh
        run: Install-PSResource -Name ExchangeOnlineManagement -Scope CurrentUser -TrustRepository -Quiet

      - name: Terraform apply
        env:
          EXCHANGE_AUTH_METHOD: oidc_github
          EXCHANGE_TENANT_ID: contoso.onmicrosoft.com
          EXCHANGE_CLIENT_ID: ${{ vars.EXCHANGE_CLIENT_ID }}
        run: |
          terraform init
          terraform apply -auto-approve
```

Pin the actions to a commit SHA in production workflows.

## Step 3 — Configure the Provider

With the environment variables of the workflow above, the provider block can stay empty. In HCL:

```hcl
provider "msexchangeonline" {
  auth_method = "oidc_github"
  tenant_id   = "contoso.onmicrosoft.com"
  client_id   = var.client_id
}
```

Do not set a certificate at the same time: the provider refuses two methods at once.

## Security Best Practices

- **Trust an environment, not a branch**, and protect that environment with deployment branches and required reviewers. A pull request from a fork never gets its token.
- **One app per role**: a read app trusted by the pull request or plan environment, a write app trusted only by the deployment environment (see the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication)).
- **Least privilege**: give the write app the provider's role group, not Exchange Administrator.
- **Monitor** the app's sign-ins in Entra ID: each run appears as a service principal sign-in with a federated credential.

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `Missing GitHub Actions OIDC variables` | The job lacks `permissions: id-token: write`, or Terraform does not run in GitHub Actions | Add the permission to the workflow or the job |
| `requesting the OIDC token from GitHub Actions failed (HTTP 403)` | The job may not request a token | Same: check `id-token: write`, also in reusable workflows |
| `AADSTS70021` or `AADSTS700213` | No federated credential matches the token's subject | Compare the subject with the table above: environment name, branch, organization and repository must match exactly, with the IDs if the repository uses immutable subjects |
| `AADSTS700211` | The issuer does not match | Use `https://token.actions.githubusercontent.com`, without a trailing slash |
| `AADSTS700212` | The audience does not match | Use `api://AzureADTokenExchange` in the federated credential |
| `AADSTS700016` | No application with this client ID in the tenant | Check `client_id` and `tenant_id` |
| The connection works, then cmdlets fail with access denied | The app has no Exchange role | Run `tools/new_role_group.ps1` (step 2 of the Authentication guide); roles take up to 30 minutes to apply |

More errors are listed in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/troubleshooting).
