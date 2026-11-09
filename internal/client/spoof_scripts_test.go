package client

import "testing"

// TestSpoofItemScript guards tenant_allow_block_list_spoof_item.ps1: Get- is
// called without -Identity (it returns nothing with it), a pair that exists is
// never silently taken over, and an entry is found by its pair whatever the case.
func TestSpoofItemScript(t *testing.T) {
	out := runStub(t, "tenant_allow_block_list_spoof_item.ps1", "tenant_allow_block_list_spoof_item.ps1")
	expectLines(t, out,
		"absent: []",
		`create: {"SpoofType":"External","SpoofedUser":"contoso.com","SendingInfrastructure":"192.0.2.1/24","Action":"Allow"}`,
		`read: {"SpoofType":"External","SpoofedUser":"contoso.com","SendingInfrastructure":"192.0.2.1/24","Action":"Allow"}`,
		"already exists",
		"duplicate calls: []",
		`"Action":"Block"`,
		"update calls: Set guid-1 Block",
		"delete: {}",
		"delete again: {}",
	)
}

// TestListSpoofItems guards the spoof item branch of list.ps1: import IDs are
// "<spoof_type>:<spoofed_user>:<sending_infrastructure>".
func TestListSpoofItems(t *testing.T) {
	out := runStub(t, "list_spoof.ps1", "list.ps1")
	expectLines(t, out, `ids: ["External:contoso.com:192.0.2.1/24","Internal:bob@contoso.com:mail.contoso.com"]`)
}
