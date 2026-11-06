package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRoleAssignmentPolicySchema(t *testing.T) {
	var resp resource.SchemaResponse
	NewRoleAssignmentPolicyResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if a, ok := resp.Schema.Attributes["roles"].(schema.SetAttribute); !ok || !a.Required {
		t.Error("roles must be a required set")
	}
}

func TestRoleAssignmentPolicyFromAPI(t *testing.T) {
	prior := roleAssignmentPolicyModel{Name: types.StringValue("TF-CI-Policy"), Roles: stringSet("mybaseoptions")}
	m, err := roleAssignmentPolicyFromAPI(prior, []byte(`{"Name":"TF-CI-Policy","Description":"d","IsDefault":false,`+
		`"Roles":["MyBaseOptions"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Roles.Equal(prior.Roles) || m.ID.ValueString() != "TF-CI-Policy" || m.IsDefault.ValueBool() {
		t.Errorf("got %+v", m)
	}
}
