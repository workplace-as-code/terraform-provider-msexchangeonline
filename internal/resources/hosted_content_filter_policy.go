package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

var _ resource.Resource = &hostedContentFilterPolicyResource{}
var _ resource.ResourceWithImportState = &hostedContentFilterPolicyResource{}

// hostedContentFilterPolicyResource manages Exchange Online spam filter policies and their rules.
type hostedContentFilterPolicyResource struct {
	client *client.Client
}

// hostedContentFilterPolicyModel merges policy settings + rule conditions in one Terraform resource
type hostedContentFilterPolicyModel struct {
	// Policy settings
	ID                               types.String `tfsdk:"id"`
	Name                             types.String `tfsdk:"name"`
	SpamAction                       types.String `tfsdk:"spam_action"`
	HighConfidenceSpamAction         types.String `tfsdk:"high_confidence_spam_action"`
	PhishSpamAction                  types.String `tfsdk:"phish_spam_action"`
	HighConfidencePhishAction        types.String `tfsdk:"high_confidence_phish_action"`
	BulkSpamAction                   types.String `tfsdk:"bulk_spam_action"`
	BulkThreshold                    types.Int64  `tfsdk:"bulk_threshold"`
	QuarantineTag                    types.String `tfsdk:"quarantine_tag"`
	HighConfidenceSpamQuarantineTag  types.String `tfsdk:"high_confidence_spam_quarantine_tag"`
	PhishQuarantineTag               types.String `tfsdk:"phish_quarantine_tag"`
	HighConfidencePhishQuarantineTag types.String `tfsdk:"high_confidence_phish_quarantine_tag"`
	BulkQuarantineTag                types.String `tfsdk:"bulk_quarantine_tag"`
	MarkAsSpamBulkMail               types.String `tfsdk:"mark_as_spam_bulk_mail"`
	MarkAsSpamEmptyMessages          types.String `tfsdk:"mark_as_spam_empty_messages"`
	IncreaseScoreWithImageLinks      types.String `tfsdk:"increase_score_with_image_links"`
	// Rule conditions
	Priority                  types.Int64 `tfsdk:"priority"`
	Enabled                   types.Bool  `tfsdk:"enabled"`
	RecipientDomainIs         types.List  `tfsdk:"recipient_domain_is"`
	SentTo                    types.List  `tfsdk:"sent_to"`
	SentToMemberOf            types.List  `tfsdk:"sent_to_member_of"`
	ExceptIfRecipientDomainIs types.List  `tfsdk:"except_if_recipient_domain_is"`
	ExceptIfSentTo            types.List  `tfsdk:"except_if_sent_to"`
}

type hostedContentFilterPolicyAPIResponse struct {
	Identity                         string          `json:"Identity"`
	Name                             string          `json:"Name"`
	SpamAction                       string          `json:"SpamAction"`
	HighConfidenceSpamAction         string          `json:"HighConfidenceSpamAction"`
	PhishSpamAction                  string          `json:"PhishSpamAction"`
	HighConfidencePhishAction        string          `json:"HighConfidencePhishAction"`
	BulkSpamAction                   string          `json:"BulkSpamAction"`
	BulkThreshold                    int64           `json:"BulkThreshold"`
	QuarantineTag                    string          `json:"QuarantineTag"`
	HighConfidenceSpamQuarantineTag  string          `json:"HighConfidenceSpamQuarantineTag"`
	PhishQuarantineTag               string          `json:"PhishQuarantineTag"`
	HighConfidencePhishQuarantineTag string          `json:"HighConfidencePhishQuarantineTag"`
	BulkQuarantineTag                string          `json:"BulkQuarantineTag"`
	MarkAsSpamBulkMail               string          `json:"MarkAsSpamBulkMail"`
	MarkAsSpamEmptyMessages          string          `json:"MarkAsSpamEmptyMessages"`
	IncreaseScoreWithImageLinks      string          `json:"IncreaseScoreWithImageLinks"`
	Priority                         int64           `json:"Priority"`
	Enabled                          bool            `json:"Enabled"`
	RecipientDomainIs                FlexStringSlice `json:"RecipientDomainIs"`
	SentTo                           FlexStringSlice `json:"SentTo"`
	SentToMemberOf                   FlexStringSlice `json:"SentToMemberOf"`
	ExceptIfRecipientDomainIs        FlexStringSlice `json:"ExceptIfRecipientDomainIs"`
	ExceptIfSentTo                   FlexStringSlice `json:"ExceptIfSentTo"`
}

