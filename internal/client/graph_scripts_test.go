package client

import (
	"strings"
	"testing"
)

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

// TestCrossTenantPartnerScript guards cross_tenant_partner.ps1: only the
// Exchange capabilities of the partner are read, changed and removed (never
// the profile card), an Exchange capability outside the configuration is
// removed, an "All" scope uses the resource type Graph stores, Create refuses
// a partner that already has Exchange capabilities, and a missing Entra role is
// an error instead of an empty partner.
func TestCrossTenantPartnerScript(t *testing.T) {
	out := runStub(t, "cross_tenant_partner.ps1", "cross_tenant_partner.ps1")
	expectLines(t, out,
		`get: {"PartnerTenantId":"11111111-2222-3333-4444-555555555555","Capabilities":[{"name":"crossTenantMailTipsAll","allowed":true,`+
			`"includedGroups":[],"excludedGroups":[],"includedUsers":[],"excludedUsers":[]}]}`,
		"CrossTenantPartner Create failed: Partner 11111111-2222-3333-4444-555555555555 already has Exchange capabilities "+
			"(crossTenantMailTipsAll): import it",
		"create-existing calls: GET list\n",
		`DELETE crossTenantMailTipsAll`,
		`DELETE crossTenantMailTipsLimited`,
		`POST list {"@odata.type":"#microsoft.graph.crossTenantCalendarAvailabilityBasic","inboundAccess":{"isAllowed":true,`+
			`"resourceScopes":{"included":[{"resourceId":"aaaaaaaa-0000-0000-0000-000000000001","resourceType":"group"}],"excluded":[]}}}`,
		`POST list {"@odata.type":"#microsoft.graph.crossTenantCalendarSharingFreeBusySimple","inboundAccess":{"isAllowed":true,`+
			`"resourceScopes":{"included":[{"resourceId":"All","resourceType":"group"}],`+
			`"excluded":[{"resourceId":"bbbbbbbb-0000-0000-0000-000000000002","resourceType":"user"}]}}}`,
		`"name":"crossTenantCalendarSharingFreeBusySimple","allowed":true,"includedGroups":[],"excludedGroups":[],`+
			`"includedUsers":[],"excludedUsers":["bbbbbbbb-0000-0000-0000-000000000002"]`,
		`patch calls: GET list; PATCH crossTenantCalendarAvailabilityBasic {"@odata.type":"#microsoft.graph.crossTenantCalendarAvailabilityBasic","inboundAccess":{"isAllowed":false,`,
		"left: crossTenantOpenProfileCard",
		"gone: \n",
		"norole calls: GET list; GET crossTenantMailTipsAll",
		"CrossTenantPartner Get failed: The app needs the Microsoft Entra role Exchange Administrator to manage the "+
			"cross-tenant capabilities of partner 11111111-2222-3333-4444-555555555555 (Graph 403 forbidden",
	)
	// Both reads without the role (with and without capabilities in the state) fail.
	if n := strings.Count(out, "CrossTenantPartner Get failed: The app needs the Microsoft Entra role"); n != 2 {
		t.Errorf("%d role errors, want 2:\n%s", n, out)
	}
	for _, forbidden := range []string{"DELETE crossTenantOpenProfileCard", "PATCH crossTenantOpenProfileCard",
		"PATCH crossTenantCalendarSharingFreeBusySimple"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("unexpected call %q in:\n%s", forbidden, out)
		}
	}
}

// TestListCrossTenantPartners guards list.ps1 for cross_tenant_partner: only
// partners with an Exchange capability are listed (in lowercase, all result
// pages), and without Policy.Read.All the error names the permission.
func TestListCrossTenantPartners(t *testing.T) {
	out := runStub(t, "list_cross_tenant.ps1", "list.ps1")
	expectLines(t, out,
		`partners: ["aaaaaaaa-0000-0000-0000-000000000001"]`,
		"Listing cross-tenant partners needs the Microsoft Graph application permission Policy.Read.All",
	)
}
