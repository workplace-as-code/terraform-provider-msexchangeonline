# common.ps1 — Helpers shared by the bridge scripts.
# Dot-sourced after auth.ps1.

# ConvertTo-BridgeSplat builds the cmdlet parameters from the script parameters
# that the Go client actually sent. Attributes absent from the Terraform
# configuration are not sent, so Exchange keeps its current value for them.
#   -List  base64 JSON lists; "[]" clears the property
#   -Csv   comma-separated lists (older resources); "" clears the property
function ConvertTo-BridgeSplat {
    param(
        $Bound,
        [string[]]$Bool   = @(),
        [string[]]$Int    = @(),
        [string[]]$String = @(),
        [string[]]$List   = @(),
        [string[]]$Csv    = @()
    )
    $splat = @{}
    foreach ($k in $Bool)   { if ($Bound.ContainsKey($k)) { $splat[$k] = ([string]$Bound[$k] -eq "true") } }
    foreach ($k in $Int)    { if ($Bound.ContainsKey($k)) { $splat[$k] = [int64]$Bound[$k] } }
    foreach ($k in $String) { if ($Bound.ContainsKey($k)) { $splat[$k] = [string]$Bound[$k] } }
    foreach ($k in $List)   {
        if (-not $Bound.ContainsKey($k)) { continue }
        $items = @(ConvertFrom-BridgeList ([string]$Bound[$k]))
        $splat[$k] = if ($items.Count -gt 0) { [string[]]$items } else { $null }
    }
    foreach ($k in $Csv) {
        if (-not $Bound.ContainsKey($k)) { continue }
        $items = @(ConvertFrom-BridgeCsv ([string]$Bound[$k]))
        $splat[$k] = if ($items.Count -gt 0) { [string[]]$items } else { $null }
    }
    $splat
}

