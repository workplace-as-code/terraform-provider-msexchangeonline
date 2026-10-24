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

var _ resource.Resource = &safeAttachmentPolicyResource{}
var _ resource.ResourceWithImportState = &safeAttachmentPolicyResource{}

type safeAttachmentPolicyResource struct {
	client *client.Client
}

// safeAttachmentPolicyModel represents the Terraform state — policy settings + rule conditions merged
type safeAttachmentPolicyModel struct {
	// SafeAttachmentPolicy settings
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	SafeAttachmentAction types.String `tfsdk:"safe_attachment_action"`
	Redirect             types.Bool   `tfsdk:"redirect"`
	RedirectAddress      types.String `tfsdk:"redirect_address"`
	QuarantineTag        types.String `tfsdk:"quarantine_tag"`
	// SafeAttachmentRule conditions
	Priority                  types.Int64 `tfsdk:"priority"`
	Enabled                   types.Bool  `tfsdk:"enabled"`
	RecipientDomainIs         types.List  `tfsdk:"recipient_domain_is"`
	SentToMemberOf            types.List  `tfsdk:"sent_to_member_of"`
	ExceptIfRecipientDomainIs types.List  `tfsdk:"except_if_recipient_domain_is"`
}

// safeAttachmentPolicyAPIResponse maps the merged JSON returned by the PowerShell bridge
type safeAttachmentPolicyAPIResponse struct {
	Identity                  string          `json:"Identity"`
	Name                      string          `json:"Name"`
	SafeAttachmentAction      string          `json:"SafeAttachmentAction"`
	Redirect                  bool            `json:"Redirect"`
	RedirectAddress           string          `json:"RedirectAddress"`
	QuarantineTag             string          `json:"QuarantineTag"`
	Priority                  int64           `json:"Priority"`
	Enabled                   bool            `json:"Enabled"`
	RecipientDomainIs         FlexStringSlice `json:"RecipientDomainIs"`
	SentToMemberOf            FlexStringSlice `json:"SentToMemberOf"`
	ExceptIfRecipientDomainIs FlexStringSlice `json:"ExceptIfRecipientDomainIs"`
}

func NewSafeAttachmentPolicyResource() resource.Resource {
	return &safeAttachmentPolicyResource{}
}

func (r *safeAttachmentPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_safe_attachment_policy"
}

func (r *safeAttachmentPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Defender for Office 365 Safe Attachments policy and its associated rule. The policy holds the attachment scanning settings; the rule defines which recipients it applies to." +
			" The rule exists only while one of `recipient_domain_is` or `sent_to_member_of` is set. Without one, the policy has no rule of its own and can be scoped by a standalone `msexchangeonline_safe_attachment_rule` resource instead: Exchange links a policy to at most one rule.",
		Attributes: map[string]schema.Attribute{
			// — Policy settings —
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the Safe Attachments policy.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the Safe Attachments policy. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"safe_attachment_action": optionalEnum("Response to attachments with unknown malware, as in the Microsoft Defender portal. Valid values: `Off` (attachments are not scanned), `Allow` (Monitor in the portal: deliver and track the scan results), `Block`, `DynamicDelivery`. `Off` sets the cmdlet's `-Enable` to `$false`; the other values set it to `$true` with `-Action`.", "Off", "Allow", "Block", "DynamicDelivery"),
			"redirect":               optionalBool("Redirect messages with detected malicious attachments to another email address."),
			"redirect_address":       optionalString("Email address to redirect messages to when `redirect` is `true`."),
			"quarantine_tag":         optionalString("The quarantine policy tag applied to quarantined messages. Leave empty to use the organization default."),
			// — Rule conditions —
			"priority":                      optionalInt64("Priority of the associated rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority. Only applies when a recipient condition is set."),
			"enabled":                       optionalBool("Whether the associated rule is enabled."),
			"recipient_domain_is":           optionalList("Apply this policy to messages sent to recipients in these domains."),
			"sent_to_member_of":             optionalList("Apply this policy to messages sent to members of these distribution groups."),
			"except_if_recipient_domain_is": optionalList("Exclude messages sent to recipients in these domains."),
		},
	}
}

func (r *safeAttachmentPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *safeAttachmentPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan safeAttachmentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg safeAttachmentPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "safe_attachment_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Safe Attachments policy", err.Error())
		return
	}

	var apiResp safeAttachmentPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, safeAttachmentPolicyFromAPI(apiResp))...)
}

func (r *safeAttachmentPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state safeAttachmentPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "safe_attachment_policy.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Safe Attachments policy", err.Error())
		return
	}

	var apiResp safeAttachmentPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, safeAttachmentPolicyFromAPI(apiResp))...)
}

func (r *safeAttachmentPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan safeAttachmentPolicyModel
	var state safeAttachmentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg safeAttachmentPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "safe_attachment_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Safe Attachments policy", err.Error())
		return
	}

	var apiResp safeAttachmentPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, safeAttachmentPolicyFromAPI(apiResp))...)
}

func (r *safeAttachmentPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state safeAttachmentPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "safe_attachment_policy.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Safe Attachments policy", err.Error())
	}
}

func (r *safeAttachmentPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *safeAttachmentPolicyResource) buildParams(ctx context.Context, action string, cfg safeAttachmentPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putString(p, "SafeAttachmentAction", cfg.SafeAttachmentAction)
	putBool(p, "Redirect", cfg.Redirect)
	putString(p, "RedirectAddress", cfg.RedirectAddress)
	putString(p, "QuarantineTag", cfg.QuarantineTag)
	if known(cfg.Priority) {
		p["Priority"] = int64OrEmpty(cfg.Priority)
	}
	putBool(p, "Enabled", cfg.Enabled)
	if known(cfg.RecipientDomainIs) {
		p["RecipientDomainIs"] = listToCSV(ctx, cfg.RecipientDomainIs)
	}
	if known(cfg.SentToMemberOf) {
		p["SentToMemberOf"] = listToCSV(ctx, cfg.SentToMemberOf)
	}
	if known(cfg.ExceptIfRecipientDomainIs) {
		p["ExceptIfRecipientDomainIs"] = listToCSV(ctx, cfg.ExceptIfRecipientDomainIs)
	}
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func safeAttachmentPolicyFromAPI(a safeAttachmentPolicyAPIResponse) safeAttachmentPolicyModel {
	return safeAttachmentPolicyModel{
		ID:                        types.StringValue(a.Identity),
		Name:                      types.StringValue(a.Name),
		SafeAttachmentAction:      types.StringValue(a.SafeAttachmentAction),
		Redirect:                  types.BoolValue(a.Redirect),
		RedirectAddress:           types.StringValue(a.RedirectAddress),
		QuarantineTag:             types.StringValue(a.QuarantineTag),
		Priority:                  types.Int64Value(a.Priority),
		Enabled:                   types.BoolValue(a.Enabled),
		RecipientDomainIs:         stringsToList(a.RecipientDomainIs),
		SentToMemberOf:            stringsToList(a.SentToMemberOf),
		ExceptIfRecipientDomainIs: stringsToList(a.ExceptIfRecipientDomainIs),
	}
}
