package resources

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// reportLanguagesValue builds a languages value with only the language set.
func reportLanguagesValue(languages ...string) tftypes.Value {
	attributes := map[string]tftypes.Type{"language": tftypes.String}
	for _, t := range reportSubmissionLanguageTexts() {
		attributes[t.name] = tftypes.String
	}
	object := tftypes.Object{AttributeTypes: attributes}
	elems := make([]tftypes.Value, 0, len(languages))
	for _, l := range languages {
		values := map[string]tftypes.Value{}
		for name := range attributes {
			values[name] = tftypes.NewValue(tftypes.String, nil)
		}
		values["language"] = tftypes.NewValue(tftypes.String, l)
		elems = append(elems, tftypes.NewValue(object, values))
	}
	return tftypes.NewValue(tftypes.Set{ElementType: object}, elems)
}

// reportLanguagesEmptyText builds one language with an empty text.
func reportLanguagesEmptyText(language string) tftypes.Value {
	set := reportLanguagesValue(language)
	var elements []tftypes.Value
	if err := set.As(&elements); err != nil {
		panic(err)
	}
	var values map[string]tftypes.Value
	if err := elements[0].As(&values); err != nil {
		panic(err)
	}
	values["pre_submit_junk_title"] = tftypes.NewValue(tftypes.String, "")
	return tftypes.NewValue(set.Type(), []tftypes.Value{tftypes.NewValue(elements[0].Type(), values)})
}

func TestReportSubmissionValidate(t *testing.T) {
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }
	boolean := func(b bool) tftypes.Value { return tftypes.NewValue(tftypes.Bool, b) }
	cases := []struct {
		name   string
		values map[string]tftypes.Value
		want   string
	}{
		{"third party without mailbox", map[string]tftypes.Value{
			"enable_third_party_address": boolean(true), "enable_report_to_microsoft": boolean(false),
		}, "reporting_mailbox"},
		{"third party and Microsoft", map[string]tftypes.Value{
			"enable_third_party_address": boolean(true), "enable_report_to_microsoft": boolean(true),
			"reporting_mailbox": str("a@contoso.com"),
		}, "enable_report_to_microsoft"},
		{"third party leaving Microsoft unset", map[string]tftypes.Value{
			"enable_third_party_address": boolean(true), "reporting_mailbox": str("a@contoso.com"),
		}, "enable_report_to_microsoft"},
		{"third party complete", map[string]tftypes.Value{
			"enable_third_party_address": boolean(true), "enable_report_to_microsoft": boolean(false),
			"reporting_mailbox": str("a@contoso.com"),
		}, ""},
		{"unknown language code", map[string]tftypes.Value{"languages": reportLanguagesValue("en-US")}, "en-US"},
		{"duplicate language", map[string]tftypes.Value{"languages": reportLanguagesValue("en", "EN")}, "more than once"},
		{"no language", map[string]tftypes.Value{"languages": reportLanguagesValue()}, "last language"},
		{"language in another case", map[string]tftypes.Value{"languages": reportLanguagesValue("EN")}, "EN"},
		{"empty text", map[string]tftypes.Value{"languages": reportLanguagesEmptyText("en")}, "leave it out"},
		{"two languages", map[string]tftypes.Value{"languages": reportLanguagesValue("en", "fr")}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := validateResource(t, &reportSubmissionPolicyResource{}, c.values)
			if c.want == "" {
				if diags.HasError() {
					t.Fatalf("unexpected error: %v", diags)
				}
				return
			}
			found := false
			for _, d := range diags.Errors() {
				found = found || strings.Contains(d.Detail(), c.want)
			}
			if !found {
				t.Fatalf("want an error with %q, got %v", c.want, diags)
			}
		})
	}
}

func TestReportSubmissionLanguageTexts(t *testing.T) {
	texts := reportSubmissionLanguageTexts()
	if len(texts) != 20 {
		t.Fatalf("want 20 texts, got %d", len(texts))
	}
	byName := map[string]string{}
	for _, x := range texts {
		byName[x.name] = x.param
	}
	for name, param := range map[string]string{
		"pre_submit_junk_title":            "MultiLanguagePreSubmitMessageTitleForJunk",
		"pre_submit_not_junk_message":      "MultiLanguagePreSubmitMessageForNotJunk",
		"post_submit_phishing_button_link": "MultiLanguagePostSubmitMessageButtonLinkForPhishing",
	} {
		if byName[name] != param {
			t.Errorf("%s: %q, want %q", name, byName[name], param)
		}
	}
	if _, ok := byName["post_submit_not_junk_title"]; ok {
		t.Error("Microsoft has no post-submit text for not junk")
	}
}

