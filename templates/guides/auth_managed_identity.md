---
page_title: "Managed Identity"
subcategory: "Guides"
---

# Managed Identity

With `auth_method = "managed_identity"`, the provider connects as the **managed identity** of the machine running Terraform: an Azure VM, a CI runner hosted in Azure, or a server outside Azure connected to **Azure Arc**. Nothing is stored and nothing expires: Azure creates the identity, keeps its credential and rotates it.

The machine running Terraform needs the ExchangeOnlineManagement module **3.1.0 or later**.

## The Limit to Check First

A managed identity only exists on an Azure resource or an Azure Arc-enabled server. A workstation or an on-premises CI runner that is not connected to Arc has none: use [certificate authentication](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/auth_client_certificate) there.

| Where Terraform runs | Identities available |
|---|---|
| Azure VM, VM scale set | System-assigned and user-assigned |
| Azure Container Instances, App Service, Azure Functions, other Azure compute | System-assigned and user-assigned |
| Azure Arc-enabled server (on-premises or another cloud) | System-assigned |
| Workstation, on-premises runner without Arc | None: use a certificate |

The two kinds of identity:

- **System-assigned**: created with the resource and deleted with it. One per resource. Leave `client_id` empty.
- **User-assigned**: a separate Azure resource that you attach to one or more machines, so several runners can share one identity and its Exchange permissions. Set `client_id` to the identity's **client ID**.

## How It Works

```text
1. The provider asks the local identity endpoint for an Exchange Online token
     Azure VM:  http://169.254.169.254 (reachable only from the VM itself)
     Azure Arc: http://localhost:40342 (the Connected Machine agent)
2. Azure returns an access token for the managed identity, valid up to 24 hours
3. PowerShell connects with Connect-ExchangeOnline -AccessToken
```

The provider asks for a new token before the current one expires, so a long run keeps working. `AZURE_CLIENT_ID` and the other `AZURE_*` variables are ignored: only `client_id` selects a user-assigned identity.

## Step 1 — Get the Identity

You need two values of the identity, shown in **Entra admin center → Enterprise applications**, with the **Application type** filter set to **Managed Identities**:

| Value | Where it is used |
|---|---|
| **Application ID** (the client ID) | `client_id` of the provider, for a user-assigned identity; `-AppId` of the Exchange role group script |
| **Object ID** (of the service principal) | Granting `Exchange.ManageAsApp` and the Entra or Exchange roles |

### System-assigned Identity of an Azure VM

In the Azure portal, open the VM, then **Security → Identity → System assigned → Status: On → Save**. With the Azure CLI:

```bash
az vm identity assign --resource-group <resource-group> --name <vm>
az vm identity show --resource-group <resource-group> --name <vm> --query principalId --output tsv
```

The second command prints the Object ID. The identity appears in Entra ID with the name of the VM.

### User-assigned Identity

Create the identity, then attach it to the VM (or to the other resources that run Terraform):

```bash
az identity create --resource-group <resource-group> --name terraform-msexchangeonline
az vm identity assign --resource-group <resource-group> --name <vm> --identities terraform-msexchangeonline
az identity show --resource-group <resource-group> --name terraform-msexchangeonline --query "{clientId: clientId, objectId: principalId}"
```

`--identities` accepts a name when the identity is in the VM's resource group. For an identity in another resource group, give its resource ID (`az identity show ... --query id`).

### Azure Arc-enabled Server

Connecting the server to Azure Arc creates its system-assigned identity, named after the server. Azure Arc protects the token with a file that only privileged accounts can read: add the account that runs Terraform to the group that may read it, and never make that account an administrator for this purpose.

- **Linux**: the `himds` group.

  ```bash
  sudo usermod --append --groups himds <account>
  ```

- **Windows**: the local **Hybrid agent extension applications** group.

  ```powershell
  Add-LocalGroupMember -Group "Hybrid agent extension applications" -Member "<DOMAIN>\<account>"
  ```

The new membership applies to new sessions: restart the CI runner service, or log off and back on. Azure Arc may only offer the system-assigned identity: leave `client_id` empty.

## Step 2 — Grant Exchange.ManageAsApp

