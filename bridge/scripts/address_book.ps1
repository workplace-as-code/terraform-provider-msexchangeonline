# address_book.ps1 - exo_object.ps1 for address lists, offline address books
# and address book policies, called with the same parameters. Exchange
# returns references to address lists, GALs and offline address books as
# identities ("\Sales", "\Sites\Paris"): they go back to Terraform as names,
# as the configuration writes them. An address list also gets its Container,
# the name of its parent list, from its identity.

param(
    [string]$TenantId              = "",
    [string]$ClientId              = "",
    [string]$CertificateThumbprint = "",
    [string]$CertificatePath       = "",
    [string]$CertificatePassword   = "",
    [string]$Environment           = "global",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("AddressList","OfflineAddressBook","AddressBookPolicy")] [string]$Noun,
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Get-BridgeLeafName returns the last part of an identity: "\Sites\Paris" gives "Paris".
function Get-BridgeLeafName([string]$Value) {
    @($Value -split '\\' | Where-Object { $_ })[-1]
}

# The identity of an address list gives its container.
if ($Noun -eq "AddressList" -and $Properties) {
    $props = @(ConvertFrom-Json -InputObject ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Properties)))) + "Identity"
    $PSBoundParameters["Properties"] = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((ConvertTo-Json -Compress -InputObject @($props))))
}

$global:LASTEXITCODE = 0
$output = @(& "$PSScriptRoot/exo_object.ps1" @PSBoundParameters)
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$json = ($output -join "`n").Trim()
# Nothing (not found) or "{}" (deleted): pass it on as is.
if (-not $json -or $json -eq "{}") { $json; exit 0 }

$object = ConvertFrom-Json -InputObject $json
foreach ($p in "GlobalAddressList", "OfflineAddressBook", "RoomList") {
    if ($null -ne $object.PSObject.Properties[$p] -and $object.$p) { $object.$p = Get-BridgeLeafName $object.$p }
}
if ($null -ne $object.PSObject.Properties["AddressLists"]) {
    $object.AddressLists = @($object.AddressLists | Where-Object { $_ } | ForEach-Object { Get-BridgeLeafName $_ })
}
if ($null -ne $object.PSObject.Properties["Identity"]) {
    $parts = @("$($object.Identity)" -split '\\' | Where-Object { $_ })
    $container = if ($parts.Count -gt 1) { $parts[-2] } else { "" }
    $object | Add-Member -NotePropertyName Container -NotePropertyValue $container -Force
    $object.PSObject.Properties.Remove("Identity")
}
ConvertTo-Json -Compress -Depth 4 -InputObject $object
