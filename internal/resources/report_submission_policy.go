package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

const (
	reportSubmissionID     = "default"
	reportSubmissionScript = "report_submission_policy.ps1"
)

// reportSubmissionLanguages are the codes Exchange Online accepts in
// MultiLanguageSetting (from its error message: Microsoft does not document them).
var reportSubmissionLanguages = strings.Fields("en fr de it ja es ko pt ru zn am ar bg bn ca cs da el et eu fi fil gl gu " +
	"he hi hr hu id is kk kn lt lv ml mr ms nl nn nb pl ro sk sl sr sv sw ta te th tr uk ur vi zh")

// reportSubmissionField maps an attribute to a parameter of the policy; the
// table also builds the schema, the values sent and the state.
type reportSubmissionField struct {
	name, param, desc string
	isBool            bool
}

var reportSubmissionFields = []reportSubmissionField{
	{"enable_report_to_microsoft", "EnableReportToMicrosoft", "Send reported messages to Microsoft. `false` with no reporting mailbox turns reporting in Outlook off.", true},
	{"enable_third_party_address", "EnableThirdPartyAddress", "Users report with a non-Microsoft tool. Requires `reporting_mailbox` and `enable_report_to_microsoft = false`.", true},
	{"disable_quarantine_reporting_option", "DisableQuarantineReportingOption", "Prevent users from reporting messages in quarantine.", true},
	{"enable_user_email_notification", "EnableUserEmailNotification", "Send users a result message after an admin reviews what they reported.", true},
	{"enable_custom_notification_sender", "EnableCustomNotificationSender", "Use `notification_sender_address` for result messages.", true},
	{"notification_sender_address", "NotificationSenderAddress", "Sender of result messages. Empty removes it.", false},
	{"enable_organization_branding", "EnableOrganizationBranding", "Show the company logo in the footer of result messages.", true},
	{"notification_footer_message", "NotificationFooterMessage", "Footer of result messages.", false},
	{"junk_review_result_message", "JunkReviewResultMessage", "Text of the result message for junk.", false},
	{"not_junk_review_result_message", "NotJunkReviewResultMessage", "Text of the result message for not junk.", false},
	{"phishing_review_result_message", "PhishingReviewResultMessage", "Text of the result message for phishing.", false},
	{"pre_submit_message_enabled", "PreSubmitMessageEnabled", "Show a message before users report.", true},
	{"post_submit_message_enabled", "PostSubmitMessageEnabled", "Show a message after users report.", true},
	{"pre_submit_message", "PreSubmitMessage", "Text of the message shown before users report.", false},
	{"pre_submit_message_title", "PreSubmitMessageTitle", "Title of the message shown before users report.", false},
	{"post_submit_message", "PostSubmitMessage", "Text of the message shown after users report.", false},
	{"post_submit_message_title", "PostSubmitMessageTitle", "Title of the message shown after users report.", false},
}

// reportSubmissionLanguageText maps an attribute of a language to its list.
type reportSubmissionLanguageText struct {
	name, param string
}

// reportSubmissionLanguageTexts returns the 20 customized texts of a language:
// before and after reporting, for each category. Microsoft has no text after
// reporting a message as not junk.
func reportSubmissionLanguageTexts() []reportSubmissionLanguageText {
	categories := []struct{ name, param string }{{"junk", "Junk"}, {"not_junk", "NotJunk"}, {"phishing", "Phishing"}}
	kinds := []struct{ name, param string }{{"title", "Title"}, {"message", ""}, {"button_text", "ButtonText"}, {"button_link", "ButtonLink"}}
	var out []reportSubmissionLanguageText
	for _, moment := range []struct{ name, param string }{{"pre", "Pre"}, {"post", "Post"}} {
		for _, c := range categories {
			if moment.name == "post" && c.name == "not_junk" {
				continue
			}
			for _, k := range kinds {
				out = append(out, reportSubmissionLanguageText{
					name:  moment.name + "_submit_" + c.name + "_" + k.name,
					param: "MultiLanguage" + moment.param + "SubmitMessage" + k.param + "For" + c.param,
				})
			}
		}
	}
	return out
}

func reportSubmissionLanguageType() types.ObjectType {
	attributes := map[string]attr.Type{"language": types.StringType}
	for _, t := range reportSubmissionLanguageTexts() {
		attributes[t.name] = types.StringType
	}
	return types.ObjectType{AttrTypes: attributes}
}

