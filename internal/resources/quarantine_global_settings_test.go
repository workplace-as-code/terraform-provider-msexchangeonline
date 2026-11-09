package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// quarantineLanguagesValue builds a languages value from [language, disclaimer] pairs.
func quarantineLanguagesValue(pairs ...[2]string) tftypes.Value {
	object := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"language": tftypes.String, "sender_name": tftypes.String, "subject": tftypes.String, "disclaimer": tftypes.String,
	}}
	elems := make([]tftypes.Value, 0, len(pairs))
	for _, p := range pairs {
		elems = append(elems, tftypes.NewValue(object, map[string]tftypes.Value{
			"language":    tftypes.NewValue(tftypes.String, p[0]),
			"sender_name": tftypes.NewValue(tftypes.String, nil),
			"subject":     tftypes.NewValue(tftypes.String, nil),
			"disclaimer":  tftypes.NewValue(tftypes.String, p[1]),
		}))
	}
	return tftypes.NewValue(tftypes.Set{ElementType: object}, elems)
}

func TestQuarantineGlobalSettingsValidate(t *testing.T) {
	long := strings.Repeat("x", 201)
	cases := []struct {
		name  string
		pairs [][2]string
		want  string
	}{
		{"three languages", [][2]string{{"English", "a"}, {"French", "b"}, {"German", "c"}}, ""},
		{"four languages", [][2]string{{"English", "a"}, {"French", "b"}, {"German", "c"}, {"Italian", "d"}}, "at most 3"},
		{"duplicate language", [][2]string{{"French", "a"}, {"french", "b"}}, "more than once"},
		{"long disclaimer", [][2]string{{"French", long}}, "200 characters"},
		{"unknown language", [][2]string{{"Klingon", "a"}}, "Klingon"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := validateResource(t, &quarantineGlobalSettingsResource{}, map[string]tftypes.Value{
				"languages": quarantineLanguagesValue(c.pairs...),
			})
			if c.want == "" {
				if diags.HasError() {
					t.Fatalf("unexpected error: %v", diags)
				}
				return
			}
			if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), c.want) {
				t.Fatalf("want an error with %q, got %v", c.want, diags)
			}
		})
	}
}

func TestQuarantineGlobalSettingsFrequencyValues(t *testing.T) {
	var resp resource.SchemaResponse
	(&quarantineGlobalSettingsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	a := resp.Schema.Attributes["notification_frequency"].(schema.StringAttribute)
	for value, valid := range map[string]bool{"04:00:00": true, "1.00:00:00": true, "7.00:00:00": true, "2.00:00:00": false} {
		var out validator.StringResponse
		for _, v := range a.Validators {
			v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value)}, &out)
		}
		if out.Diagnostics.HasError() == valid {
			t.Errorf("%s: valid=%v, diagnostics %v", value, valid, out.Diagnostics)
		}
	}
}

func TestQuarantineGlobalSettingsFromAPI(t *testing.T) {
	m, err := quarantineGlobalSettingsFromAPI([]byte(`{"EndUserSpamNotificationFrequency":"1.00:00:00","EndUserSpamNotificationCustomFromAddress":"",` +
		`"OrganizationBrandingEnabled":true,"Languages":[{"Language":"French","SenderName":"Détails","Subject":"","Disclaimer":"d'assistance"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID.ValueString() != "default" || m.NotificationFrequency.ValueString() != "1.00:00:00" || !m.OrganizationBrandingEnabled.ValueBool() {
		t.Errorf("unexpected model: %+v", m)
	}
	elems := m.Languages.Elements()
	if len(elems) != 1 {
		t.Fatalf("want one language, got %d", len(elems))
	}
	a := elems[0].(types.Object).Attributes()
	if a["sender_name"].(types.String).ValueString() != "Détails" || !a["subject"].(types.String).IsNull() ||
		a["disclaimer"].(types.String).ValueString() != "d'assistance" {
		t.Errorf("unexpected language: %v", a)
	}
}
