package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestServicePrincipalSchema(t *testing.T) {
	var resp resource.SchemaResponse
	NewServicePrincipalResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	for _, name := range []string{"app_id", "object_id"} {
		a, ok := resp.Schema.Attributes[name].(schema.StringAttribute)
		if !ok || !a.Required || len(a.PlanModifiers) == 0 {
			t.Errorf("%s must be required and replace the service principal when it changes", name)
		}
	}
	if a, ok := resp.Schema.Attributes["display_name"].(schema.StringAttribute); !ok || !a.Optional || !a.Computed {
		t.Error("display_name must be optional and computed")
	}
}

func TestServicePrincipalFromAPI(t *testing.T) {
	prior := servicePrincipalModel{AppId: types.StringValue("AAAAAAAA-0000-0000-0000-000000000001")}
	m, err := servicePrincipalFromAPI(prior, []byte(`{"AppId":"aaaaaaaa-0000-0000-0000-000000000001",`+
		`"ObjectId":"bbbbbbbb-0000-0000-0000-000000000002","DisplayName":"Mail sender"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.AppId.ValueString() != "AAAAAAAA-0000-0000-0000-000000000001" || m.ID.ValueString() != m.AppId.ValueString() {
		t.Errorf("app_id keeps the configured case and is the id: %v %v", m.AppId, m.ID)
	}
	if m.ObjectId.ValueString() != "bbbbbbbb-0000-0000-0000-000000000002" || m.DisplayName.ValueString() != "Mail sender" {
		t.Errorf("object_id and display_name come from Exchange: %v %v", m.ObjectId, m.DisplayName)
	}
}