func TestReportSubmissionLanguagesRoundTrip(t *testing.T) {
	obj, err := reportSubmissionFromAPI([]byte(`{"EnableReportToMicrosoft":true,"NotificationSenderAddress":"","ReportingMailbox":"a@contoso.com",` +
		`"Languages":[{"Language":"fr","MultiLanguagePreSubmitMessageForJunk":"Détails","MultiLanguagePostSubmitMessageForJunk":""}]}`))
	if err != nil {
		t.Fatal(err)
	}
	a := obj.Attributes()
	if a["id"].(types.String).ValueString() != "default" || a["reporting_mailbox"].(types.String).ValueString() != "a@contoso.com" {
		t.Errorf("unexpected object: %v", a)
	}
	elems := a["languages"].(types.Set).Elements()
	if len(elems) != 1 {
		t.Fatalf("want one language, got %d", len(elems))
	}
	nulls := 0
	for name, v := range elems[0].(types.Object).Attributes() {
		s := v.(types.String)
		switch name {
		case "language":
			if s.ValueString() != "fr" {
				t.Errorf("language = %q", s.ValueString())
			}
		case "pre_submit_junk_message":
			if s.ValueString() != "Détails" {
				t.Errorf("message = %q", s.ValueString())
			}
		default:
			if !s.IsNull() {
				t.Errorf("%s must be null, got %q", name, s.ValueString())
			}
			nulls++
		}
	}
	if nulls != 19 {
		t.Errorf("want 19 null texts, got %d", nulls)
	}
}

// TestReportSubmissionExcludedParameters: the parameters Microsoft reserves or
// tells users not to use, and the Teams ones, are never sent or exposed.
func TestReportSubmissionExcludedParameters(t *testing.T) {
	excluded := regexp.MustCompile(`DisableUserSubmissionOptions|EnableCustomizedMsg|OnlyShowPhishingDisclaimer|` +
		`UserSubmissionOptions|AirInvestigationsEnabled|ReportChat|ChatMessage|\b(Pre|Post)SubmitMessage(Title)?For(Junk|NotJunk|Phishing)\b`)
	script, err := os.ReadFile("../../bridge/scripts/report_submission_policy.ps1")
	if err != nil {
		t.Fatal(err)
	}
	if m := excluded.FindString(regexp.MustCompile(`MultiLanguage\w+`).ReplaceAllString(string(script), "")); m != "" {
		t.Errorf("the script uses the excluded parameter %s", m)
	}
	var resp resource.SchemaResponse
	(&reportSubmissionPolicyResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	for _, f := range reportSubmissionFields {
		if excluded.MatchString(f.param) {
			t.Errorf("%s is excluded", f.param)
		}
		if _, ok := resp.Schema.Attributes[f.name]; !ok {
			t.Errorf("%s is not in the schema", f.name)
		}
	}
}

// A set element entirely unknown at validation must not crash the provider.
func TestReportSubmissionValidateUnknownElement(t *testing.T) {
	set := reportLanguagesValue("en")
	var elements []tftypes.Value
	if err := set.As(&elements); err != nil {
		t.Fatal(err)
	}
	value := tftypes.NewValue(set.Type(), []tftypes.Value{tftypes.NewValue(elements[0].Type(), tftypes.UnknownValue)})
	if diags := validateResource(t, &reportSubmissionPolicyResource{}, map[string]tftypes.Value{"languages": value}); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
}

// With no language in Exchange the state is null: the configuration terraform
// query generates must not hold the empty set that validation refuses.
func TestReportSubmissionNoLanguageIsNull(t *testing.T) {
	obj, err := reportSubmissionFromAPI([]byte(`{"ReportingMailbox":"","Languages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if languages := obj.Attributes()["languages"].(types.Set); !languages.IsNull() {
		t.Errorf("want a null set, got %v", languages)
	}
}

// An unmanaged mailbox is still compared with the one in the state on Update.
func TestReportSubmissionParamsExpectedMailbox(t *testing.T) {
	values := map[string]attr.Value{}
	for name, typ := range reportSubmissionAttrTypes() {
		switch at := typ.(type) {
		case types.SetType:
			values[name] = types.SetNull(at.ElemType)
		default:
			if typ == types.BoolType {
				values[name] = types.BoolNull()
			} else {
				values[name] = types.StringNull()
			}
		}
	}
	cfg := types.ObjectValueMust(reportSubmissionAttrTypes(), values)
	p := reportSubmissionParams(cfg, types.StringValue("alias@contoso.com"))
	if p["ExpectedMailbox"] != "alias@contoso.com" {
		t.Errorf("ExpectedMailbox = %q", p["ExpectedMailbox"])
	}
	if _, ok := p["ReportingMailbox"]; ok {
		t.Error("an unmanaged mailbox must not be sent")
	}
	values["reporting_mailbox"] = types.StringValue("a@contoso.com")
	p = reportSubmissionParams(types.ObjectValueMust(reportSubmissionAttrTypes(), values), types.StringValue("alias@contoso.com"))
	if p["ReportingMailbox"] != "a@contoso.com" {
		t.Errorf("ReportingMailbox = %q", p["ReportingMailbox"])
	}
}