# ConvertFrom-BridgeCsv splits a comma-separated list ("" is an empty list).
function ConvertFrom-BridgeCsv {
    param([string]$Value)
    @($Value -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

# ConvertTo-BridgeConditions returns the rule conditions that the Go client
# sent, as string arrays (an empty array clears the condition). Conditions
# absent from the Terraform configuration are left out: Sync-BridgeRule keeps
# their current value.
function ConvertTo-BridgeConditions {
    param($Bound, [string[]]$Csv)
    $conditions = @{}
    foreach ($k in $Csv) {
        if ($Bound.ContainsKey($k)) { $conditions[$k] = [string[]]@(ConvertFrom-BridgeCsv ([string]$Bound[$k])) }
    }
    $conditions
}

# Assert-BridgeParameters fails when a configured setting is not a parameter of
# the cmdlet in this tenant, typically a Microsoft Defender for Office 365
# setting in a tenant without that license. Dropping it silently would leave
# Terraform with a value that never sticks.
function Assert-BridgeParameters {
    param([hashtable]$Splat, [string]$Cmdlet)
    $available = (Get-Command $Cmdlet -ErrorAction Stop).Parameters.Keys
    $missing = @($Splat.Keys | Where-Object { $available -notcontains $_ })
    if ($missing.Count -gt 0) {
        throw "$Cmdlet has no parameter $($missing -join ', ') in this tenant: these settings need a Microsoft Defender for Office 365 license. Remove them from the configuration."
    }
}

# ConvertFrom-BridgeList decodes a list sent by the Go client: base64 JSON, or
# "[]" / "" for an empty list.
function ConvertFrom-BridgeList {
    param([string]$Value)
    if (-not $Value -or $Value -eq "[]") { return @() }
    $json = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
    return @($json | ConvertFrom-Json)
}

# ConvertFrom-BridgeJson decodes a base64 JSON value sent by the Go client.
function ConvertFrom-BridgeJson {
    param([string]$Value)
    if (-not $Value) { return $null }
    $json = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
    return ($json | ConvertFrom-Json)
}

# ConvertFrom-BridgeValues turns the base64 JSON object of parameter values sent
# by the generic resources into a splat hashtable. JSON arrays become string
# arrays; an empty array clears a multi-valued parameter.
function ConvertFrom-BridgeValues {
    param([string]$Value)
    $splat = @{}
    $object = ConvertFrom-BridgeJson $Value
    if ($null -eq $object) { return $splat }
    foreach ($p in $object.PSObject.Properties) {
        $v = $p.Value
        if ($v -is [array]) { $v = if ($v.Count -gt 0) { [string[]]$v } else { $null } }
        $splat[$p.Name] = $v
    }
    $splat
}

# ConvertTo-BridgeValue normalizes a property value for JSON: booleans and
# numbers stay typed, collections become string arrays, anything else (time
# spans, sizes, "Unlimited", ...) becomes a string.
function ConvertTo-BridgeValue {
    param($Value)
    if ($null -eq $Value) { return $null }
    if ($Value -is [bool] -or $Value -is [string]) { return $Value }
    if ($Value -is [int] -or $Value -is [long] -or $Value -is [uint32] -or $Value -is [uint64] -or
        $Value -is [int16] -or $Value -is [uint16] -or $Value -is [byte]) { return [long]$Value }
    if ($Value -is [System.Collections.IEnumerable]) {
        return ,([string[]]@($Value | Where-Object { $null -ne $_ } | ForEach-Object { $_.ToString() }))
    }
    return $Value.ToString()
}

# ConvertTo-BridgeStrings turns a multi-valued Exchange property into a string
# array (empty when the property is null).
function ConvertTo-BridgeStrings {
    param($Values)
    if ($null -eq $Values) { return @() }
    return [string[]]@($Values | ForEach-Object { $_.ToString() })
}

# Test-BridgeNotFound reports whether an error means the object does not exist.
function Test-BridgeNotFound {
    param([string]$Message)
    $Message -match "couldn't be found|could not be found|not found|ObjectNotFound|doesn't exist|does not exist"
}

# Get-BridgeProp reads a property that may be missing on some objects or module
# versions; under StrictMode a direct access to it would throw.
function Get-BridgeProp {
    param($Object, [string]$Name, $Default = $null)
    $p = $Object.PSObject.Properties[$Name]
    if ($null -ne $p) { $p.Value } else { $Default }
}

# Invoke-BridgeRetry runs a script block and retries it when Exchange Online
# reports a transient error (server-side error, throttling, timeout).
function Invoke-BridgeRetry {
    param([scriptblock]$Command, [int]$MaxAttempts = 4, [switch]$Create)
    for ($attempt = 1; ; $attempt++) {
        try {
            return & $Command
        } catch {
            $message = $_.Exception.Message
            $transient = $message -match 'server side error|try again|throttl|\b429\b|\b503\b|timed out|temporarily unavailable'
            # "A task was canceled" is a client-side timeout: the request may
            # have been processed. Reads and updates can be sent again, a
            # creation would then fail on a duplicate.
            if (-not $Create -and $message -match 'task was canceled') { $transient = $true }
            if (-not $transient -or $attempt -ge $MaxAttempts) { throw }
            Start-Sleep -Seconds (5 * $attempt)
        }
    }
}

# Wait-BridgeConsistent reads an object until a check passes. Exchange Online
# can return the previous value for a few seconds after a write.
function Wait-BridgeConsistent {
    param([scriptblock]$Read, [scriptblock]$Check, [int]$MaxAttempts = 6)
    for ($attempt = 1; ; $attempt++) {
        $object = & $Read
        if ((& $Check $object) -or $attempt -ge $MaxAttempts) { return $object }
        Start-Sleep -Seconds (2 * $attempt)
    }
}

# Sync-BridgeRule creates, updates or removes the rule that scopes a policy to
# recipients, for the resources that manage a policy and its rule as one unit
# (anti-phish, malware filter, spam filter, Safe Links, Safe Attachments, ...).
#
# Exchange links a policy to at most one rule, and a rule needs at least one
# inclusion condition (RecipientDomainIs, SentTo, SentToMemberOf, or the sender
# conditions of outbound spam rules). So the rule
# exists only while one of them is set; ExceptIf* conditions alone do not
# count. Without one, any existing rule is removed and the policy can be scoped
# by a standalone rule resource instead.
#
# Only the conditions present in -Conditions are managed: the others keep
# their current value on the existing rule. With no condition, priority or
# state to manage, the rule is left alone.
#
#   -Noun        cmdlet noun of the rule, e.g. "MalwareFilterRule"
#   -PolicyParam rule parameter that names the policy, e.g. "MalwareFilterPolicy"
#   -Conditions  hashtable of condition parameter -> string array (empty = clear)
#   -Priority    "" keeps the current priority (or lets Exchange choose)
#   -Enabled     $null keeps the current state (a new rule is enabled)
function Sync-BridgeRule {
    param(
        [string]$Noun,
        [string]$PolicyParam,
        [string]$Name,
        [hashtable]$Conditions,
        [string]$Priority = "",
        $Enabled = $null
    )
    if ($Conditions.Count -eq 0 -and $Priority -eq "" -and $null -eq $Enabled) { return }

    $rule = $null
    try { $rule = & "Get-$Noun" -Identity $Name -ErrorAction Stop }
    catch { if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw } }

    # Inclusion conditions after this change: the configured ones, and the
    # current ones that the configuration does not manage.
    $scoped = $false
    foreach ($k in "RecipientDomainIs", "SentTo", "SentToMemberOf", "SenderDomainIs", "From", "FromMemberOf") {
        $values = @(if ($Conditions.ContainsKey($k)) { $Conditions[$k] }
                    elseif ($null -ne $rule) { Get-BridgeProp $rule $k | Where-Object { $null -ne $_ } })
        if ($values.Count -gt 0) { $scoped = $true }
    }

    if (-not $scoped) {
        if ($null -ne $rule) { & "Remove-$Noun" -Identity $Name }
        return
    }

    if ($null -eq $rule) {
        $p = @{ Name = $Name; $PolicyParam = $Name; Enabled = ($null -eq $Enabled -or [bool]$Enabled) }
        foreach ($k in $Conditions.Keys) { if (@($Conditions[$k]).Count -gt 0) { $p[$k] = [string[]]$Conditions[$k] } }
        # Priority is set afterwards: New-*Rule rejects a value outside
        # 0..(rule count - 1), which is only known once the rule exists.
        & "New-$Noun" @p | Out-Null
        if ($Priority -ne "") {
            try { & "Set-$Noun" -Identity $Name -Priority ([int]$Priority) -ErrorAction Stop }
            catch {
                $priorityError = $_.Exception.Message
                & "Remove-$Noun" -Identity $Name -ErrorAction SilentlyContinue
                throw "Invalid priority $Priority for rule '$Name': $priorityError"
            }
        }
        return
    }

    $p = @{}
    # $null clears a condition that was emptied in the configuration.
    foreach ($k in $Conditions.Keys) { $p[$k] = if (@($Conditions[$k]).Count -gt 0) { [string[]]$Conditions[$k] } else { $null } }
    if ($Priority -ne "") { $p["Priority"] = [int]$Priority }
    if ($p.Count -gt 0) { & "Set-$Noun" -Identity $Name @p }
    if ($null -ne $Enabled) {
        if ([bool]$Enabled) { & "Enable-$Noun" -Identity $Name }
        else                { & "Disable-$Noun" -Identity $Name }
    }
}

# Remove-BridgeRule removes the rule of a policy when it exists.
function Remove-BridgeRule {
    param([string]$Noun, [string]$Name)
    try { & "Remove-$Noun" -Identity $Name -ErrorAction Stop }
    catch { if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw } }
}