func NewHostedContentFilterPolicyResource() resource.Resource {
	return &hostedContentFilterPolicyResource{}
}

func (r *hostedContentFilterPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hosted_content_filter_policy"
}

func (r *hostedContentFilterPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Online spam filter (hosted content filter) policy and its associated rule (Exchange Online Protection). The policy holds spam detection settings; the rule defines which recipients it applies to." +
			" The rule exists only while one of `recipient_domain_is`, `sent_to` or `sent_to_member_of` is set. Without one, the policy has no rule of its own and can be scoped by a standalone `msexchangeonline_hosted_content_filter_rule` resource instead: Exchange links a policy to at most one rule.",
		Attributes: map[string]schema.Attribute{
			// — Policy settings —
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the spam filter policy.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the spam filter policy. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"spam_action":                          optionalString("Action for messages detected as spam. Valid values: `MoveToJmf`, `Quarantine`, `Redirect`, `Delete`, `AddXHeader`."),
			"high_confidence_spam_action":          optionalString("Action for high-confidence spam messages. Valid values: `MoveToJmf`, `Quarantine`, `Redirect`, `Delete`, `AddXHeader`."),
			"phish_spam_action":                    optionalString("Action for messages detected as phishing. Valid values: `MoveToJmf`, `Quarantine`, `Redirect`, `Delete`, `AddXHeader`."),
			"high_confidence_phish_action":         optionalString("Action for high-confidence phishing messages. Valid values: `Quarantine`, `Delete`."),
			"bulk_spam_action":                     optionalString("Action for bulk email at or above `bulk_threshold`. Valid values: `MoveToJmf`, `Quarantine`, `AddXHeader`."),
			"bulk_threshold":                       optionalInt64("Bulk complaint level threshold (1–9). Messages at or above this threshold trigger `bulk_spam_action`. Lower = stricter."),
			"quarantine_tag":                       optionalString("Quarantine policy for spam messages. Leave empty to use the tenant default."),
			"high_confidence_spam_quarantine_tag":  optionalString("Quarantine policy for high-confidence spam messages."),
			"phish_quarantine_tag":                 optionalString("Quarantine policy for phishing messages."),
			"high_confidence_phish_quarantine_tag": optionalString("Quarantine policy for high-confidence phishing messages."),
			"bulk_quarantine_tag":                  optionalString("Quarantine policy for bulk messages."),
			// MarkAsSpam* values are Exchange "On/Off" strings, not booleans
			"mark_as_spam_bulk_mail":          optionalString("Mark bulk email as spam. Valid values: `On`, `Off`."),
			"mark_as_spam_empty_messages":     optionalString("Mark empty messages as spam. Valid values: `On`, `Off`."),
			"increase_score_with_image_links": optionalString("Increase spam score for messages containing image links. Valid values: `On`, `Off`."),
			// — Rule conditions —
			"priority":                      optionalInt64("Priority of the associated rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority. Only applies when a recipient condition is set."),
			"enabled":                       optionalBool("Whether the associated rule is enabled."),
			"recipient_domain_is":           optionalList("Apply this policy to messages sent to recipients in these domains."),
			"sent_to":                       optionalList("Apply this policy to messages sent to these specific recipients."),
			"sent_to_member_of":             optionalList("Apply this policy to messages sent to members of these distribution groups."),
			"except_if_recipient_domain_is": optionalList("Exclude messages sent to recipients in these domains."),
			"except_if_sent_to":             optionalList("Exclude messages sent to these specific recipients."),
		},
	}
}

func (r *hostedContentFilterPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = apiClient
}

func (r *hostedContentFilterPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostedContentFilterPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg hostedContentFilterPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "hosted_content_filter_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create spam filter policy", err.Error())
		return
	}

	var apiResp hostedContentFilterPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, hostedContentFilterPolicyFromAPI(apiResp))...)
}

func (r *hostedContentFilterPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostedContentFilterPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "hosted_content_filter_policy.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read spam filter policy", err.Error())
		return
	}

	var apiResp hostedContentFilterPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, hostedContentFilterPolicyFromAPI(apiResp))...)
}

func (r *hostedContentFilterPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state hostedContentFilterPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg hostedContentFilterPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "hosted_content_filter_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update spam filter policy", err.Error())
		return
	}

	var apiResp hostedContentFilterPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, hostedContentFilterPolicyFromAPI(apiResp))...)
}

func (r *hostedContentFilterPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostedContentFilterPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "hosted_content_filter_policy.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete spam filter policy", err.Error())
	}
}

