package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestApplicationAccessPolicyScopeGroup(t *testing.T) {
	response := []byte(`{"Identity":"t\\a:S-1;g","AppId":"a","AccessRight":"RestrictAccess","Description":"","ScopeGroupEmail":"group@contoso.com"}`)

	// After an import the state holds no scope group: the email address stands for it.
	imported, err := applicationAccessPolicyFromAPI(applicationAccessPolicyModel{ID: types.StringValue("t\\a:S-1;g")}, response)
	if err != nil {
		t.Fatal(err)
	}
	if got := imported.PolicyScopeGroupID.ValueString(); got != "group@contoso.com" {
		t.Errorf("imported policy_scope_group_id = %q, want the group email", got)
	}

	// Otherwise the configured form is kept, even when it is not the email.
	configured, err := applicationAccessPolicyFromAPI(applicationAccessPolicyModel{PolicyScopeGroupID: types.StringValue("Sales Team")}, response)
	if err != nil {
		t.Fatal(err)
	}
	if got := configured.PolicyScopeGroupID.ValueString(); got != "Sales Team" {
		t.Errorf("configured policy_scope_group_id = %q, want it kept", got)
	}
}
