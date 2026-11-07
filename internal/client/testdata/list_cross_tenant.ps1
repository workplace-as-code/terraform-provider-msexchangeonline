# list_cross_tenant.ps1 - runs bridge/scripts/list.ps1 for the cross-tenant
# partners against a fake Microsoft Graph (run by graph_scripts_test.go): only
# partners with an Exchange capability are listed, across result pages, and a
# missing Policy.Read.All is named.
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:BridgeGraph = @{ Token = "gt-1"; Environment = "global" }
$global:Forbidden = $false
function Response([int]$Status, $Object) {
    [pscustomobject]@{ StatusCode = $Status; Content = (ConvertTo-Json -InputObject $Object -Depth 10 -Compress); Headers = @{} }
}
function global:Invoke-WebRequest {
    param($Method, $Uri, $Headers, $Body, $ContentType, [switch]$SkipHttpErrorCheck)
    $u = [string]$Uri
    if ($global:Forbidden) { return Response 403 @{ error = @{ code = "Authorization_RequestDenied"; message = "Insufficient privileges to complete the operation." } } }
    if ($u -like "*/partners[?]$*") {
        return Response 200 @{ value = @(@{ tenantId = "AAAAAAAA-0000-0000-0000-000000000001" }); "@odata.nextLink" = "https://graph.microsoft.com/v1.0/policies/crossTenantAccessPolicy/partners?page=2" }
    }
    if ($u -like "*/partners[?]page=2") { return Response 200 @{ value = @(@{ tenantId = "bbbbbbbb-0000-0000-0000-000000000002" }) } }
    if ($u -like "*aaaaaaaa-0000-0000-0000-000000000001/m365Capabilities") { return Response 200 @{ value = @(@{ name = "crossTenantMailTipsAll" }) } }
    if ($u -like "*bbbbbbbb-0000-0000-0000-000000000002/m365Capabilities") { return Response 200 @{ value = @(@{ name = "crossTenantOpenProfileCard" }) } }
    Response 404 @{ error = @{ code = "NotFound"; message = "unexpected $u" } }
}
"partners: " + (& $Script -Noun CrossTenantPartner 2>&1)
$global:Forbidden = $true
& $Script -Noun CrossTenantPartner
