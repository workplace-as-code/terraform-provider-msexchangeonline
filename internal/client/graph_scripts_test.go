package client

import "testing"

// TestInvokeBridgeGraphRetries guards Invoke-BridgeGraph (common.ps1): Graph
// throttling (429) is retried after Retry-After, a refusal (403) is not and
// keeps Graph's message, and 204 returns nothing. Get-BridgeGraphToken returns
// the token the provider obtained, and refuses clouds without the API.
func TestInvokeBridgeGraphRetries(t *testing.T) {
	out := runStub(t, "graph_invoke.ps1", "common.ps1")
	expectLines(t, out,
		"token: gt-1",
		"retry: crossTenantMailTipsAll calls=GET https://graph.microsoft.com/v1.0/x Bearer gt-1 | sleep 0 | "+
			"GET https://graph.microsoft.com/v1.0/x Bearer gt-1",
		"403: Graph 403 forbidden: Your administrative role is not allowed calls=1",
		"204: null=True",
		"gcchigh: Microsoft Graph cross-tenant access policy is only available in the global and gcc environments",
	)
}
