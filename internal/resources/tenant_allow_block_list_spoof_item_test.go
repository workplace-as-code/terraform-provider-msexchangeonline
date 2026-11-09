package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSpoofItemImportID(t *testing.T) {
	cases := []struct {
		id                     string
		spoofType, user, infra string
		ok                     bool
	}{
		{"External:contoso.com:192.0.2.1/24", "External", "contoso.com", "192.0.2.1/24", true},
		{"Internal:user@contoso.com:mail.contoso.com", "Internal", "user@contoso.com", "mail.contoso.com", true},
		{"External:contoso.com:host:25", "External", "contoso.com", "host:25", true},
		{"External:contoso.com", "", "", "", false},
		{"External::x", "", "", "", false},
		{"Sideways:contoso.com:x", "", "", "", false},
	}
	for _, c := range cases {
		spoofType, user, infra, ok := parseSpoofItemID(c.id)
		if ok != c.ok || spoofType != c.spoofType || user != c.user || infra != c.infra {
			t.Errorf("%q: got %q %q %q %v", c.id, spoofType, user, infra, ok)
		}
	}
}

func TestSpoofItemFromAPIKeepsConfiguredCase(t *testing.T) {
	prior := spoofItemModel{}
	prior.SpoofType = types.StringValue("external")
	prior.SpoofedUser = types.StringValue("CONTOSO.com")
	prior.SendingInfrastructure = types.StringValue("192.0.2.1/24")
	m, err := spoofItemFromAPI(prior, []byte(`{"SpoofType":"External","SpoofedUser":"contoso.com","SendingInfrastructure":"192.0.2.1/24","Action":"Block"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.SpoofedUser.ValueString() != "CONTOSO.com" || m.SpoofType.ValueString() != "external" {
		t.Errorf("the configured case must be kept: %+v", m)
	}
	if m.ID.ValueString() != "external:CONTOSO.com:192.0.2.1/24" || m.Action.ValueString() != "Block" {
		t.Errorf("unexpected model: %+v", m)
	}
}
