# graph_invoke.ps1 - runs Invoke-BridgeGraph and Get-BridgeGraphToken from
# bridge/scripts/common.ps1 against a fake Invoke-WebRequest (run by
# graph_scripts_test.go): throttling is retried, errors are not, and a request
# without an answer is sent again unless it is a POST.
param([string]$Script)
$ErrorActionPreference = "Stop"
. $Script

$global:Responses = [System.Collections.Generic.Queue[object]]::new()
$global:Calls = [System.Collections.Generic.List[string]]::new()
function global:Invoke-WebRequest {
    param($Method, $Uri, $Headers, $Body, $ContentType, [switch]$SkipHttpErrorCheck)
    $global:Calls.Add("$Method $Uri $($Headers.Authorization)")
    $next = $global:Responses.Dequeue()
    if ($next -is [string]) { throw $next }   # no answer: a network error
    $next
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

# No answer: GET and DELETE are sent again (a DELETE then finds the object
# gone), a POST is not.
$global:Calls.Clear()
$global:Responses.Enqueue("An error occurred while sending the request.")
$global:Responses.Enqueue((Response 200 '{"value":[]}'))
$r = Invoke-BridgeGraph -Method GET -Path "/v1.0/n"
"network-get: calls=$($global:Calls.Count) $(@($global:Calls)[1])"
$global:Calls.Clear()
$global:Responses.Enqueue("An error occurred while sending the request.")
try { Invoke-BridgeGraph -Method POST -Path "/v1.0/n" -Body @{ a = 1 } | Out-Null; "network-post: no error" } catch {
    "network-post: $($_.Exception.Message) calls=$($global:Calls.Count)"
}
$global:Calls.Clear()
$global:Responses.Enqueue("An error occurred while sending the request.")
$global:Responses.Enqueue((Response 404 '{"error":{"code":"NotFound","message":"gone"}}'))
$n = Invoke-BridgeGraph -Method DELETE -Path "/v1.0/n"
"network-delete: null=$($null -eq $n) calls=$($global:Calls.Count)"
$global:Responses.Enqueue((Response 404 '{"error":{"code":"NotFound","message":"gone"}}'))
try { Invoke-BridgeGraph -Method DELETE -Path "/v1.0/n" | Out-Null; "delete-404: no error" } catch { "delete-404: $($_.Exception.Message)" }

$global:BridgeGraph = @{ Token = "gt-1"; Environment = "gcchigh" }
try { Get-BridgeGraphToken | Out-Null; "gcchigh: no error" } catch { "gcchigh: $($_.Exception.Message)" }
