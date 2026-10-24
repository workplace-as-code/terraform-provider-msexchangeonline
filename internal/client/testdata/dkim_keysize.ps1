# dkim_keysize.ps1 - runs Get-DkimKeySize of dkim_signing_config.ps1 under StrictMode on
# simulated DKIM configurations (run by dkim_test.go): Get-DkimSigningConfig has no
# KeySize property, only one per selector.
param([string]$Script)
Set-StrictMode -Version Latest
. ([scriptblock]::Create((Get-Content (Join-Path (Split-Path $Script) "common.ps1") -Raw)))
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Script, [ref]$null, [ref]$null)
$f = $ast.FindAll({ $args[0] -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $args[0].Name -eq "Get-DkimKeySize" }, $true) | Select-Object -First 1
. ([scriptblock]::Create($f.Extent.Text))
"no KeySize property: " + (Get-DkimKeySize ([pscustomobject]@{ Selector1KeySize = 2048; Selector2KeySize = 1024; SelectorAfterRotateOnDate = "Selector1" }))
"after rotation to selector2: " + (Get-DkimKeySize ([pscustomobject]@{ Selector1KeySize = 1024; Selector2KeySize = 2048; SelectorAfterRotateOnDate = "Selector2" }))
"nothing reported: " + (Get-DkimKeySize ([pscustomobject]@{ Domain = "x" }))