The Entra admin center cannot grant an application permission to a managed identity: use Microsoft Graph PowerShell, as a Global Administrator or Privileged Role Administrator.

```powershell
Connect-MgGraph -Scopes "AppRoleAssignment.ReadWrite.All", "Application.Read.All"

# The Object ID of the managed identity (Step 1), not its client ID
$identity = "<object-id>"

# Office 365 Exchange Online and its Exchange.ManageAsApp permission, the same in every tenant
$exchange = Get-MgServicePrincipal -Filter "appId eq '00000002-0000-0ff1-ce00-000000000000'"
New-MgServicePrincipalAppRoleAssignment -ServicePrincipalId $identity `
    -PrincipalId $identity `
    -ResourceId $exchange.Id `
    -AppRoleId "dc50a0fb-09a3-484d-be87-e023b12c6440"
```

Or with the Azure CLI, for example in Azure Cloud Shell, which is already signed in: useful when the tenant blocks the device code sign-in of `Connect-MgGraph`.

```bash
identity="<object-id>"
exchange=$(az ad sp show --id 00000002-0000-0ff1-ce00-000000000000 --query id --output tsv)
az rest --method POST \
  --uri "https://graph.microsoft.com/v1.0/servicePrincipals/$identity/appRoleAssignments" \
  --body "{\"principalId\": \"$identity\", \"resourceId\": \"$exchange\", \"appRoleId\": \"dc50a0fb-09a3-484d-be87-e023b12c6440\"}"
```

`Exchange.ManageAsApp` grants no rights by itself: it only lets the identity open an Exchange Online PowerShell session. Check the result in **Enterprise applications → the identity → Permissions**.

## Step 3 — Grant Exchange Permissions

Follow step 2 of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/authentication) with the identity's values:

- **Least-privilege role group (recommended)**: run `tools/new_role_group.ps1` with `-AppId` set to the identity's client ID and `-ServicePrincipalObjectId` set to its Object ID.
- **Exchange Administrator**: in **Roles & admins → Exchange Administrator → Add assignments**, search for the identity by name (the VM, the server, or the user-assigned identity). With the Azure CLI (`29232cdf-9323-42fd-ade2-1d097af3e4de` is the Exchange Administrator role in every tenant):

  ```bash
  az rest --method POST \
    --uri "https://graph.microsoft.com/v1.0/roleManagement/directory/roleAssignments" \
    --body '{"principalId": "<object-id>", "roleDefinitionId": "29232cdf-9323-42fd-ade2-1d097af3e4de", "directoryScopeId": "/"}'
  ```

For `terraform plan` and drift detection, a separate identity with the **View-Only Organization Management** role group is enough: see the read and write split in the Authentication guide.

Role assignments can take up to 30 minutes to apply.

**Grant the permissions before Terraform first runs on the machine.** Azure caches the tokens of a managed identity for about 24 hours and cannot be forced to renew them: a token obtained before the roles applied keeps lacking them for up to a day. After granting the roles, wait about 10 minutes, then check the token from the machine itself. This prints the token's claims, never the token:

```bash
curl -s -H Metadata:true "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://outlook.office365.com" \
  | jq -r .access_token | cut -d. -f2 \
  | python3 -c "import sys,json,base64; p=sys.stdin.read().strip(); p+='='*(-len(p)%4); d=json.loads(base64.urlsafe_b64decode(p)); print(json.dumps({k: d.get(k) for k in ('appid','roles','wids')}, indent=1))"
