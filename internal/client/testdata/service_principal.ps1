# service_principal.ps1 - runs bridge/scripts/service_principal.ps1 against fake
# service principal cmdlets (run by rbac_scripts_test.go): the service
# principal is found by its app ID among the others.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:Sps = [System.Collections.Generic.List[object]]::new()
$global:Sps.Add([pscustomobject]@{ Identity = "sp-other"; AppId = "99999999-0000-0000-0000-000000000009"; ObjectId = "x"; DisplayName = "Other" })
function global:Get-ServicePrincipal { param($Identity) $global:Sps }
function global:New-ServicePrincipal {
    param($AppId, $ObjectId, $DisplayName)
    $global:Calls.Add("New AppId=$AppId,DisplayName=$DisplayName,ObjectId=$ObjectId")
    $global:Sps.Add([pscustomobject]@{ Identity = "sp-$ObjectId"; AppId = $AppId; ObjectId = $ObjectId; DisplayName = $DisplayName })
}
function global:Set-ServicePrincipal { param($Identity, $DisplayName) $global:Calls.Add("Set $Identity DisplayName=$DisplayName"); ($global:Sps | Where-Object Identity -eq $Identity).DisplayName = $DisplayName }
function global:Remove-ServicePrincipal { param($Identity) $global:Calls.Add("Remove $Identity") }

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject $Object)))
}
$app = "11111111-2222-3333-4444-555555555555"
"missing: [" + (& $Script -Action Get -Identity $app) + "]"
"create: " + (& $Script -Action Create -Identity $app -Values (ConvertTo-B64 @{ ObjectId = "obj-1"; DisplayName = "TF-CI-App" }))
"update: " + (& $Script -Action Update -Identity $app -Values (ConvertTo-B64 @{ DisplayName = "TF-CI-App-2" }))
"delete: " + (& $Script -Action Delete -Identity $app)
"calls: " + ($global:Calls -join "; ")
