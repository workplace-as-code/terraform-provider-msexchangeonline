package resources

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestNoMembersField guards the rule that role group members belong to the
// identity management process: no resource reads or writes them, so a member
// added outside Terraform never shows as a change.
func TestNoMembersField(t *testing.T) {
	for _, spec := range objectSpecs {
		for _, f := range spec.fields {
			// ManagedBy says who can change the members: membership governance
			// belongs to the IAM process too, and Exchange returns it as names.
			if f.param == "Members" || f.name == "members" || f.param == "ManagedBy" {
				t.Errorf("%s: %s must not be managed", spec.typeName, f.name)
			}
		}
	}
}

// TestRBACSpecs checks the resources of the Exchange RBAC delegation.
func TestRBACSpecs(t *testing.T) {
	want := map[string][]string{
		"management_scope": {"recipient_restriction_filter", "recipient_root", "exclusive"},
		"role_group":       {"display_name", "description"},
	}
	for typeName, attrs := range want {
		spec, ok := findSpec(typeName)
		if !ok {
			t.Errorf("no spec for %s", typeName)
			continue
		}
		have := map[string]field{}
		for _, f := range spec.fields {
			have[f.name] = f
		}
		for _, a := range attrs {
			if _, ok := have[a]; !ok {
				t.Errorf("%s: missing attribute %s", typeName, a)
			}
		}
	}
	if spec, ok := findSpec("management_scope"); ok {
		for _, f := range spec.fields {
			if f.name == "exclusive" && !f.forceNew {
				t.Error("management_scope.exclusive can only be set by New-ManagementScope: it must replace the scope")
			}
			if f.name == "recipient_restriction_filter" && f.kind != filterField {
				t.Error("management_scope.recipient_restriction_filter is rewritten by Exchange: it must be a filterField")
			}
		}
	}
}

func findSpec(typeName string) (objectSpec, bool) {
	for _, s := range objectSpecs {
		if s.typeName == typeName {
			return s, true
		}
	}
	return objectSpec{}, false
}

// TestManagementScopeReadsRecipientFilter: the filter is written with
// -RecipientRestrictionFilter but Get-ManagementScope returns it as
// RecipientFilter; reading the parameter name would always give "" and hide
// any change made outside Terraform.
func TestManagementScopeReadsRecipientFilter(t *testing.T) {
	spec, _ := findSpec("management_scope")
	r := newObjectResource(spec)().(*objectResource)
	props := r.params("Get", "TF-CI-Scope", nil)["Properties"]
	raw, _ := base64.StdEncoding.DecodeString(props)
	if !strings.Contains(string(raw), `"RecipientFilter"`) || strings.Contains(string(raw), `"RecipientRestrictionFilter"`) {
		t.Errorf("Properties must ask for RecipientFilter, got %s", raw)
	}
	got := r.filtersOf([]byte(`{"Name":"TF-CI-Scope","RecipientFilter":"CustomAttribute15 -eq 'X'"}`))
	if got["recipient_restriction_filter"] != "CustomAttribute15 -eq 'X'" {
		t.Errorf("filter read back: %q", got["recipient_restriction_filter"])
	}
}

// TestForceNewBoolReplaces: a bool field that only New- accepts must replace
// the object when it changes, or the update sends nothing and Terraform
// reports an inconsistent result.
func TestForceNewBoolReplaces(t *testing.T) {
	spec, _ := findSpec("management_scope")
	var resp resource.SchemaResponse
	newObjectResource(spec)().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	a, ok := resp.Schema.Attributes["exclusive"].(schema.BoolAttribute)
	if !ok || len(a.PlanModifiers) < 2 {
		t.Errorf("exclusive must have a replace plan modifier, got %+v", a.PlanModifiers)
	}
}
