<#
.SYNOPSIS
    Creates the least-privilege Exchange Online role group for the
    workplace-as-code/msexchangeonline Terraform provider and adds the
    provider's app registration to it.

.DESCRIPTION
    The role group gets the management roles listed in $ProviderRoles, chosen
    by hand: the narrowest roles that contain the cmdlets the bridge scripts
    run. Retention Management is replaced by a custom child role that keeps
    only the cmdlets in $RetentionCmdlets: the built-in role can also lift
    litigation holds and disable accounts. The script does not pick roles
    itself: it checks with Exchange Online that these roles still contain
    every cmdlet of the provider, and stops if one is missing.

    Without -Apply, the script only checks the roles and shows what it would do.

    Run it from an Exchange Online PowerShell session opened by an
    administrator (Connect-ExchangeOnline), with ExchangeOnlineManagement 3.x.

.PARAMETER AppId
    Application (client) ID of the app registration used by the provider.

.PARAMETER ServicePrincipalObjectId
    Object ID of the app's service principal: Entra admin center, Enterprise
    applications, select the app, Object ID. This is not the Object ID shown
    on the App registrations page.

.PARAMETER RoleGroupName
    Name of the role group to create.

.PARAMETER Apply
    Create the role group, or add the missing roles to it, and add the
    service principal. Without it, the script only shows what it would do.

.PARAMETER RemoveExtraRoles
    With -Apply, also remove from an existing role group the roles that are
    not in $ProviderRoles nor the custom retention role. Without it, the
    script only lists them.

.EXAMPLE
    ./new_role_group.ps1

    Checks the roles and shows what the script would change.

.EXAMPLE
    ./new_role_group.ps1 -AppId <client-id> -ServicePrincipalObjectId <object-id> -Apply
