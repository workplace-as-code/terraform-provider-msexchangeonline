package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
	// Exchange has no Set-ManagementRole: a new description replaces the role.
	description := resp.Schema.Attributes["description"].(schema.StringAttribute)
	replaces := false
	for _, m := range description.PlanModifiers {
		replaces = replaces || strings.Contains(m.Description(context.Background()), "recreate")
	}
	if !replaces {
		t.Error("description must replace the role when it changes: Exchange cannot change it")
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

func paramsMap(cmdlet string, params ...string) tftypes.Value {
	set := make([]tftypes.Value, len(params))
	for i, p := range params {
		set[i] = tftypes.NewValue(tftypes.String, p)
	}
	elem := tftypes.Set{ElementType: tftypes.String}
	return tftypes.NewValue(tftypes.Map{ElementType: elem}, map[string]tftypes.Value{cmdlet: tftypes.NewValue(elem, set)})
}

func cmdletSet(items ...string) tftypes.Value {
	vals := make([]tftypes.Value, len(items))
	for i, s := range items {
		vals[i] = tftypes.NewValue(tftypes.String, s)
	}
	return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, vals)
}

// TestManagementRoleParametersNeedTheirCmdlet: a cmdlet of cmdlet_parameters
// must be kept by cmdlets, or the script ignores it and the apply fails.
func TestManagementRoleParametersNeedTheirCmdlet(t *testing.T) {
	r := &managementRoleResource{}
	d := validateResource(t, r, map[string]tftypes.Value{
		"name": str("r"), "parent": str("Mail Recipients"), "cmdlets": cmdletSet("Get-Mailbox"),
		"cmdlet_parameters": paramsMap("Set-User", "Identity"),
	})
	if !hasError(d, "cmdlet_parameters") {
		t.Errorf("a cmdlet of cmdlet_parameters missing from cmdlets must be refused, got %v", d)
	}
	d = validateResource(t, r, map[string]tftypes.Value{
		"name": str("r"), "parent": str("Mail Recipients"), "cmdlets": cmdletSet("set-user"),
		"cmdlet_parameters": paramsMap("Set-User", "Identity"),
	})
	if d.HasError() {
		t.Errorf("the case of the cmdlet does not matter, got %v", d)
	}
}

// TestManagementRoleKeepsEntriesEqualToTheParent: Exchange only reports the
// entries whose parameters differ from the parent's; an entry configured with
// all of them stays in the state.
func TestManagementRoleKeepsEntriesEqualToTheParent(t *testing.T) {
	prior := managementRoleModel{
		Name:    types.StringValue("TF-CI-Role"),
		Parent:  types.StringValue("Mail Recipients"),
		Cmdlets: stringSet("Get-User"),
		CmdletParameters: types.MapValueMust(cmdletParametersType, map[string]attr.Value{
			"Get-User": stringSet("Identity"),
		}),
	}
	m, err := managementRoleFromAPI(prior, []byte(`{"Name":"TF-CI-Role","Parent":"Mail Recipients",`+
		`"Cmdlets":["Get-User"],"CmdletParameters":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !m.CmdletParameters.Equal(prior.CmdletParameters) {
		t.Errorf("got %v, want %v", m.CmdletParameters, prior.CmdletParameters)
	}
}
