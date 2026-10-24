# transport_rule.ps1 — CRUD for Exchange transport rules (mail flow rules)
# Called by the Terraform provider via the Go client bridge.
#
# Output contract (shared by every bridge script):
#   - success            → JSON object on stdout, exit 0
#   - Get on missing rule → empty stdout, exit 0 (provider drops it from state)
#   - any other failure  → message on stderr, exit 1
#
# List parameters arrive as base64-encoded JSON arrays so that values containing
# commas, quotes or spaces survive the command line intact.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity,
    [string]$Name,
    [string]$Priority               = "",       # empty = let Exchange decide
    [string]$Enabled,  # only sent when set in the configuration
    [string]$RuleAction,
    [string]$RejectMessage,
    [string]$FromAddressContains    = "",       # base64 JSON array
    [string]$SubjectContains        = "",       # base64 JSON array
    [string]$AttachmentIsExecutable = "false"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -TenantId $TenantId -ClientId $ClientId -CertificatePath $CertificatePath -CertificatePassword $CertificatePassword -CertificateThumbprint $CertificateThumbprint -Environment $Environment

# SCL value used for the "MoveToJmf" action: SCL 6 routes the message to the
# recipient's Junk Email folder.
$JunkScl = 6

function ConvertFrom-B64JsonArray([string]$Value) {
    if (-not $Value) { return @() }
    $json = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
    return @($json | ConvertFrom-Json)
}

function ConvertTo-StringArray($Values) {
    if ($null -eq $Values) { return @() }
    return [string[]]@($Values | ForEach-Object { $_.ToString() })
}

$AttachmentIsExecutableBool = $AttachmentIsExecutable -eq "true"
$FromAddressArr             = @(ConvertFrom-B64JsonArray $FromAddressContains)
$SubjectArr                 = @(ConvertFrom-B64JsonArray $SubjectContains)

# Map a rule object to the JSON schema returned to Terraform.
function ConvertTo-BridgeRule {
    param($Rule)

    $actionName = if ($Rule.RejectMessageReasonText) { "Reject" }
                  elseif ($Rule.Quarantine)           { "Quarantine" }
                  elseif ($Rule.SetSCL -eq $JunkScl)  { "MoveToJmf" }
                  else                                { "Unsupported" }

    @{
        Identity               = "$($Rule.Identity)"
        Name                   = $Rule.Name
        Priority               = $Rule.Priority
        Enabled                = ($Rule.State -eq "Enabled")
        Action                 = $actionName
        RejectMessage          = if ($Rule.RejectMessageReasonText) { $Rule.RejectMessageReasonText } else { "" }
        FromAddressContains    = @(ConvertTo-StringArray $Rule.FromAddressContainsWords)
        SubjectContains        = @(ConvertTo-StringArray $Rule.SubjectContainsWords)
        AttachmentIsExecutable = [bool]$Rule.AttachmentHasExecutableContent
    } | ConvertTo-Json -Compress
}

# Builds the action and condition parameters. On Update, removed conditions and
# previous actions are explicitly cleared with $null so Exchange matches the plan.
function Get-RuleParams([bool]$IsUpdate) {
    $p = @{}

    switch ($RuleAction) {
        "Reject"     { $p["RejectMessageReasonText"] = $RejectMessage }
        "Quarantine" { $p["Quarantine"] = $true }
        "MoveToJmf"  { $p["SetSCL"] = $JunkScl }
        default      { throw "Unsupported action '$RuleAction'. Valid values: Reject, Quarantine, MoveToJmf." }
    }
    if ($IsUpdate) {
        if ($RuleAction -ne "Reject")     { $p["RejectMessageReasonText"] = $null }
        if ($RuleAction -ne "Quarantine") { $p["Quarantine"] = $false }
        if ($RuleAction -ne "MoveToJmf")  { $p["SetSCL"] = $null }
    }

    if ($FromAddressArr.Count -gt 0)  { $p["FromAddressContainsWords"] = [string[]]$FromAddressArr }
    elseif ($IsUpdate)                { $p["FromAddressContainsWords"] = $null }

    if ($SubjectArr.Count -gt 0)      { $p["SubjectContainsWords"] = [string[]]$SubjectArr }
    elseif ($IsUpdate)                { $p["SubjectContainsWords"] = $null }

    if ($AttachmentIsExecutableBool)  { $p["AttachmentHasExecutableContent"] = $true }
    elseif ($IsUpdate)                { $p["AttachmentHasExecutableContent"] = $false }

    # Safety net, duplicated from the provider's plan-time validation: a rule
    # without conditions applies to every message in the organization.
    if ($FromAddressArr.Count -eq 0 -and $SubjectArr.Count -eq 0 -and -not $AttachmentIsExecutableBool) {
        throw "Refusing to write transport rule '$Name' without any condition: it would apply to all mail."
    }
    return $p
}

try {
    switch ($Action) {

        "Get" {
            try {
                $rule = Get-TransportRule -Identity $Identity -ErrorAction Stop
            } catch {
                if ($_.Exception.Message -match "couldn't be found|not found|ObjectNotFound") { exit 0 }
                throw
            }
            ConvertTo-BridgeRule -Rule $rule
        }

        "Create" {
            $params = Get-RuleParams -IsUpdate $false
            $params["Name"] = $Name
            # Create disabled when requested, so the rule never runs unwanted.
            if ($PSBoundParameters.ContainsKey("Enabled")) { $params["Enabled"] = ($Enabled -eq "true") }
            if ($Priority -ne "") { $params["Priority"] = [int]$Priority }

            $rule = New-TransportRule @params
            ConvertTo-BridgeRule -Rule (Get-TransportRule -Identity $rule.Identity -ErrorAction Stop)
        }

        "Update" {
            $params = Get-RuleParams -IsUpdate $true
            $params["Identity"] = $Identity
            if ($Name -and $Name -ne $Identity) { $params["Name"] = $Name }
            if ($Priority -ne "") { $params["Priority"] = [int]$Priority }

            Set-TransportRule @params

            $newIdentity = if ($params.ContainsKey("Name")) { $Name } else { $Identity }
            if ($PSBoundParameters.ContainsKey("Enabled")) {
                if ($Enabled -eq "true") { Enable-TransportRule -Identity $newIdentity }
                else                     { Disable-TransportRule -Identity $newIdentity }
            }

            ConvertTo-BridgeRule -Rule (Get-TransportRule -Identity $newIdentity -ErrorAction Stop)
        }

        "Delete" {
            try {
                Remove-TransportRule -Identity $Identity -ErrorAction Stop
            } catch {
                # Already gone is a successful delete; anything else is a real error.
                if ($_.Exception.Message -notmatch "couldn't be found|not found|ObjectNotFound") { throw }
            }
            Write-Output "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("transport_rule $Action failed: $($_.Exception.Message)")
    exit 1
}