# Invoke-BridgePresetRule manages a rule of a preset security policy (Standard
# or Strict) for eop_protection_policy_rule.ps1 and atp_protection_policy_rule.ps1,
# with the contract of exo_object.ps1. The Defender portal creates these rules
# when the preset is first turned on, with the policies they need, and
# Microsoft advises against New-<noun>: Create adopts the existing rule, and
# Delete turns the preset off and keeps the rule. Enabled is read from State.
function Invoke-BridgePresetRule {
    param(
        [ValidateSet("EOPProtectionPolicyRule", "ATPProtectionPolicyRule")] [string]$Noun,
        [string]$Action, [string]$Identity, [string]$Values, [string]$Properties
    )
    $splat = ConvertFrom-BridgeValues $Values
    $props = @(ConvertFrom-BridgeJson $Properties)
    $enabled = $null
    if ($splat.ContainsKey("Enabled")) { $enabled = [bool]$splat["Enabled"]; $splat.Remove("Enabled") }

    $read = {
        $rule = Invoke-BridgeRetry { & "Get-$Noun" -Identity $Identity }
        $out = [ordered]@{ Name = [string]$rule.Name }
        foreach ($p in $props) {
            $out[$p] = if ($p -eq "Enabled") { [string](Get-BridgeProp $rule "State") -eq "Enabled" }
                       else { ConvertTo-BridgeValue (Get-BridgeProp $rule $p) }
        }
        ConvertTo-Json -Compress -Depth 4 -InputObject $out
    }
    $write = {
        if ($splat.Count -gt 0) { & "Set-$Noun" -Identity $Identity @splat }
        if ($enabled -eq $true)  { & "Enable-$Noun" -Identity $Identity }
        if ($enabled -eq $false) { & "Disable-$Noun" -Identity $Identity }
    }

    switch ($Action) {
        "Get" {
            try { & $read } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { return }
                throw
            }
        }
        "Create" {
            try {
                Invoke-BridgeRetry { & "Get-$Noun" -Identity $Identity } | Out-Null
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
                $part = if ($Noun -eq "EOPProtectionPolicyRule") { "Exchange Online Protection" } else { "Defender for Office 365" }
                throw "Rule '$Identity' does not exist: in the Microsoft Defender portal (Email & collaboration > Policies & rules > Threat policies > Preset security policies), open Manage protection settings of the preset and apply its $part part to recipients, which creates the rule, then apply again. The preset can show as on while this part has no rule"
            }
            & $write
            & $read
        }
        "Update" { & $write; & $read }
        "Delete" {
            # The rule stays: only the portal can recreate it with its policies.
            try { & "Disable-$Noun" -Identity $Identity } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
}