#>
[CmdletBinding()]
param(
    [string]$AppId = "",
    [string]$ServicePrincipalObjectId = "",
    [string]$RoleGroupName = "terraform-msexchangeonline",
    [switch]$Apply,
    [switch]$RemoveExtraRoles
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# The built-in management roles of the role group. Change this list only in a
# pull request, after checking what the new role allows: the docs list the
# same roles (a unit test compares them). Get-Recipient comes with Journaling:
# no recipient management role is needed.
$ProviderRoles = @(
    "E-Mail Address Policies",
    "Journaling",
    "Organization Configuration",
    "Organization Transport Settings",
    "Recipient Policies",
    "Remote and Accepted Domains",
    "Transport Hygiene",
    "Transport Rules"
)

# The custom role that replaces Retention Management: a child of it that keeps
# only these cmdlets. The built-in role also grants Set-Mailbox for litigation
# holds, hold exclusions and AccountDisabled, which the provider never uses.
$RetentionParent = "Retention Management"
$RetentionRole = "$RoleGroupName Retention"
$RetentionCmdlets = @(
    "Get-Mailbox",
    "Get-RetentionPolicy",
    "Get-RetentionPolicyTag",
    "New-RetentionPolicy",
    "New-RetentionPolicyTag",
    "Remove-RetentionPolicy",
    "Remove-RetentionPolicyTag",
    "Set-RetentionPolicy",
    "Set-RetentionPolicyTag"
)

# Every Exchange cmdlet run by the provider's bridge scripts (bridge/scripts).
# A unit test (tools/tools_test.go) fails when this list drifts from the scripts.
# The REST cmdlets of the module (Get-EXORecipient, Get-EXOMailbox, ...) are
# listed under the server cmdlet whose permissions they use (Get-Recipient,
# Get-Mailbox): management roles only know the server cmdlets.
$ProviderCmdlets = @(
    "Disable-ATPProtectionPolicyRule",
    "Disable-AntiPhishRule",
    "Disable-EOPProtectionPolicyRule",
    "Disable-HostedContentFilterRule",
    "Disable-HostedOutboundSpamFilterRule",
    "Disable-JournalRule",
    "Disable-MalwareFilterRule",
    "Disable-SafeAttachmentRule",
    "Disable-SafeLinksRule",
    "Disable-TransportRule",
    "Enable-ATPProtectionPolicyRule",
    "Enable-AntiPhishRule",
    "Enable-EOPProtectionPolicyRule",
    "Enable-HostedContentFilterRule",
    "Enable-HostedOutboundSpamFilterRule",
    "Enable-JournalRule",
    "Enable-MalwareFilterRule",
    "Enable-SafeAttachmentRule",
    "Enable-SafeLinksRule",
    "Enable-TransportRule",
    "Get-ATPProtectionPolicyRule",
    "Get-AcceptedDomain",
    "Get-AntiPhishPolicy",
    "Get-AntiPhishRule",
    "Get-ApplicationAccessPolicy",
    "Get-DkimSigningConfig",
    "Get-EOPProtectionPolicyRule",
    "Get-EmailAddressPolicy",
    "Get-ExternalInOutlook",
    "Get-HostedConnectionFilterPolicy",
    "Get-HostedContentFilterPolicy",
    "Get-HostedContentFilterRule",
    "Get-HostedOutboundSpamFilterPolicy",
    "Get-HostedOutboundSpamFilterRule",
    "Get-InboundConnector",
    "Get-JournalRule",
    "Get-Mailbox",
    "Get-MalwareFilterPolicy",
    "Get-MalwareFilterRule",
    "Get-MobileDeviceMailboxPolicy",
    "Get-OrganizationConfig",
    "Get-OutboundConnector",
    "Get-OwaMailboxPolicy",
    "Get-QuarantinePolicy",
    "Get-Recipient",
    "Get-RemoteDomain",
    "Get-RetentionPolicy",
    "Get-RetentionPolicyTag",
    "Get-SafeAttachmentPolicy",
    "Get-SafeAttachmentRule",
    "Get-SafeLinksPolicy",
    "Get-SafeLinksRule",
    "Get-TenantAllowBlockListItems",
    "Get-TransportConfig",
    "Get-TransportRule",
    "New-ATPProtectionPolicyRule",
    "New-AntiPhishPolicy",
    "New-AntiPhishRule",
    "New-ApplicationAccessPolicy",
    "New-DkimSigningConfig",
    "New-EOPProtectionPolicyRule",
    "New-EmailAddressPolicy",
    "New-HostedContentFilterPolicy",
    "New-HostedContentFilterRule",
    "New-HostedOutboundSpamFilterPolicy",
    "New-HostedOutboundSpamFilterRule",
    "New-InboundConnector",
    "New-JournalRule",
    "New-MalwareFilterPolicy",
    "New-MalwareFilterRule",
    "New-MobileDeviceMailboxPolicy",
    "New-OutboundConnector",
    "New-OwaMailboxPolicy",
    "New-QuarantinePolicy",
    "New-RemoteDomain",
    "New-RetentionPolicy",
    "New-RetentionPolicyTag",
    "New-SafeAttachmentPolicy",
    "New-SafeAttachmentRule",
    "New-SafeLinksPolicy",
    "New-SafeLinksRule",
    "New-TenantAllowBlockListItems",
    "New-TransportRule",
    "Remove-ATPProtectionPolicyRule",
    "Remove-AntiPhishPolicy",
    "Remove-AntiPhishRule",
    "Remove-ApplicationAccessPolicy",
    "Remove-EOPProtectionPolicyRule",
    "Remove-EmailAddressPolicy",
    "Remove-HostedContentFilterPolicy",
    "Remove-HostedContentFilterRule",
    "Remove-HostedOutboundSpamFilterPolicy",
    "Remove-HostedOutboundSpamFilterRule",
    "Remove-InboundConnector",
    "Remove-JournalRule",
    "Remove-MalwareFilterPolicy",
    "Remove-MalwareFilterRule",
    "Remove-MobileDeviceMailboxPolicy",
    "Remove-OutboundConnector",
    "Remove-OwaMailboxPolicy",
    "Remove-QuarantinePolicy",
    "Remove-RemoteDomain",
    "Remove-RetentionPolicy",
    "Remove-RetentionPolicyTag",
    "Remove-SafeAttachmentPolicy",
    "Remove-SafeAttachmentRule",
    "Remove-SafeLinksPolicy",
    "Remove-SafeLinksRule",
    "Remove-TenantAllowBlockListItems",
    "Remove-TransportRule",
    "Set-ATPProtectionPolicyRule",
    "Set-AcceptedDomain",
    "Set-AntiPhishPolicy",
    "Set-AntiPhishRule",
    "Set-ApplicationAccessPolicy",
    "Set-DkimSigningConfig",
    "Set-EOPProtectionPolicyRule",
    "Set-EmailAddressPolicy",
    "Set-ExternalInOutlook",
    "Set-HostedConnectionFilterPolicy",
    "Set-HostedContentFilterPolicy",
    "Set-HostedContentFilterRule",
    "Set-HostedOutboundSpamFilterPolicy",
    "Set-HostedOutboundSpamFilterRule",
    "Set-InboundConnector",
    "Set-JournalRule",
    "Set-MalwareFilterPolicy",
    "Set-MalwareFilterRule",
    "Set-MobileDeviceMailboxPolicy",
    "Set-OrganizationConfig",
    "Set-OutboundConnector",
    "Set-OwaMailboxPolicy",
    "Set-QuarantinePolicy",
    "Set-RemoteDomain",
    "Set-RetentionPolicy",
    "Set-RetentionPolicyTag",
    "Set-SafeAttachmentPolicy",
    "Set-SafeAttachmentRule",
    "Set-SafeLinksPolicy",
    "Set-SafeLinksRule",
    "Set-TenantAllowBlockListItems",
    "Set-TransportConfig",
    "Set-TransportRule"
)

# Find-UncoveredCmdlets returns the cmdlets that none of $Roles contains.
# $RolesByCmdlet maps each cmdlet to the names of the roles containing it.
function Find-UncoveredCmdlets {
    param([hashtable]$RolesByCmdlet, [string[]]$Roles)
    @($RolesByCmdlet.Keys | Where-Object {
        $cmdlet = $_
        -not @($RolesByCmdlet[$cmdlet] | Where-Object { $Roles -contains $_ })
    } | Sort-Object)
}

# Allows the test suite to load the functions without running anything.
if ($env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN) { return }

if (-not (Get-Command Get-ManagementRole -ErrorAction SilentlyContinue)) {
    throw "Connect to Exchange Online first: Connect-ExchangeOnline"
}

Write-Host "Checking that the roles contain the $($ProviderCmdlets.Count) cmdlets of the provider..."
$unknownRoles = @(@($ProviderRoles) + $RetentionParent | Where-Object { -not (Get-ManagementRole -Identity $_ -ErrorAction SilentlyContinue) })
if ($unknownRoles.Count -gt 0) {
    throw "These roles do not exist in this organization: $($unknownRoles -join ', ')"
}

$rolesByCmdlet = @{}
$skipped = @()
foreach ($cmdlet in $ProviderCmdlets) {
    # Some cmdlets only exist with a license (Safe Links and Safe Attachments
    # need Defender for Office 365): the organization has no role for them.
    if (-not (Get-Command $cmdlet -ErrorAction SilentlyContinue)) { $skipped += $cmdlet; continue }
    $rolesByCmdlet[$cmdlet] = @(Get-ManagementRoleEntry "*\$cmdlet" -ErrorAction SilentlyContinue |
        ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
}
if ($skipped.Count -gt 0) {
    Write-Warning ("Not available in this organization, skipped: $($skipped -join ', '). These cmdlets need a license " +
        "(Safe Links and Safe Attachments need Defender for Office 365): run the script again after adding the license.")
}

# The custom retention role contains the cmdlets of $RetentionCmdlets that its
# parent contains, whether it exists yet or not.
$parentCmdlets = @(Get-ManagementRoleEntry "$RetentionParent\*" | ForEach-Object { [string]$_.Name })
$missingFromParent = @($RetentionCmdlets | Where-Object { $parentCmdlets -notcontains $_ })
if ($missingFromParent.Count -gt 0) {
    throw "$RetentionParent no longer contains: $($missingFromParent -join ', '). Please open an issue on the provider, with this output."
}
foreach ($cmdlet in $RetentionCmdlets) {
    if ($rolesByCmdlet.ContainsKey($cmdlet)) { $rolesByCmdlet[$cmdlet] = @($rolesByCmdlet[$cmdlet]) + $RetentionRole }
}
$groupRoles = @($ProviderRoles) + $RetentionRole
$uncovered = @(Find-UncoveredCmdlets -RolesByCmdlet $rolesByCmdlet -Roles $groupRoles)
if ($uncovered.Count -gt 0) {
    $details = $uncovered | ForEach-Object { "  - $_ (in: $(@($rolesByCmdlet[$_]) -join ', '))" }
    throw ("The roles of the provider no longer contain these cmdlets:`n$($details -join "`n")`n" +
        "Exchange Online has moved them: please open an issue on the provider, with this output.")
}

Write-Host ""
Write-Host "Management roles of '$RoleGroupName':"
$ProviderRoles | ForEach-Object { Write-Host "  - $_" }
Write-Host "  - $RetentionRole (custom role from ${RetentionParent}: $($RetentionCmdlets -join ', '))"

# The custom retention role: entries to add or remove to match $RetentionCmdlets.
$retentionExists = [bool](Get-ManagementRole -Identity $RetentionRole -ErrorAction SilentlyContinue)
$retentionEntries = @()
if ($retentionExists) { $retentionEntries = @(Get-ManagementRoleEntry "$RetentionRole\*" | ForEach-Object { [string]$_.Name }) }
$entriesToRemove = @($retentionEntries | Where-Object { $RetentionCmdlets -notcontains $_ })
$entriesToAdd = @($RetentionCmdlets | Where-Object { $retentionEntries -notcontains $_ })
if (-not $retentionExists) {
    Write-Host "The custom role '$RetentionRole' will be created."
} elseif ($entriesToRemove.Count + $entriesToAdd.Count -gt 0) {
    Write-Host "The custom role '$RetentionRole' will be updated: add $($entriesToAdd.Count) and remove $($entriesToRemove.Count) cmdlets."
}

# Roles of an existing role group that are not in the list, for example Mail
# Recipient Creation, granted by versions before 0.3.1.
$extra = @()
if (Get-RoleGroup -Identity $RoleGroupName -ErrorAction SilentlyContinue) {
    $current = @((Get-RoleGroup -Identity $RoleGroupName).Roles | ForEach-Object { (Get-ManagementRole -Identity $_).Name })
    $extra = @($current | Where-Object { $groupRoles -notcontains $_ } | Sort-Object)
}
if ($extra.Count -gt 0 -and -not ($Apply -and $RemoveExtraRoles)) {
    Write-Host ""
    Write-Warning ("'$RoleGroupName' has roles the provider does not need: $($extra -join ', '). " +
        "Remove them with -Apply -RemoveExtraRoles.")
}

if (-not $Apply) {
    Write-Host ""
    Write-Host "Nothing changed. Run again with -AppId, -ServicePrincipalObjectId and -Apply to create or update '$RoleGroupName'."
    return
}
if (-not $AppId -or -not $ServicePrincipalObjectId) {
    throw "-Apply needs -AppId and -ServicePrincipalObjectId."
}

# The custom role first, so that the role group never misses its cmdlets. A
# child role starts with every entry of its parent: the unneeded ones are removed.
if (-not $retentionExists) {
    New-ManagementRole -Name $RetentionRole -Parent $RetentionParent -Description "Retention policy cmdlets for the workplace-as-code/msexchangeonline Terraform provider" | Out-Null
    Write-Host "Created the custom role '$RetentionRole'."
    $entriesToRemove = @(Get-ManagementRoleEntry "$RetentionRole\*" | ForEach-Object { [string]$_.Name } | Where-Object { $RetentionCmdlets -notcontains $_ })
    $entriesToAdd = @()
}
foreach ($cmdlet in $entriesToRemove) {
    Remove-ManagementRoleEntry -Identity "$RetentionRole\$cmdlet" -Confirm:$false
}
foreach ($cmdlet in $entriesToAdd) {
    Add-ManagementRoleEntry -Identity "$RetentionRole\$cmdlet" | Out-Null
}
if ($entriesToRemove.Count + $entriesToAdd.Count -gt 0) {
    Write-Host "'$RetentionRole' now contains: $($RetentionCmdlets -join ', ')."
}

$sp = Get-ServicePrincipal -Identity $ServicePrincipalObjectId -ErrorAction SilentlyContinue
if (-not $sp) {
    $sp = New-ServicePrincipal -AppId $AppId -ObjectId $ServicePrincipalObjectId -DisplayName $RoleGroupName
    Write-Host "Created the Exchange service principal for app $AppId."
}

if (Get-RoleGroup -Identity $RoleGroupName -ErrorAction SilentlyContinue) {
    $current = @((Get-RoleGroup -Identity $RoleGroupName).Roles | ForEach-Object { (Get-ManagementRole -Identity $_).Name })
    foreach ($role in $groupRoles | Where-Object { $current -notcontains $_ }) {
        New-ManagementRoleAssignment -SecurityGroup $RoleGroupName -Role $role | Out-Null
        Write-Host "Added role '$role' to '$RoleGroupName'."
    }
    if ($RemoveExtraRoles) {
        foreach ($role in $extra) {
            Get-ManagementRoleAssignment -RoleAssignee $RoleGroupName -Role $role |
                Where-Object { $_.RoleAssigneeName -eq $RoleGroupName } |
                Remove-ManagementRoleAssignment -Confirm:$false
            Write-Host "Removed role '$role' from '$RoleGroupName'."
        }
    }
} else {
    New-RoleGroup -Name $RoleGroupName -Roles $groupRoles -Description "Least-privilege roles for the workplace-as-code/msexchangeonline Terraform provider" | Out-Null
    Write-Host "Created role group '$RoleGroupName'."
}

$members = @(Get-RoleGroupMember -Identity $RoleGroupName | ForEach-Object { [string]$_.Identity })
if ($members -notcontains [string]$sp.Identity) {
    Add-RoleGroupMember -Identity $RoleGroupName -Member $sp.Identity
    Write-Host "Added the app's service principal to '$RoleGroupName'."
}
Write-Host "Done. New permissions can take a while, sometimes more than an hour, to apply to new sessions."