func reportSubmissionAttrTypes() map[string]attr.Type {
	attributes := map[string]attr.Type{
		"id": types.StringType, "reporting_mailbox": types.StringType,
		"languages": types.SetType{ElemType: reportSubmissionLanguageType()},
	}
	for _, f := range reportSubmissionFields {
		if f.isBool {
			attributes[f.name] = types.BoolType
		} else {
			attributes[f.name] = types.StringType
		}
	}
	return attributes
}

var (
	_ resource.Resource                   = &reportSubmissionPolicyResource{}
	_ resource.ResourceWithImportState    = &reportSubmissionPolicyResource{}
	_ resource.ResourceWithValidateConfig = &reportSubmissionPolicyResource{}
)

// reportSubmissionPolicyResource manages how users report messages: the report
// submission policy and its rule. Destroy leaves them in place.
type reportSubmissionPolicyResource struct {
	client *client.Client
}

func NewReportSubmissionPolicyResource() resource.Resource {
	return &reportSubmissionPolicyResource{}
}

func (r *reportSubmissionPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report_submission_policy"
}

func (r *reportSubmissionPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Always `default`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"reporting_mailbox": optionalString("Exchange Online mailbox that receives user reported messages, written to the policy and to the rule. " +
			"Empty sends nothing to a mailbox and removes the rule. The mailbox must already exist."),
	}
	for _, f := range reportSubmissionFields {
		if f.isBool {
			attributes[f.name] = optionalBool(f.desc)
		} else {
			attributes[f.name] = optionalString(f.desc)
		}
	}
	nested := map[string]schema.Attribute{
		"language": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Language code, for example `en` or `fr`.",
		},
	}
	for _, t := range reportSubmissionLanguageTexts() {
		nested[t.name] = schema.StringAttribute{Optional: true, MarkdownDescription: "Customized text (`" + t.param + "`)."}
	}
	attributes["languages"] = schema.SetNestedAttribute{
		Optional: true,
		Computed: true,
		MarkdownDescription: "Customized messages, one element per language. Exchange Online cannot remove the last language: " +
			"the set needs at least one element.",
		PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
		NestedObject:  schema.NestedAttributeObject{Attributes: nested},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages how users report messages (`Set-ReportSubmissionPolicy`, `Set-ReportSubmissionRule`): reporting tools, reporting mailbox, " +
			"notifications and messages. This is a singleton: there is exactly one per tenant, and destroying the resource only removes it from Terraform state.",
		Attributes: attributes,
	}
}

func (r *reportSubmissionPolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a := cfg.Attributes()
	if thirdParty := a["enable_third_party_address"].(types.Bool); !thirdParty.IsUnknown() && thirdParty.ValueBool() {
		if mailbox := a["reporting_mailbox"].(types.String); !mailbox.IsUnknown() && mailbox.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("reporting_mailbox"), "Reporting mailbox required",
				"A non-Microsoft reporting tool needs reporting_mailbox.")
		}
		if microsoft := a["enable_report_to_microsoft"].(types.Bool); !microsoft.IsUnknown() && (microsoft.IsNull() || microsoft.ValueBool()) {
			resp.Diagnostics.AddAttributeError(path.Root("enable_report_to_microsoft"), "Conflicting reporting tools",
				"A non-Microsoft reporting tool needs enable_report_to_microsoft = false.")
		}
	}
	languages := a["languages"].(types.Set)
	if languages.IsNull() || languages.IsUnknown() {
		return
	}
	at := path.Root("languages")
	if len(languages.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(at, "At least one language", "Exchange Online cannot remove the last language of the report submission policy.")
	}
	if d := duplicateLanguage(languages); d != "" {
		resp.Diagnostics.AddAttributeError(at, "Duplicate language", fmt.Sprintf("The language %q is set more than once.", d))
	}
	allowed := map[string]bool{}
	for _, l := range reportSubmissionLanguages {
		allowed[l] = true
	}
	for _, a := range knownLanguages(languages) {
		if l := a["language"].(types.String); !l.IsUnknown() && !l.IsNull() && !allowed[l.ValueString()] {
			resp.Diagnostics.AddAttributeError(at, "Unknown language", fmt.Sprintf("%q is not a language code Exchange Online accepts: lowercase, such as en or fr.", l.ValueString()))
		}
	}
	if name := emptyLanguageText(languages); name != "" {
		resp.Diagnostics.AddAttributeError(at, "Empty text", fmt.Sprintf("%s is empty: leave it out instead.", name))
	}
}

