---
page_title: "Troubleshooting"
subcategory: "Guides"
---

# Troubleshooting

The provider runs PowerShell scripts that call the ExchangeOnlineManagement module. When something fails, find the layer first: installing the provider, starting PowerShell, connecting to Exchange Online, permissions and licenses, or the Exchange operation itself. The sections below follow that order.

## Read the Error

When an Exchange cmdlet fails, the error ends with the PowerShell message, unchanged:

```text
PowerShell script transport_rule.ps1 failed: <message of the failing cmdlet>
```

The script name tells which resource failed: each resource has its own script (`transport_rule.ps1`, `remote_domain.ps1`...), the generic ones are `exo_object.ps1` and `exo_rule.ps1`, and `list.ps1` lists objects for `terraform query`. The message after `failed:` comes from Exchange or from PowerShell: search for it as is. For a few known errors, the provider adds a line of advice after the message.

## Provider Installation

**`no available releases match the given constraints`, although the version exists on the registry.** A local copy of the provider hides the registry. Terraform looks for providers in local plugin directories first (`~/.terraform.d/plugins`, `~/.local/share/terraform/plugins`, `/usr/local/share/terraform/plugins`, `/usr/share/terraform/plugins`, on macOS `~/Library/Application Support/io.terraform/plugins`, on Windows `%APPDATA%\terraform.d\plugins` and `%APPDATA%\HashiCorp\Terraform\plugins`). When it finds a provider there, it installs that provider only from there. Remove the `workplace-as-code` folder from these directories, or declare an explicit `provider_installation` block in the CLI configuration.

**`Provider development overrides are in effect`.** The CLI configuration (`~/.terraformrc`, or the file named by `TF_CLI_CONFIG_FILE`) has a `dev_overrides` entry for the provider: Terraform runs a local build instead of the published version. Remove the entry to use the registry.

**Checksum errors on another machine** (`does not match any of the checksums recorded in the dependency lock file`). The lock file only lists the platforms it was created on. Record every platform that runs Terraform:

```bash
terraform providers lock -platform=linux_amd64 -platform=darwin_arm64 -platform=windows_amd64
```

**No Internet access.** The registry sends Terraform to GitHub to download the release files. Without access to both, serve the provider from a network mirror (`terraform providers mirror`, or a repository manager such as Artifactory) and point the CLI configuration at it.

## PowerShell and the ExchangeOnlineManagement Module

**`the provider needs PowerShell 7+ (pwsh)`.** Install [PowerShell 7](https://aka.ms/install-powershell). `pwsh` must be on the `PATH` of the account that runs Terraform: check it in the CI job or service, not only in your own shell.

**`Failed to connect to Exchange Online: The term 'Connect-ExchangeOnline' is not recognized`.** The module is not installed for the account that runs Terraform. The provider starts PowerShell with `-NoProfile`, so a module loaded from a profile is not found either. Install version 3 or later for that account:

```powershell
Install-PSResource -Name ExchangeOnlineManagement -Scope CurrentUser -TrustRepository
```

Check which versions PowerShell finds, and where:

```powershell
Get-Module -ListAvailable ExchangeOnlineManagement | Select-Object Name, Version, ModuleBase
```

When several versions are installed, PowerShell loads the newest one. The tested versions are listed on the [provider page](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs).

**Linux and macOS: `Access to the path '.../powershell/7/Scripts' is denied` when the module loads.** `Install-PSResource` also installs the dependencies declared by ExchangeOnlineManagement, PowerShellGet and PackageManagement, in their oldest accepted version, 1.1.0.0. That old PowerShellGet tries to create a folder in the PowerShell installation directory, which only an administrator can write. PowerShell 7 already ships newer versions of both modules: remove the copies, PowerShellGet first since it depends on PackageManagement:

```powershell
Uninstall-PSResource -Name PowerShellGet -Version 1.1.0.0 -Scope CurrentUser
Uninstall-PSResource -Name PackageManagement -Version 1.1.0.0 -Scope CurrentUser
```

**`PowerShell bridge host failed to start`, `did not start within`, or `exited while running`.** The provider keeps a few PowerShell processes open and sends them requests. To get the error of a single call, closer to the failing cmdlet, run once with one process per call:

```bash
EXCHANGE_BRIDGE_MODE=oneshot terraform plan
```

**PowerShell crashes when many processes start at once.** PowerShell keeps a startup cache that a burst of parallel starts can corrupt; every later start then fails. Delete it, it is rebuilt automatically:

- Linux and macOS: `~/.cache/powershell/StartupProfileData-NonInteractive`
- Windows: `%LOCALAPPDATA%\Microsoft\PowerShell\StartupProfileData-NonInteractive`

## Authentication

**`tenant_id must be the tenant's .onmicrosoft.com domain`.** `Connect-ExchangeOnline` does not accept a tenant GUID: use the primary `.onmicrosoft.com` domain, for example `contoso.onmicrosoft.com`.

Other connection errors start with `Failed to connect to Exchange Online:`, followed by the Entra ID message. The `AADSTS` code tells what is wrong:

| Code | Meaning | Fix |
|---|---|---|
| `AADSTS700016` | No application with this client ID in the tenant | Check `client_id`, and that the app registration is in the tenant named by `tenant_id` |
| `AADSTS90002` | Tenant not found | Check the spelling of the `.onmicrosoft.com` domain |
| `AADSTS700027` | The certificate is not registered on the application, or has expired | Upload the `.cer` of the same certificate as the `.pfx`, and check its end date in **Certificates & secrets** |
| `AADSTS70021`, `AADSTS700213` | OIDC: no federated credential matches the token's subject | Compare the subject of the federated credential with the job's context (repository, environment, branch) |
| `AADSTS700211`, `AADSTS700212` | OIDC: the issuer or the audience does not match | Check the issuer URL and the audience `api://AzureADTokenExchange` in the federated credential |
| `AADSTS700024` | OIDC: the token has expired | Use a token file renewed by the platform instead of a fixed `oidc_token` |

**The certificate file cannot be read.** With a `.pfx` file, a wrong password fails before any connection, with a message such as `The certificate data cannot be read with the provided password`. Check `certificate_password`, and give `certificate_path` as an absolute path: CI jobs and services often run in another working directory.

**macOS: `This platform does not support loading with EphemeralKeySet`.** Fixed in version 0.1.1: upgrade the provider.

**Configuration errors at plan time.** `Missing certificate`, `Conflicting authentication settings`, `Conflicting certificate settings`, `Missing OIDC token` and `Missing GitHub Actions OIDC variables` come from the provider itself, before any connection: the message says which attribute or environment variable to change. A leftover `EXCHANGE_CERTIFICATE_*` variable in the CI environment is a common cause of the conflicts.

**GitHub Actions: `requesting the OIDC token from GitHub Actions failed`.** The job needs `permissions: id-token: write`. See [OIDC with GitHub Actions](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc_github).

The steps to create the app registration and the role group are in the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication); each method has its own guide: [certificate](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_client_certificate), [OIDC with GitHub Actions](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc_github), [OIDC with any platform](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_oidc).

