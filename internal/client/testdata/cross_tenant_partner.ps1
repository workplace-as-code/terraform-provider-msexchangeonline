# cross_tenant_partner.ps1 - runs bridge/scripts/cross_tenant_partner.ps1
# against a fake Microsoft Graph (run by graph_scripts_test.go): only the
# Exchange capabilities of the partner are read and changed, an "All" scope
# uses the resource type Graph stores, and a missing Entra role is an error.
param([string]$Script)
$ErrorActionPreference = "Stop"

function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
$global:BridgeGraph = @{ Token = "gt-1"; Environment = "global" }
function global:Start-Sleep { param([int]$Seconds) }

$partner = "11111111-2222-3333-4444-555555555555"
$global:Caps = [ordered]@{}
$global:RoleMissing = $false
$global:ListHides = $false
$global:FailPost = ""
$global:Calls = [System.Collections.Generic.List[string]]::new()

function Cap([string]$Name, [bool]$Allowed, [object[]]$Included, [object[]]$Excluded = @()) {
    [pscustomobject]@{ "@odata.type" = "#microsoft.graph.$Name"; name = $Name
        inboundAccess = [pscustomobject]@{ isAllowed = $Allowed
            resourceScopes = [pscustomobject]@{ included = @($Included); excluded = @($Excluded) } } }
}
function Scope([string]$Id, [string]$Type) { [pscustomobject]@{ resourceId = $Id; resourceType = $Type } }
function Response([int]$Status, $Object = $null) {
    $content = if ($null -ne $Object) { ConvertTo-Json -InputObject $Object -Depth 10 -Compress } else { "" }
    [pscustomobject]@{ StatusCode = $Status; Content = $content; Headers = @{} }
}
function global:Invoke-WebRequest {
    param($Method, $Uri, $Headers, $Body, $ContentType, [switch]$SkipHttpErrorCheck)
    $path = ([uri]$Uri).AbsolutePath
    $name = if ($path -match '/m365Capabilities/([^/]+)$') { $Matches[1] } else { "" }
    $global:Calls.Add("$Method $(if ($name) { $name } else { 'list' })$(if ($Body) { ' ' + $Body })")
    $denied = @{ error = @{ code = "forbidden"; message = "Your administrative role is not allowed to access the cross-tenant capability '$name'." } }
    if ($global:RoleMissing -and ($name -or $Method -ne "GET")) { return Response 403 $denied }
    switch ($Method) {
        "GET" {
            if (-not $name) { return Response 200 @{ value = @(if (-not $global:RoleMissing -and -not $global:ListHides) { $global:Caps.Values }) } }
            if ($global:Caps.Contains($name)) { return Response 200 $global:Caps[$name] }
            return Response 404 @{ error = @{ code = "NotFound"; message = "Capability $name not found." } }
        }
        "POST" {
            $c = $Body | ConvertFrom-Json
            $n = $c.'@odata.type' -replace '^#microsoft\.graph\.', ''
            if ($n -eq $global:FailPost) { return Response 400 @{ error = @{ code = "BadRequest"; message = "Invalid scope for $n." } } }
            $c | Add-Member -NotePropertyName name -NotePropertyValue $n -Force
            $global:Caps[$n] = $c
            return Response 201 $c
        }
        "PATCH" { $c = $Body | ConvertFrom-Json; $c | Add-Member -NotePropertyName name -NotePropertyValue $name -Force; $global:Caps[$name] = $c; return Response 204 }
        "DELETE" {
            if (-not $global:Caps.Contains($name)) { return Response 404 @{ error = @{ code = "NotFound"; message = "Capability $name not found." } } }
            $global:Caps.Remove($name); return Response 204
        }
    }
}

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -Depth 6 -InputObject $Object)))
}
function Desired([string]$Name, [bool]$Allowed, [string[]]$IncludedGroups = @(), [string[]]$ExcludedUsers = @()) {
    [ordered]@{ name = $Name; allowed = $Allowed; includedGroups = @($IncludedGroups); excludedGroups = @()
        includedUsers = @(); excludedUsers = @($ExcludedUsers) }
}
function Invoke-Step($Label, $Action, $Desired = $null, [string[]]$Known = @()) {
    $global:Calls.Clear()
    $params = @{ Action = $Action; PartnerTenantId = $partner; Known = (ConvertTo-B64 @($Known)) }
    if ($null -ne $Desired) { $params.Capabilities = ConvertTo-B64 @($Desired) }
    $out = & $Script @params 2>&1
    "${Label}: $out"
    "$Label calls: $($global:Calls -join '; ')"
}

# A profile card capability (not Exchange) and one Exchange capability.
$global:Caps["crossTenantOpenProfileCard"] = Cap crossTenantOpenProfileCard $true @(Scope All user)
$global:Caps["crossTenantMailTipsAll"] = Cap crossTenantMailTipsAll $true @(Scope All user)
Invoke-Step "get" Get
Invoke-Step "create-existing" Create @(Desired crossTenantMailTipsAll $true)

# The configuration replaces MailTips with free/busy for one group, and keeps
# a user out of calendar sharing ("All" for sharing is stored as a group scope).
$global:Caps["crossTenantMailTipsLimited"] = Cap crossTenantMailTipsLimited $true @(Scope All user)
Invoke-Step "update" Update @((Desired crossTenantCalendarAvailabilityBasic $true @("aaaaaaaa-0000-0000-0000-000000000001")),
    (Desired crossTenantCalendarSharingFreeBusySimple $true @() @("bbbbbbbb-0000-0000-0000-000000000002"))) @("crossTenantMailTipsAll")
Invoke-Step "patch" Update @((Desired crossTenantCalendarAvailabilityBasic $false @("aaaaaaaa-0000-0000-0000-000000000001")),
    (Desired crossTenantCalendarSharingFreeBusySimple $true @() @("bbbbbbbb-0000-0000-0000-000000000002"))) @("crossTenantCalendarAvailabilityBasic")
Invoke-Step "delete" Delete $null @("crossTenantCalendarAvailabilityBasic", "crossTenantCalendarSharingFreeBusySimple")
"left: $($global:Caps.Keys -join ',')"

# Nothing configured any more: gone. A missing Entra role is an error, never "gone".
Invoke-Step "gone" Get $null @("crossTenantCalendarAvailabilityBasic")
$global:Caps["crossTenantMailTipsAll"] = Cap crossTenantMailTipsAll $true @(Scope All user)
$global:RoleMissing = $true
Invoke-Step "norole" Get $null @("crossTenantMailTipsAll")
Invoke-Step "norole-import" Get
# The list hides a capability that a read by name finds: not reliable, never "gone".
$global:RoleMissing = $false
$global:ListHides = $true
Invoke-Step "hidden" Get $null @("crossTenantMailTipsAll")

# A create that fails on its second capability removes the first one: the next
# apply can create the partner again.
$global:ListHides = $false
$global:Caps.Remove("crossTenantMailTipsAll")
$global:FailPost = "crossTenantMailTipsLimited"
Invoke-Step "create-fails" Create @((Desired crossTenantMailTipsAll $true), (Desired crossTenantMailTipsLimited $true))
"after-failed-create: $($global:Caps.Keys -join ',')"