```

`roles` must contain `Exchange.ManageAsApp`, and `wids` the ID of the Entra role you assigned (Exchange Administrator: `29232cdf-9323-42fd-ade2-1d097af3e4de`). An Exchange role group does not appear in the token. For a user-assigned identity, add `&client_id=<client-id>` to the URL.

## Step 4 — Configure the Provider

System-assigned identity (Azure VM or Azure Arc):

```hcl
provider "msexchangeonline" {
  auth_method = "managed_identity"
  tenant_id   = "contoso.onmicrosoft.com"
}
```

User-assigned identity:

```hcl
provider "msexchangeonline" {
  auth_method = "managed_identity"
  tenant_id   = "contoso.onmicrosoft.com"
  client_id   = "<client-id-of-the-identity>"
}
```

Or with environment variables, for example in a CI job:

```bash
export EXCHANGE_AUTH_METHOD=managed_identity
export EXCHANGE_TENANT_ID=contoso.onmicrosoft.com
export EXCHANGE_CLIENT_ID=<client-id-of-the-identity>   # user-assigned identity only
terraform plan
```

For the system-assigned identity, make sure `EXCHANGE_CLIENT_ID` is not set, for example left over from a certificate configuration on the same runner: the provider would ask for a user-assigned identity with that ID.

`tenant_id` stays required: it is the tenant's `.onmicrosoft.com` domain that `Connect-ExchangeOnline` connects to. Managed identities are supported in the `global` and `gcc` environments; use a certificate in the other clouds.

## Security Best Practices

- **Everything on the machine can use its identity**: any process, job or user that runs there obtains a token. Run Terraform on machines dedicated to it, and do not give the identity more Exchange roles than the provider needs.
- **Shared runners**: on a runner used by several projects, every job gets the same identity. Prefer a runner reserved for the Exchange configuration.
- **On Azure Arc**, keep the `himds` group or the Hybrid agent extension applications group limited to the accounts that run Terraform.
- **One identity per role**: a read identity for plans, a write identity for applies.

## Troubleshooting

Every error from the identity endpoint starts with `obtaining an Exchange Online access token with the managed identity failed`, followed by the reason given by Azure.

| Error or symptom | Cause | Fix |
|---|---|---|
| The error comes after less than a minute: `Get "http://169.254.169.254/...": context deadline exceeded` | Terraform does not run on an Azure resource with a managed identity | Run it on an Azure resource or an Arc-enabled server, or use a certificate |
| `Identity not found`, or an HTTP 400 that names the client ID | The user-assigned identity is not attached to this resource, `client_id` holds another ID, or a leftover `EXCHANGE_CLIENT_ID` (an app registration's ID) is set | Attach the identity to the resource and set its **client ID** (not its Object ID or resource ID), or unset `EXCHANGE_CLIENT_ID` to use the system-assigned identity |
| Azure Arc: an error about reading a file under `/var/opt/azcmagent/tokens` or `C:\ProgramData\AzureConnectedMachineAgent\Tokens` | The account running Terraform is not in the `himds` group (Linux) or Hybrid agent extension applications group (Windows) | Add it, then restart the runner service or open a new session |
| Azure VM: an HTTP 410 from `169.254.169.254` | The instance metadata service is being updated, which Microsoft says can take up to 70 seconds | Run Terraform again |
| Azure Arc with `client_id` set: `expected a 401 response, received 404` | The Arc agent only offers the system-assigned identity | Remove `client_id` (and `EXCHANGE_CLIENT_ID`) |
| `Invalid client_id ... is not a GUID` at plan time | `client_id` holds a name or a resource ID | Use the identity's client ID |
| `Unknown client_id` at plan time | `client_id` comes from a resource that is not created yet, for example a user-assigned identity created in the same configuration | Create the identity and attach it to the runner first, then pass its client ID as a variable or read it with a data source |
| `Conflicting authentication settings` at plan time | A certificate or an OIDC token is also set, often a leftover `EXCHANGE_CERTIFICATE_*` or `EXCHANGE_OIDC_*` variable | Remove it, or choose another `auth_method` |
| Connected, but Exchange answers `The role assigned to application ... isn't supported in this scenario` or cmdlets are missing | `Exchange.ManageAsApp` or the Exchange roles are missing, or were granted after the machine obtained its first token | Check steps 2 and 3 and the token's claims (end of step 3). A token obtained before the roles applied stays cached for up to 24 hours: wait, or use a new user-assigned identity granted before its first use |

More errors are listed in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/troubleshooting).

## Cleanup

Deleting a VM or disconnecting an Arc server deletes its system-assigned identity, and with it the `Exchange.ManageAsApp` permission and the Entra role assignments. A user-assigned identity stays until you delete it. In both cases, an identity added to an Exchange role group leaves an Exchange service principal behind: remove it with `Remove-ServicePrincipal -Identity <object-id>` in Exchange Online PowerShell.
