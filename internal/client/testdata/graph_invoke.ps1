# graph_invoke.ps1 - runs Invoke-BridgeGraph and Get-BridgeGraphToken from
# bridge/scripts/common.ps1 against a fake Invoke-WebRequest (run by
# graph_scripts_test.go): throttling is retried, errors are not.
param([string]$Script)
$ErrorActionPreference = "Stop"
. $Script

$global:Responses = [System.Collections.Generic.Queue[object]]::new()
$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Invoke-WebRequest {
    param($Method, $Uri, $Headers, $Body, $ContentType, [switch]$SkipHttpErrorCheck)
    $global:Calls.Add("$Method $Uri $($Headers.Authorization)")
    $global:Responses.Dequeue()
}
function global:Start-Sleep { param([int]$Seconds) $global:Calls.Add("sleep $Seconds") }
function Response([int]$Status, [string]$Content, [hashtable]$Headers = @{}) {
    [pscustomobject]@{ StatusCode = $Status; Content = $Content; Headers = $Headers }
}

$global:BridgeGraph = @{ Token = "gt-1"; Environment = "global" }
"token: $(Get-BridgeGraphToken)"

$global:Responses.Enqueue((Response 429 '{"error":{"code":"TooManyRequests","message":"slow down"}}' @{ "Retry-After" = @("0") }))
$global:Responses.Enqueue((Response 200 '{"value":[{"name":"crossTenantMailTipsAll"}]}'))
$r = Invoke-BridgeGraph -Method GET -Path "/v1.0/x"
"retry: $($r.value[0].name) calls=$($global:Calls -join ' | ')"

$global:Calls.Clear()
$global:Responses.Enqueue((Response 403 '{"error":{"code":"forbidden","message":"Your administrative role is not allowed"}}'))
try { Invoke-BridgeGraph -Method DELETE -Path "/v1.0/y" | Out-Null; "403: no error" } catch { "403: $($_.Exception.Message) calls=$($global:Calls.Count)" }

$global:Responses.Enqueue((Response 204 ''))
$n = Invoke-BridgeGraph -Method DELETE -Path "/v1.0/z"
"204: null=$($null -eq $n)"

$global:BridgeGraph = @{ Token = "gt-1"; Environment = "gcchigh" }
try { Get-BridgeGraphToken | Out-Null; "gcchigh: no error" } catch { "gcchigh: $($_.Exception.Message)" }
