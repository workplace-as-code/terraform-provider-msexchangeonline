package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestManagementRoleSchema(t *testing.T) {
	var resp resource.SchemaResponse
	NewManagementRoleResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if _, ok := resp.Schema.Attributes["cmdlets"].(schema.SetAttribute); !ok {
		t.Error("cmdlets must be a set: Exchange returns the cmdlets in its own order")
	}
	if _, ok := resp.Schema.Attributes["cmdlet_parameters"].(schema.MapAttribute); !ok {
		t.Error("cmdlet_parameters must be a map of cmdlet to parameters")
	}
	for _, name := range []string{"name", "parent"} {
		a := resp.Schema.Attributes[name].(schema.StringAttribute)
		if !a.Required || len(a.PlanModifiers) == 0 {
			t.Errorf("%s must be required and replace the role when it changes", name)
		}
	}
}

func stringSet(items ...string) types.Set {
	elems := make([]attr.Value, len(items))
	for i, s := range items {
		elems[i] = types.StringValue(s)
	}
	return types.SetValueMust(types.StringType, elems)
}

// TestManagementRoleKeepsTheConfiguredCase: Exchange returns "Get-Mailbox"
// for a configured "get-mailbox"; the state keeps the configured spelling so
// that the plan shows no change.
func TestManagementRoleKeepsTheConfiguredCase(t *testing.T) {
	prior := managementRoleModel{
		Name:             types.StringValue("TF-CI-Role"),
		Parent:           types.StringValue("mail recipients"),
		Cmdlets:          stringSet("get-mailbox", "Set-User"),
		CmdletParameters: types.MapNull(types.SetType{ElemType: types.StringType}),
	}
	out := []byte(`{"Name":"TF-CI-Role","Parent":"Mail Recipients","Description":"",` +
		`"Cmdlets":["Get-Mailbox","Set-User"],"CmdletParameters":{}}`)
	m, err := managementRoleFromAPI(prior, out)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Cmdlets.Equal(prior.Cmdlets) {
		t.Errorf("cmdlets: got %v, want %v", m.Cmdlets, prior.Cmdlets)
	}
	if m.Parent.ValueString() != "mail recipients" {
		t.Errorf("parent: got %v", m.Parent)
	}
	if !m.CmdletParameters.IsNull() {
		t.Errorf("an unset cmdlet_parameters must stay null when no entry is restricted, got %v", m.CmdletParameters)
	}
	if m.ID.ValueString() != "TF-CI-Role" {
		t.Errorf("id: got %v", m.ID)
	}
}

// TestManagementRoleReadsRestrictedParameters: a parameter restriction made
// outside Terraform shows in the state.
func TestManagementRoleReadsRestrictedParameters(t *testing.T) {
	prior := managementRoleModel{
		Name:             types.StringValue("TF-CI-Role"),
		Parent:           types.StringValue("Mail Recipients"),
		Cmdlets:          stringSet("Set-User"),
		CmdletParameters: types.MapNull(types.SetType{ElemType: types.StringType}),
	}
	out := []byte(`{"Name":"TF-CI-Role","Parent":"Mail Recipients","Cmdlets":["Set-User"],` +
		`"CmdletParameters":{"Set-User":["Identity","Phone"]}}`)
	m, err := managementRoleFromAPI(prior, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.CmdletParameters.Elements()) != 1 {
		t.Fatalf("cmdlet_parameters: got %v", m.CmdletParameters)
	}
}