## Permissions and Licenses

**`The term 'New-TransportRule' is not recognized` (or any other Exchange cmdlet).** In Exchange Online, a session only contains the cmdlets its roles allow: a missing role looks like a missing cmdlet. Check, in this order:

1. **Admin consent** is granted for `Exchange.ManageAsApp` (**API permissions**, status column).
2. The app is a member of the provider's role group, or has the Exchange Administrator role. A new assignment can take up to 30 minutes to apply.
3. After upgrading the provider, run `tools/new_role_group.ps1` again: new resources can need roles the group does not have yet.
4. `terraform apply` does not use the read-only app: an app with View-Only Organization Management can plan, but not change the organization.

**Safe Links or Safe Attachments cmdlets are not recognized.** They only exist in tenants licensed for Microsoft Defender for Office 365 (Plan 1 or 2): the provider adds this advice to the error. The license each resource needs is listed in the [Resources and Availability guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/availability).

**The first custom policy fails and the advice mentions `Enable-OrganizationCustomization`.** A new tenant must be customized once: an Exchange administrator runs `Enable-OrganizationCustomization`, which can take up to an hour to apply.

**Objects deleted outside Terraform stay in the state, or `terraform query` fails on an empty list, on a machine whose language is not English.** Fixed in version 0.1.1: the provider now asks Exchange Online for English messages, which it needs to recognize a missing object. Upgrade the provider.

## Timeouts and Throttling

**`PowerShell script <name> timed out after 10m (set EXCHANGE_SCRIPT_TIMEOUT to raise the limit)`.** One call took longer than the limit. The first call of a run also loads the module and connects. Raise the limit, for example `EXCHANGE_SCRIPT_TIMEOUT=20m`.

**Many objects, slow or throttled runs.** Exchange Online limits the number of requests per app. `EXCHANGE_MAX_CONCURRENCY` (default `4`) limits the number of PowerShell processes, and so the number of simultaneous requests. Lower it, for example to `2`, if Exchange throttles the requests.

## Debug Logging

```bash
TF_LOG_PROVIDER=DEBUG TF_LOG_PATH=terraform.log terraform plan
```

`TF_LOG_PROVIDER` limits the log to the providers; `TF_LOG=DEBUG` adds Terraform itself. The log shows which resource and which operation failed, in order. The PowerShell error is already in the error message.

The log can contain values from your configuration: read it before sharing it. The certificate password is never written to the log or put on a command line: the provider passes it to PowerShell through the process environment.

## Run a Bridge Script by Hand

Running the script that failed, outside Terraform, shows the full PowerShell error and lets you try a fix.

1. Find the scripts. The provider writes them, at each start, to a folder named after their content hash:
   - Linux: `~/.cache/terraform-provider-msexchangeonline/<hash>/`
   - macOS: `~/Library/Caches/terraform-provider-msexchangeonline/<hash>/`
   - Windows: `%LOCALAPPDATA%\terraform-provider-msexchangeonline\<hash>\`

   They are also in the repository, under `bridge/scripts`, at the tag of your provider version.

2. In `pwsh`, from that folder, connect and call the script with the same parameters as the provider:

   ```powershell
   $auth = @{
       TenantId            = "contoso.onmicrosoft.com"
       ClientId            = "00000000-0000-0000-0000-000000000000"
       CertificatePath     = "/secrets/terraform-msexchangeonline.pfx"
       CertificatePassword = Read-Host "PFX password" -MaskInput
   }

   # Lists the remote domains, as terraform query does
   ./list.ps1 @auth -Noun RemoteDomain

   # Reads one transport rule, as terraform plan does
   ./transport_rule.ps1 @auth -Action Get -Identity "Block executable attachments"
   ```

   `-Noun` is the noun of the Exchange cmdlets of the resource, listed in its documentation (`Get-RemoteDomain` → `RemoteDomain`). A read prints one line of JSON; an empty output means that the object does not exist.

3. To test a change to a script, copy the folder, edit the copy and point the provider at it with `EXCHANGE_BRIDGE_DIR=/path/to/the/copy`. This is for script development only: remove the variable afterwards.

## Report a Problem

If none of this helps, [open an issue](https://github.com/workplace-as-code/terraform-provider-msexchangeonline/issues/new/choose) with the provider, Terraform, PowerShell and ExchangeOnlineManagement module versions, the resource configuration and the full error message. Remove tenant names, IDs and certificate details first.
