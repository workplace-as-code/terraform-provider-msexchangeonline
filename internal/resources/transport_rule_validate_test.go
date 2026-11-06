package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// validateTransportRule runs ValidateConfig on a configuration made of the
// given attributes; every other attribute is null.
func validateTransportRule(t *testing.T, values map[string]tftypes.Value) diag.Diagnostics {
	t.Helper()
	return validateResource(t, &transportRuleResource{}, values)
}

// validateResource runs ValidateConfig (and the resource's config validators)
// on a configuration made of the given attributes; every other attribute is null.
func validateResource(t *testing.T, r resource.ResourceWithValidateConfig, values map[string]tftypes.Value) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()
	var sresp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, at := range typ.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	for name, v := range values {
		if _, ok := typ.AttributeTypes[name]; !ok {
			t.Fatalf("attribute %q is not in the schema", name)
		}
		vals[name] = v
	}
	cfg := tfsdk.Config{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, vals)}
	var resp resource.ValidateConfigResponse
	if cv, ok := r.(resource.ResourceWithConfigValidators); ok {
		for _, v := range cv.ConfigValidators(ctx) {
			v.ValidateResource(ctx, resource.ValidateConfigRequest{Config: cfg}, &resp)
		}
	}
	r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: cfg}, &resp)
	return resp.Diagnostics
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func strList(items ...string) tftypes.Value {
	vals := make([]tftypes.Value, len(items))
	for i, s := range items {
		vals[i] = str(s)
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, vals)
}

func hasError(diags diag.Diagnostics, summary string) bool {
	for _, d := range diags.Errors() {
		if strings.Contains(d.Summary(), summary) {
			return true
		}
	}
	return false
}

func TestTransportRuleNoAction(t *testing.T) {
	diags := validateTransportRule(t, map[string]tftypes.Value{
		"name":             str("r"),
		"subject_contains": strList("secret"),
	})
	if !hasError(diags, "Missing action") {
		t.Errorf("expected a Missing action error, got %v", diags)
	}
}

func TestTransportRuleCustomizationWithoutTemplate(t *testing.T) {
	diags := validateTransportRule(t, map[string]tftypes.Value{
		"name":          str("r"),
		"sent_to_scope": str("NotInOrganization"),
		"action":        str("Quarantine"),
		"apply_rights_protection_customization_template": str("OME Configuration"),
	})
	if !hasError(diags, "Missing apply_rights_protection_template") {
		t.Errorf("expected a Missing apply_rights_protection_template error, got %v", diags)
	}
}

func TestTransportRuleEncryptOnly(t *testing.T) {
	diags := validateTransportRule(t, map[string]tftypes.Value{
		"name":                             str("r"),
		"sent_to_scope":                    str("NotInOrganization"),
		"apply_rights_protection_template": str("Encrypt"),
	})
	if diags.HasError() {
		t.Errorf("an encryption rule with a scope condition must be valid, got %v", diags)
	}
}

func TestTransportRuleNewConditionCounts(t *testing.T) {
	diags := validateTransportRule(t, map[string]tftypes.Value{
		"name":                str("r"),
		"action":              str("Quarantine"),
		"recipient_domain_is": strList("contoso.com"),
	})
	if diags.HasError() {
		t.Errorf("recipient_domain_is alone must count as a condition, got %v", diags)
	}
}

func TestTransportRuleExceptionIsNotACondition(t *testing.T) {
	diags := validateTransportRule(t, map[string]tftypes.Value{
		"name":                          str("r"),
		"action":                        str("Quarantine"),
		"except_if_recipient_domain_is": strList("contoso.com"),
	})
	if !hasError(diags, "Transport rule without condition") {
		t.Errorf("an exception alone must not count as a condition, got %v", diags)
	}
}

func TestApplyTransportRuleResponseEncryption(t *testing.T) {
	m := transportRuleModel{
		Action:                        types.StringNull(),
		FromScope:                     types.StringNull(),
		ApplyRightsProtectionTemplate: types.StringNull(),
		ApplyRightsProtectionCustomizationTemplate: types.StringNull(),
		RecipientDomainIs:                          types.ListNull(types.StringType),
		SenderDomainIs:                             types.ListNull(types.StringType),
		SubjectOrBodyContainsWords:                 types.ListNull(types.StringType),
		ExceptIfRecipientDomainIs:                  types.ListNull(types.StringType),
	}
	applyTransportRuleResponse(&m, transportRuleAPIResponse{
		Identity: "r", Name: "r", SentToScope: "NotInOrganization",
		ApplyRightsProtectionTemplate: "Encrypt", RecipientDomainIs: []string{"contoso.com"},
	})
	if !m.Action.IsNull() || !m.FromScope.IsNull() || !m.ApplyRightsProtectionCustomizationTemplate.IsNull() {
		t.Error("unset strings must stay null when Exchange has no value")
	}
	if !m.SenderDomainIs.IsNull() || !m.SubjectOrBodyContainsWords.IsNull() || !m.ExceptIfRecipientDomainIs.IsNull() {
		t.Error("unset lists must stay null when Exchange has no values")
	}
	if m.SentToScope.ValueString() != "NotInOrganization" || m.ApplyRightsProtectionTemplate.ValueString() != "Encrypt" {
		t.Errorf("values from Exchange must reach the state: %v %v", m.SentToScope, m.ApplyRightsProtectionTemplate)
	}
	if len(m.RecipientDomainIs.Elements()) != 1 {
		t.Error("recipient_domain_is must hold the domain from Exchange")
	}
}
