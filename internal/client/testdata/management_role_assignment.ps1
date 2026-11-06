# management_role_assignment.ps1 - runs bridge/scripts/management_role_assignment.ps1
# against fake assignment cmdlets (run by rbac_scripts_test.go): a role group
# gets the role with -SecurityGroup, an app with -App, and a scope left out of
# the configuration is removed.
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Calls = [System.Collections.Generic.List[string]]::new()
$global:Assignments = @{}
function global:Get-ConnectionInformation { [pscustomobject]@{ State = "Connected" } }
function Save-Call($Verb, $Bound) {
    $keys = $Bound.Keys | Where-Object { $_ -ne "Identity" } | Sort-Object
    $global:Calls.Add("$Verb " + (($keys | ForEach-Object { "$_=$($Bound[$_])" }) -join ","))
}
function global:New-ManagementRoleAssignment {
    param($Name, $Role, $SecurityGroup, $App, $CustomRecipientWriteScope, $ExclusiveRecipientWriteScope,
        $CustomResourceScope, $RecipientAdministrativeUnitScope, $RecipientGroupScope, $RecipientRelativeWriteScope)
    Save-Call "New" $PSBoundParameters
    $assignee = if ($SecurityGroup) { $SecurityGroup } else { "SP-$App" }
    if (-not $Name) { $Name = "$Role-$assignee" }
    $a = [pscustomobject]@{
        Name = $Name; Role = $Role; RoleAssigneeName = $assignee; Enabled = $true
        RoleAssigneeType = if ($SecurityGroup) { "RoleGroup" } else { "ServicePrincipal" }
        CustomRecipientWriteScope = $CustomRecipientWriteScope; ExclusiveRecipientWriteScope = $null
        CustomResourceScope = $CustomResourceScope; RecipientAdministrativeUnitScope = $null; RecipientGroupScope = $null
        RecipientWriteScope = if ($CustomRecipientWriteScope) { "CustomRecipientScope" } else { "Organization" }
    }
    $global:Assignments[$Name] = $a
    $a
}
function global:Get-ManagementRoleAssignment {
    param($Identity)
    if (-not $global:Assignments.ContainsKey($Identity)) { throw "The operation couldn't be performed because object '$Identity' couldn't be found." }
    $global:Assignments[$Identity]
}
function global:Set-ManagementRoleAssignment {
    param($Identity, $Enabled, $CustomRecipientWriteScope, $ExclusiveRecipientWriteScope, $CustomResourceScope,
        $RecipientAdministrativeUnitScope, $RecipientGroupScope, $RecipientRelativeWriteScope)
    Save-Call "Set" $PSBoundParameters
    $a = $global:Assignments[$Identity]
    if ($PSBoundParameters.ContainsKey("Enabled")) { $a.Enabled = $Enabled }
    if ($PSBoundParameters.ContainsKey("CustomRecipientWriteScope")) { $a.CustomRecipientWriteScope = $CustomRecipientWriteScope; $a.RecipientWriteScope = "CustomRecipientScope" }
    if ($RecipientRelativeWriteScope) { $a.CustomRecipientWriteScope = $null; $a.RecipientWriteScope = $RecipientRelativeWriteScope }
}
function global:Remove-ManagementRoleAssignment { param($Identity) $global:Calls.Add("Remove $Identity"); $global:Assignments.Remove($Identity) }
function global:Get-ServicePrincipal {
    param($Identity)
    [pscustomobject]@{ Identity = $Identity; AppId = ($Identity -replace '^SP-', ''); DisplayName = $Identity }
}

function ConvertTo-B64($Object) {
    [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject $Object)))
}
function Invoke-Step($Label, $Action, $Identity, $Values) {
    $global:Calls.Clear()
    $out = & $Script -Action $Action -Identity $Identity -Values (ConvertTo-B64 $Values) 2>&1
    "${Label}: $out"
    "$Label calls: $($global:Calls -join '; ')"
}
$empty = @{ RoleGroup = ""; App = ""; CustomRecipientWriteScope = ""; ExclusiveRecipientWriteScope = ""; CustomResourceScope = ""
    RecipientAdministrativeUnitScope = ""; RecipientGroupScope = "" }
function Merge($Extra) { $h = $empty.Clone(); foreach ($k in $Extra.Keys) { $h[$k] = $Extra[$k] }; $h }

Invoke-Step "group" Create "" (Merge @{ Role = "TF-CI-Role"; RoleGroup = "TF-CI-RoleGroup"; CustomRecipientWriteScope = "TF-CI-Scope" })
Invoke-Step "app" Create "" (Merge @{ Role = "Application Mail.Read"; App = "11111111-2222-3333-4444-555555555555"; CustomResourceScope = "TF-CI-Scope" })
Invoke-Step "unscope" Update "TF-CI-Role-TF-CI-RoleGroup" (Merge @{ Role = "TF-CI-Role"; RoleGroup = "TF-CI-RoleGroup"; Enabled = $false })

# Exchange may report an administrative unit or group scope as the custom
# recipient scope, with RecipientWriteScope saying which kind it is.
$global:Assignments["TF-CI-AU"] = [pscustomobject]@{ Name = "TF-CI-AU"; Role = "Mail Recipients"; RoleAssigneeName = "TF-CI-RoleGroup"
    RoleAssigneeType = "RoleGroup"; Enabled = $true; CustomRecipientWriteScope = "au-1111"; RecipientWriteScope = "AdministrativeUnit" }
$global:Assignments["TF-CI-Group"] = [pscustomobject]@{ Name = "TF-CI-Group"; Role = "Mail Recipients"; RoleAssigneeName = "TF-CI-RoleGroup"
    RoleAssigneeType = "RoleGroup"; Enabled = $true; CustomRecipientWriteScope = "Helpdesk Users"; RecipientWriteScope = "Group" }
"au: " + (& $Script -Action Get -Identity TF-CI-AU)
"group: " + (& $Script -Action Get -Identity TF-CI-Group)
