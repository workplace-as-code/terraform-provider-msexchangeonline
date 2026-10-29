<#
.SYNOPSIS
    Creates the least-privilege Exchange Online role group for the
    workplace-as-code/msexchangeonline Terraform provider and adds the
    provider's app registration to it.

.DESCRIPTION
    The provider only needs the management roles that contain the cmdlets its
    bridge scripts run. This script asks Exchange Online which roles contain
    each of those cmdlets (Get-ManagementRole -Cmdlet), picks a small set of
    roles that covers all of them, then creates the role group and adds the
    app's service principal to it.

    Without -Apply, the script only prints the roles it would use.

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
    Create the role group and add the service principal. Without it, the
    script only shows what it would do.

.EXAMPLE
    ./new_role_group.ps1

    Lists the management roles the provider needs.

.EXAMPLE
    ./new_role_group.ps1 -AppId <client-id> -ServicePrincipalObjectId <object-id> -Apply
#>
[CmdletBinding()]
param(
    [string]$AppId = "",
    [string]$ServicePrincipalObjectId = "",
    [string]$RoleGroupName = "terraform-msexchangeonline",
    [switch]$Apply
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

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

# Select-BridgeRoles returns a small set of roles that together contain every
# cmdlet. $RolesByCmdlet maps each cmdlet to the names of the roles containing it.
# Weighted greedy set cover: repeatedly take the role whose cmdlets are most
# useful, i.e. the highest ratio of still-needed cmdlets to all cmdlets in the
# role. A narrow role such as "Transport Rules" therefore wins over a broad
# role that happens to contain the same cmdlets plus hundreds of others.
function Select-BridgeRoles {
    param([hashtable]$RolesByCmdlet, [hashtable]$RoleSize = @{})
    $missing = @($RolesByCmdlet.Keys | Where-Object { @($RolesByCmdlet[$_]).Count -eq 0 })
    if ($missing.Count -gt 0) { throw "No management role contains: $($missing -join ', ')" }

    $remaining = [System.Collections.Generic.HashSet[string]]::new([string[]]@($RolesByCmdlet.Keys))
    $selected  = [System.Collections.Generic.List[string]]::new()
    while ($remaining.Count -gt 0) {
        $coverage = @{}
        foreach ($cmdlet in $remaining) {
            foreach ($role in @($RolesByCmdlet[$cmdlet])) { $coverage[$role] = 1 + [int]$coverage[$role] }
        }
        $best = $coverage.GetEnumerator() |
            Sort-Object @{ Expression = {
                            $size = if ($RoleSize.ContainsKey($_.Key) -and $RoleSize[$_.Key] -gt 0) { $RoleSize[$_.Key] } else { $_.Value }
                            $_.Value / $size }; Descending = $true },
                        @{ Expression = { $_.Value }; Descending = $true },
                        @{ Expression = { $_.Key } } |
            Select-Object -First 1
        $selected.Add($best.Key)
        foreach ($cmdlet in @($remaining)) {
            if (@($RolesByCmdlet[$cmdlet]) -contains $best.Key) { [void]$remaining.Remove($cmdlet) }
        }
    }
    @($selected | Sort-Object)
}

# Select-FullRoles keeps, among the roles that contain a cmdlet, the ones that
# grant (almost) all its parameters. A role can grant a cmdlet with only some
# of them: Mail Tips grants Set-OrganizationConfig for the MailTips*
# parameters only. $ParameterCount maps each role to its parameter count.
function Select-FullRoles {
    param([hashtable]$ParameterCount)
    if ($ParameterCount.Count -eq 0) { return @() }
    $max = ($ParameterCount.Values | Measure-Object -Maximum).Maximum
    @($ParameterCount.Keys | Where-Object { $ParameterCount[$_] -ge 0.9 * $max } | Sort-Object)
}

# Allows the test suite to load the functions without running anything.
if ($env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN) { return }

if (-not (Get-Command Get-ManagementRole -ErrorAction SilentlyContinue)) {
    throw "Connect to Exchange Online first: Connect-ExchangeOnline"
}

Write-Host "Looking up the management roles for $($ProviderCmdlets.Count) cmdlets..."
$rolesByCmdlet = @{}
$roleSize = @{}
$skipped = @()
$endUserRoles = @(Get-ManagementRole | Where-Object { $_.IsEndUserRole } | ForEach-Object { [string]$_.Name })
foreach ($cmdlet in $ProviderCmdlets) {
    # Some cmdlets only exist with a license (Safe Links and Safe Attachments
    # need Defender for Office 365): the organization has no role for them.
    if (-not (Get-Command $cmdlet -ErrorAction SilentlyContinue)) { $skipped += $cmdlet; continue }
    $counts = @{}
    foreach ($entry in @(Get-ManagementRoleEntry "*\$cmdlet" -ErrorAction SilentlyContinue)) {
        if ($endUserRoles -notcontains [string]$entry.Role) { $counts[[string]$entry.Role] = @($entry.Parameters).Count }
    }
    $rolesByCmdlet[$cmdlet] = Select-FullRoles -ParameterCount $counts
}
if ($skipped.Count -gt 0) {
    Write-Warning ("Not available in this organization, skipped: $($skipped -join ', '). These cmdlets need a license " +
        "(Safe Links and Safe Attachments need Defender for Office 365): run the script again after adding the license.")
}
foreach ($role in ($rolesByCmdlet.Values | ForEach-Object { $_ } | Sort-Object -Unique)) {
    $roleSize[$role] = @(Get-ManagementRoleEntry "$role\*").Count
}

$selected = Select-BridgeRoles -RolesByCmdlet $rolesByCmdlet -RoleSize $roleSize
Write-Host ""
Write-Host "Management roles needed by the provider:"
$selected | ForEach-Object { Write-Host "  - $_" }

if (-not $Apply) {
    Write-Host ""
    Write-Host "Nothing changed. Run again with -AppId, -ServicePrincipalObjectId and -Apply to create '$RoleGroupName'."
    return
}
if (-not $AppId -or -not $ServicePrincipalObjectId) {
    throw "-Apply needs -AppId and -ServicePrincipalObjectId."
}

$sp = Get-ServicePrincipal -Identity $ServicePrincipalObjectId -ErrorAction SilentlyContinue
if (-not $sp) {
    $sp = New-ServicePrincipal -AppId $AppId -ObjectId $ServicePrincipalObjectId -DisplayName $RoleGroupName
    Write-Host "Created the Exchange service principal for app $AppId."
}

if (Get-RoleGroup -Identity $RoleGroupName -ErrorAction SilentlyContinue) {
    $current = @((Get-RoleGroup -Identity $RoleGroupName).Roles | ForEach-Object { (Get-ManagementRole -Identity $_).Name })
    foreach ($role in $selected | Where-Object { $current -notcontains $_ }) {
        New-ManagementRoleAssignment -SecurityGroup $RoleGroupName -Role $role | Out-Null
        Write-Host "Added role '$role' to '$RoleGroupName'."
    }
} else {
    New-RoleGroup -Name $RoleGroupName -Roles $selected -Description "Least-privilege roles for the workplace-as-code/msexchangeonline Terraform provider" | Out-Null
    Write-Host "Created role group '$RoleGroupName'."
}

$members = @(Get-RoleGroupMember -Identity $RoleGroupName | ForEach-Object { [string]$_.Identity })
if ($members -notcontains [string]$sp.Identity) {
    Add-RoleGroupMember -Identity $RoleGroupName -Member $sp.Identity
    Write-Host "Added the app's service principal to '$RoleGroupName'."
}
Write-Host "Done. New permissions can take a while to apply to new sessions."
