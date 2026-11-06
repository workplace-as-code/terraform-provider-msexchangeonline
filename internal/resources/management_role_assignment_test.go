package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRoleAssignmentExactlyOneAssignee(t *testing.T) {
	r := &managementRoleAssignmentResource{}
	if d := validateResource(t, r, map[string]tftypes.Value{"role": str("Mail Recipients")}); !d.HasError() {
		t.Error("an assignment without role_group nor app must be refused")
	}
	d := validateResource(t, r, map[string]tftypes.Value{
		"role": str("Mail Recipients"), "role_group": str("Helpdesk"), "app": str("00000000-0000-0000-0000-000000000001"),
	})
	if !d.HasError() {
		t.Error("an assignment to both a role group and an app must be refused")
	}
	if d := validateResource(t, r, map[string]tftypes.Value{"role": str("Mail Recipients"), "role_group": str("Helpdesk")}); d.HasError() {
		t.Errorf("an assignment to a role group must be valid, got %v", d)
	}
}

func TestRoleAssignmentResourceScopeNeedsApp(t *testing.T) {
	r := &managementRoleAssignmentResource{}
	d := validateResource(t, r, map[string]tftypes.Value{
		"role": str("Mail Recipients"), "role_group": str("Helpdesk"), "custom_resource_scope": str("France"),
	})
	if !hasError(d, "custom_resource_scope") {
		t.Errorf("custom_resource_scope on a role group assignment must be refused, got %v", d)
	}
	d = validateResource(t, r, map[string]tftypes.Value{
		"role": str("Application Mail.Read"), "app": str("00000000-0000-0000-0000-000000000001"),
		"custom_resource_scope": str("France"),
	})
	if d.HasError() {
		t.Errorf("custom_resource_scope on an app assignment must be valid, got %v", d)
	}
}

func TestRoleAssignmentOneWriteScope(t *testing.T) {
	d := validateResource(t, &managementRoleAssignmentResource{}, map[string]tftypes.Value{
		"role": str("Mail Recipients"), "role_group": str("Helpdesk"),
		"custom_recipient_write_scope": str("France"), "recipient_group_scope": str("Helpdesk Group"),
	})
	if !hasError(d, "write scope") {
		t.Errorf("two write scopes must be refused, got %v", d)
	}
}

func TestRoleAssignmentFromAPI(t *testing.T) {
	prior := managementRoleAssignmentModel{
		Role:                             types.StringValue("mail recipients"),
		RoleGroup:                        types.StringValue("Helpdesk"),
		App:                              types.StringNull(),
		CustomRecipientWriteScope:        types.StringValue("France"),
		ExclusiveRecipientWriteScope:     types.StringNull(),
		CustomResourceScope:              types.StringNull(),
		RecipientAdministrativeUnitScope: types.StringNull(),
		RecipientGroupScope:              types.StringNull(),
	}
	m, err := managementRoleAssignmentFromAPI(prior, []byte(`{"Name":"Mail Recipients-Helpdesk","Role":"Mail Recipients",`+
		`"RoleGroup":"Helpdesk","App":"","CustomRecipientWriteScope":"France","RecipientRelativeWriteScope":"CustomRecipientScope",`+
		`"Enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID.ValueString() != "Mail Recipients-Helpdesk" || m.Name.ValueString() != "Mail Recipients-Helpdesk" {
		t.Errorf("id and name come from Exchange: %v %v", m.ID, m.Name)
	}
	if m.Role.ValueString() != "mail recipients" {
		t.Errorf("role keeps the configured case: %v", m.Role)
	}
	if !m.App.IsNull() || !m.ExclusiveRecipientWriteScope.IsNull() || !m.CustomResourceScope.IsNull() {
		t.Error("unset attributes stay null")
	}
	if !m.Enabled.ValueBool() || m.RecipientRelativeWriteScope.ValueString() != "CustomRecipientScope" {
		t.Errorf("enabled and relative scope come from Exchange: %v %v", m.Enabled, m.RecipientRelativeWriteScope)
	}
}
