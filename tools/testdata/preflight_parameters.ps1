# preflight_parameters.ps1 - runs test/preflight_parameters.ps1 against fake
# Exchange cmdlets (run by tools_test.go).
param([string]$Script)
$ErrorActionPreference = "Stop"

$global:Cmdlets = @{
    "Set-Foo" = [pscustomobject]@{ Parameters = @{ Known = $true } }
}
function global:Get-Command {
    param([Parameter(Position = 0)] $Name)
    if ($Name -eq "Get-ManagementRoleEntry") { return [pscustomobject]@{ Parameters = @{} } }
    $global:Cmdlets[$Name]
}
function global:Get-ManagementRoleEntry {
    param([Parameter(Position = 0)] $Identity, $Parameters)
    if ($Parameters -eq "Hidden") { [pscustomobject]@{ Role = "Role A" } }
}

. $Script
$path = Join-Path ([System.IO.Path]::GetTempPath()) "parameters-$PID.txt"
@("# comment", "", "Set-Foo Known", "Set-Foo Hidden", "Set-Foo Gone", "Set-Missing Whatever") | Set-Content $path
try {
    "errors: $(Test-PreflightParameters -Path $path)"
    "optional errors: $(Test-PreflightParameters -Path $path -IsOptional { param($Cmdlet) $true })"
} finally {
    Remove-Item $path
}
