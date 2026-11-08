# not_found.ps1 - runs Test-BridgeNotFound from bridge/scripts/common.ps1 (run
# by rbac_scripts_test.go): a missing object is "gone", an unknown or
# unreachable server is an error.
param([string]$Script)
$ErrorActionPreference = "Stop"
. $Script

$messages = [ordered]@{
    object  = "The operation couldn't be performed because object 'TF-CI-Rule' couldn't be found on 'EX1.contoso.com'."
    graph   = "Capability crossTenantMailTipsAll does not exist."
    server  = 'Exchange server "EX99" was not found. Please make sure you typed the name correctly.'
    network = "The network path was not found."
    denied  = "Access is denied."
}
foreach ($k in $messages.Keys) { "${k}: $(Test-BridgeNotFound $messages[$k])" }