func (r *reportSubmissionPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// reportSubmissionParams sends the attributes set in the configuration.
func reportSubmissionParams(cfg types.Object, stateMailbox types.String) map[string]string {
	a := cfg.Attributes()
	values := map[string]any{}
	for _, f := range reportSubmissionFields {
		if !known(a[f.name]) {
			continue
		}
		if f.isBool {
			values[f.param] = a[f.name].(types.Bool).ValueBool()
		} else {
			values[f.param] = a[f.name].(types.String).ValueString()
		}
	}
	p := map[string]string{"Action": "Update", "Values": encodeJSON(values)}
	if mailbox := a["reporting_mailbox"].(types.String); known(mailbox) {
		p["ReportingMailbox"] = mailbox.ValueString()
	} else if known(stateMailbox) {
		// Not managed by the configuration: the result stays what the state holds.
		p["ExpectedMailbox"] = stateMailbox.ValueString()
	}
	if languages := a["languages"].(types.Set); known(languages) {
		texts := reportSubmissionLanguageTexts()
		out := make([]map[string]string, 0, len(languages.Elements()))
		for _, e := range languages.Elements() {
			attrs := e.(types.Object).Attributes()
			language := map[string]string{"Language": attrs["language"].(types.String).ValueString()}
			for _, t := range texts {
				language[t.param] = attrs[t.name].(types.String).ValueString()
			}
			out = append(out, language)
		}
		p["Languages"] = encodeJSON(out)
	}
	return p
}

func (r *reportSubmissionPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if !resp.Diagnostics.HasError() {
		r.write(ctx, cfg, types.StringNull(), &resp.State, &resp.Diagnostics)
	}
}

func (r *reportSubmissionPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, prior types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if !resp.Diagnostics.HasError() {
		r.write(ctx, cfg, prior.Attributes()["reporting_mailbox"].(types.String), &resp.State, &resp.Diagnostics)
	}
}

func (r *reportSubmissionPolicyResource) write(ctx context.Context, cfg types.Object, stateMailbox types.String, state stateSetter, diags diagAppender) {
	output, err := r.client.RunScript(ctx, reportSubmissionScript, reportSubmissionParams(cfg, stateMailbox))
	if err != nil {
		diags.AddError("Failed to write the report submission policy", err.Error())
		return
	}
	obj, err := reportSubmissionFromAPI(output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, obj)...)
}

func (r *reportSubmissionPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, reportSubmissionScript, map[string]string{
		"Action": "Get", "ExpectedMailbox": prior.Attributes()["reporting_mailbox"].(types.String).ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the report submission policy", err.Error())
		return
	}
	obj, err := reportSubmissionFromAPI(output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// Delete only removes the resource from the state: destroying it must not
// change how users report messages.
func (r *reportSubmissionPolicyResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	warnStateOnlyDestroy(&resp.Diagnostics, "report_submission_policy", "How users report messages stays as configured.")
}

func (r *reportSubmissionPolicyResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), reportSubmissionID)...)
}

// reportSubmissionFromAPI builds the state from the bridge response.
func reportSubmissionFromAPI(output []byte) (types.Object, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(output, &raw); err != nil {
		return types.Object{}, err
	}
	text := func(key string) string {
		var s string
		_ = json.Unmarshal(raw[key], &s)
		return s
	}
	attributes := map[string]attr.Value{
		"id":                types.StringValue(reportSubmissionID),
		"reporting_mailbox": types.StringValue(text("ReportingMailbox")),
	}
	for _, f := range reportSubmissionFields {
		if f.isBool {
			var b bool
			_ = json.Unmarshal(raw[f.param], &b)
			attributes[f.name] = types.BoolValue(b)
		} else {
			attributes[f.name] = types.StringValue(text(f.param))
		}
	}
	var languages []map[string]string
	if len(raw["Languages"]) > 0 {
		if err := json.Unmarshal(raw["Languages"], &languages); err != nil {
			return types.Object{}, err
		}
	}
	languageType := reportSubmissionLanguageType()
	elems := make([]attr.Value, 0, len(languages))
	for _, l := range languages {
		values := map[string]attr.Value{"language": types.StringValue(l["Language"])}
		for _, t := range reportSubmissionLanguageTexts() {
			values[t.name] = optionalLanguageText(l[t.param])
		}
		elems = append(elems, types.ObjectValueMust(languageType.AttrTypes, values))
	}
	// Null, not empty: the configuration terraform query generates must not hold
	// the empty set that validation refuses.
	attributes["languages"] = types.SetNull(languageType)
	if len(elems) > 0 {
		attributes["languages"] = types.SetValueMust(languageType, elems)
	}
	return types.ObjectValueMust(reportSubmissionAttrTypes(), attributes), nil
}