func (r *hostedContentFilterPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *hostedContentFilterPolicyResource) buildParams(ctx context.Context, action string, cfg hostedContentFilterPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putString(p, "SpamAction", cfg.SpamAction)
	putString(p, "HighConfidenceSpamAction", cfg.HighConfidenceSpamAction)
	putString(p, "PhishSpamAction", cfg.PhishSpamAction)
	putString(p, "HighConfidencePhishAction", cfg.HighConfidencePhishAction)
	putString(p, "BulkSpamAction", cfg.BulkSpamAction)
	putInt64(p, "BulkThreshold", cfg.BulkThreshold)
	putString(p, "QuarantineTag", cfg.QuarantineTag)
	putString(p, "HighConfidenceSpamQuarantineTag", cfg.HighConfidenceSpamQuarantineTag)
	putString(p, "PhishQuarantineTag", cfg.PhishQuarantineTag)
	putString(p, "HighConfidencePhishQuarantineTag", cfg.HighConfidencePhishQuarantineTag)
	putString(p, "BulkQuarantineTag", cfg.BulkQuarantineTag)
	putString(p, "MarkAsSpamBulkMail", cfg.MarkAsSpamBulkMail)
	putString(p, "MarkAsSpamEmptyMessages", cfg.MarkAsSpamEmptyMessages)
	putString(p, "IncreaseScoreWithImageLinks", cfg.IncreaseScoreWithImageLinks)
	if known(cfg.Priority) {
		p["Priority"] = int64OrEmpty(cfg.Priority)
	}
	putBool(p, "Enabled", cfg.Enabled)
	if known(cfg.RecipientDomainIs) {
		p["RecipientDomainIs"] = listToCSV(ctx, cfg.RecipientDomainIs)
	}
	if known(cfg.SentTo) {
		p["SentTo"] = listToCSV(ctx, cfg.SentTo)
	}
	if known(cfg.SentToMemberOf) {
		p["SentToMemberOf"] = listToCSV(ctx, cfg.SentToMemberOf)
	}
	if known(cfg.ExceptIfRecipientDomainIs) {
		p["ExceptIfRecipientDomainIs"] = listToCSV(ctx, cfg.ExceptIfRecipientDomainIs)
	}
	if known(cfg.ExceptIfSentTo) {
		p["ExceptIfSentTo"] = listToCSV(ctx, cfg.ExceptIfSentTo)
	}
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func hostedContentFilterPolicyFromAPI(a hostedContentFilterPolicyAPIResponse) hostedContentFilterPolicyModel {
	return hostedContentFilterPolicyModel{
		ID:                               types.StringValue(a.Identity),
		Name:                             types.StringValue(a.Name),
		SpamAction:                       types.StringValue(a.SpamAction),
		HighConfidenceSpamAction:         types.StringValue(a.HighConfidenceSpamAction),
		PhishSpamAction:                  types.StringValue(a.PhishSpamAction),
		HighConfidencePhishAction:        types.StringValue(a.HighConfidencePhishAction),
		BulkSpamAction:                   types.StringValue(a.BulkSpamAction),
		BulkThreshold:                    types.Int64Value(a.BulkThreshold),
		QuarantineTag:                    types.StringValue(a.QuarantineTag),
		HighConfidenceSpamQuarantineTag:  types.StringValue(a.HighConfidenceSpamQuarantineTag),
		PhishQuarantineTag:               types.StringValue(a.PhishQuarantineTag),
		HighConfidencePhishQuarantineTag: types.StringValue(a.HighConfidencePhishQuarantineTag),
		BulkQuarantineTag:                types.StringValue(a.BulkQuarantineTag),
		MarkAsSpamBulkMail:               types.StringValue(a.MarkAsSpamBulkMail),
		MarkAsSpamEmptyMessages:          types.StringValue(a.MarkAsSpamEmptyMessages),
		IncreaseScoreWithImageLinks:      types.StringValue(a.IncreaseScoreWithImageLinks),
		Priority:                         types.Int64Value(a.Priority),
		Enabled:                          types.BoolValue(a.Enabled),
		RecipientDomainIs:                stringsToList(a.RecipientDomainIs),
		SentTo:                           stringsToList(a.SentTo),
		SentToMemberOf:                   stringsToList(a.SentToMemberOf),
		ExceptIfRecipientDomainIs:        stringsToList(a.ExceptIfRecipientDomainIs),
		ExceptIfSentTo:                   stringsToList(a.ExceptIfSentTo),
	}
}
