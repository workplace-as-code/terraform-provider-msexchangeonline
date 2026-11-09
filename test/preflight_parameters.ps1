<#
.SYNOPSIS
    Checks that the parameter of every attribute exists on the cmdlet that
    writes it. Dot-sourced by test/preflight.ps1, once connected.
#>

# Test-PreflightParameters reads "<cmdlet> <parameter>" lines and returns the
# number of errors. A parameter in no role of the organization does not exist
# in Exchange Online: an error, unless the cmdlet belongs to an optional test
# that is off. A parameter that exists but that the app cannot use is a
# warning. A cmdlet the app cannot see was already reported by the cmdlet check.
function Test-PreflightParameters {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [scriptblock]$IsOptional = { param($Cmdlet) $false }
    )
    $canReadRoles = [bool](Get-Command Get-ManagementRoleEntry -ErrorAction SilentlyContinue)
    $commands = @{}
    $ok = 0
    $errors = 0
    foreach ($line in Get-Content $Path) {
        if (-not $line -or $line.StartsWith("#")) { continue }
        $cmdlet, $parameter = $line -split " ", 2
        if (-not $commands.ContainsKey($cmdlet)) { $commands[$cmdlet] = Get-Command $cmdlet -ErrorAction SilentlyContinue }
        $command = $commands[$cmdlet]
        if (-not $command) { continue }
        if ($command.Parameters.ContainsKey($parameter)) { $ok++; continue }
        $roles = @()
        if ($canReadRoles) {
            $roles = @(Get-ManagementRoleEntry "*\$cmdlet" -Parameters $parameter -ErrorAction SilentlyContinue |
                ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
        }
        if ($roles.Count -gt 0) {
            Write-Host "  ! $line is not available to the app (roles: $($roles -join ', '))" -ForegroundColor Yellow
        } elseif (-not $canReadRoles) {
            Write-Host "  ! $line is not available to the app (it cannot run Get-ManagementRoleEntry to say why)" -ForegroundColor Yellow
        } elseif (& $IsOptional $cmdlet) {
            Write-Host "  ! $line is in no role of the organization (optional test off)" -ForegroundColor Yellow
        } else {
            Write-Host "  x $line is in no role of the organization: Exchange Online has no such parameter" -ForegroundColor Red
            $errors++
        }
    }
    Write-Host "  ok: $ok"
    $errors
}
