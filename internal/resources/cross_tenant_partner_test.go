package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var capabilityType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"name": tftypes.String, "allowed": tftypes.Bool,
	"included_groups": tftypes.Set{ElementType: tftypes.String}, "excluded_groups": tftypes.Set{ElementType: tftypes.String},
	"included_users": tftypes.Set{ElementType: tftypes.String}, "excluded_users": tftypes.Set{ElementType: tftypes.String},
}}

func capabilityValue(name string, allowed bool, includedGroups ...string) tftypes.Value {
	set := func(items ...string) tftypes.Value {
		if len(items) == 0 {
			return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil)
		}
		vals := make([]tftypes.Value, len(items))
		for i, s := range items {
			vals[i] = str(s)
		}
		return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, vals)
	}
	return tftypes.NewValue(capabilityType, map[string]tftypes.Value{
		"name": str(name), "allowed": tftypes.NewValue(tftypes.Bool, allowed),
		"included_groups": set(includedGroups...), "excluded_groups": set(), "included_users": set(), "excluded_users": set(),
	})
}

func capabilities(items ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(tftypes.Set{ElementType: capabilityType}, items)
}

// TestCrossTenantPartnerSchema pins the schema: the partner is the identity
// (a change replaces the resource), the capabilities are a set of nested
// objects restricted to the seven Exchange capabilities.
func TestCrossTenantPartnerSchema(t *testing.T) {
	var resp resource.SchemaResponse
	NewCrossTenantPartnerResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	partner, ok := resp.Schema.Attributes["partner_tenant_id"].(schema.StringAttribute)
	if !ok || !partner.Required || len(partner.PlanModifiers) == 0 {
		t.Errorf("partner_tenant_id = %#v, want required with a replace plan modifier", resp.Schema.Attributes["partner_tenant_id"])
	}
	caps, ok := resp.Schema.Attributes["capabilities"].(schema.SetNestedAttribute)
	if !ok || !caps.Required {
		t.Fatalf("capabilities = %T, want a required SetNestedAttribute", resp.Schema.Attributes["capabilities"])
	}
	for _, name := range []string{"name", "allowed", "included_groups", "excluded_groups", "included_users", "excluded_users"} {
		if _, ok := caps.NestedObject.Attributes[name]; !ok {
			t.Errorf("capabilities has no %s", name)
		}
	}
	name := caps.NestedObject.Attributes["name"].(schema.StringAttribute)
	if len(name.Validators) == 0 || !strings.Contains(name.MarkdownDescription, "crossTenantCalendarSharingFreeBusyReviewer") {
		t.Errorf("name: %d validators, description %q", len(name.Validators), name.MarkdownDescription)
	}
	if err := resp.Schema.ValidateImplementation(context.Background()); err.HasError() {
		t.Errorf("invalid schema: %v", err)
	}
}

func TestCrossTenantPartnerValidate(t *testing.T) {
	r := NewCrossTenantPartnerResource().(resource.ResourceWithValidateConfig)
	partner := str("11111111-2222-3333-4444-555555555555")

	diags := validateResource(t, r, map[string]tftypes.Value{"partner_tenant_id": partner,
		"capabilities": capabilities(capabilityValue("crossTenantMailTipsAll", true), capabilityValue("crossTenantMailTipsAll", false))})
	if !hasError(diags, "Duplicate capability") {
		t.Errorf("duplicate name: %v", diags)
	}
	diags = validateResource(t, r, map[string]tftypes.Value{"partner_tenant_id": partner,
		"capabilities": capabilities(capabilityValue("crossTenantMailTipsAll", true, "finance@contoso.com"))})
	if !hasError(diags, "Invalid object ID") {
		t.Errorf("group by address: %v", diags)
	}
	diags = validateResource(t, r, map[string]tftypes.Value{"partner_tenant_id": str("contoso.com"),
		"capabilities": capabilities(capabilityValue("crossTenantMailTipsAll", true))})
	if !hasError(diags, "Invalid partner_tenant_id") {
		t.Errorf("partner by domain: %v", diags)
	}
	diags = validateResource(t, r, map[string]tftypes.Value{"partner_tenant_id": partner,
		"capabilities": capabilities(capabilityValue("crossTenantCalendarAvailabilityBasic", true, "AAAAAAAA-0000-0000-0000-000000000001"))})
	if diags.HasError() {
		t.Errorf("valid configuration: %v", diags)
	}
}

// The partner tenant ID is stored in lowercase, whatever the configuration or
// Graph uses, and capability sets left out of the configuration stay null.
func TestCrossTenantPartnerFromAPI(t *testing.T) {
	m, err := crossTenantPartnerFromAPI(context.Background(), crossTenantPartnerModel{}, []byte(
		`{"PartnerTenantId":"11111111-2222-3333-4444-555555555555","Capabilities":[{"name":"crossTenantMailTipsAll",`+
			`"allowed":true,"includedGroups":["aaaaaaaa-0000-0000-0000-000000000001"],"excludedGroups":[],"includedUsers":[],"excludedUsers":[]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID.ValueString() != "11111111-2222-3333-4444-555555555555" || m.PartnerTenantID.ValueString() != m.ID.ValueString() {
		t.Errorf("id = %s, partner = %s", m.ID, m.PartnerTenantID)
	}
	if got := m.Capabilities.String(); !strings.Contains(got, `"included_groups":["aaaaaaaa-0000-0000-0000-000000000001"]`) ||
		!strings.Contains(got, `"excluded_users":<null>`) {
		t.Errorf("capabilities = %s", got)
	}
}
