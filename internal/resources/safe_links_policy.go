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

var _ resource.Resource = &safeLinksPolicyResource{}
var _ resource.ResourceWithImportState = &safeLinksPolicyResource{}

type safeLinksPolicyResource struct {
	client *client.Client
}

// safeLinksPolicyModel represents the Terraform state — policy settings + rule conditions merged
type safeLinksPolicyModel struct {
	// SafeLinksPolicy settings
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	EnableSafeLinksForEmail  types.Bool   `tfsdk:"enable_safe_links_for_email"`
	EnableSafeLinksForTeams  types.Bool   `tfsdk:"enable_safe_links_for_teams"`
	EnableSafeLinksForOffice types.Bool   `tfsdk:"enable_safe_links_for_office"`
	TrackClicks              types.Bool   `tfsdk:"track_clicks"`
	AllowClickThrough        types.Bool   `tfsdk:"allow_click_through"`
	ScanUrls                 types.Bool   `tfsdk:"scan_urls"`
	EnableForInternalSenders types.Bool   `tfsdk:"enable_for_internal_senders"`
	DeliverMessageAfterScan  types.Bool   `tfsdk:"deliver_message_after_scan"`
	DisableUrlRewrite        types.Bool   `tfsdk:"disable_url_rewrite"`
	DoNotRewriteUrls         types.List   `tfsdk:"do_not_rewrite_urls"`
	// SafeLinksRule conditions
	Priority                  types.Int64 `tfsdk:"priority"`
	Enabled                   types.Bool  `tfsdk:"enabled"`
	RecipientDomainIs         types.List  `tfsdk:"recipient_domain_is"`
	SentToMemberOf            types.List  `tfsdk:"sent_to_member_of"`
	ExceptIfRecipientDomainIs types.List  `tfsdk:"except_if_recipient_domain_is"`
}

// safeLinksPolicyAPIResponse maps the merged JSON returned by the PowerShell bridge
type safeLinksPolicyAPIResponse struct {
	Identity                  string          `json:"Identity"`
	Name                      string          `json:"Name"`
	EnableSafeLinksForEmail   bool            `json:"EnableSafeLinksForEmail"`
	EnableSafeLinksForTeams   bool            `json:"EnableSafeLinksForTeams"`
	EnableSafeLinksForOffice  bool            `json:"EnableSafeLinksForOffice"`
	TrackClicks               bool            `json:"TrackClicks"`
	AllowClickThrough         bool            `json:"AllowClickThrough"`
	ScanUrls                  bool            `json:"ScanUrls"`
	EnableForInternalSenders  bool            `json:"EnableForInternalSenders"`
	DeliverMessageAfterScan   bool            `json:"DeliverMessageAfterScan"`
	DisableUrlRewrite         bool            `json:"DisableUrlRewrite"`
	DoNotRewriteUrls          FlexStringSlice `json:"DoNotRewriteUrls"`
	Priority                  int64           `json:"Priority"`
	Enabled                   bool            `json:"Enabled"`
	RecipientDomainIs         FlexStringSlice `json:"RecipientDomainIs"`
	SentToMemberOf            FlexStringSlice `json:"SentToMemberOf"`
	ExceptIfRecipientDomainIs FlexStringSlice `json:"ExceptIfRecipientDomainIs"`
}

func NewSafeLinksPolicyResource() resource.Resource {
	return &safeLinksPolicyResource{}
}

func (r *safeLinksPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_safe_links_policy"
}

func (r *safeLinksPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Defender for Office 365 Safe Links policy and its associated rule. The policy holds the URL protection settings; the rule defines which recipients it applies to." +
			" The rule exists only while one of `recipient_domain_is` or `sent_to_member_of` is set. Without one, the policy has no rule of its own and can be scoped by a standalone `msexchangeonline_safe_links_rule` resource instead: Exchange links a policy to at most one rule.",
		Attributes: map[string]schema.Attribute{
			// — Policy settings —
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the Safe Links policy.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the Safe Links policy. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enable_safe_links_for_email":  optionalBool("Enable Safe Links URL protection in email messages."),
			"enable_safe_links_for_teams":  optionalBool("Enable Safe Links URL protection in Microsoft Teams messages."),
			"enable_safe_links_for_office": optionalBool("Enable Safe Links URL protection in Office desktop and mobile apps."),
			"track_clicks":                 optionalBool("Track user clicks on Safe Links-protected URLs."),
			"allow_click_through":          optionalBool("Allow users to click through Safe Links warning pages to the original URL."),
			"scan_urls":                    optionalBool("Enable real-time URL scanning before message delivery."),
			"enable_for_internal_senders":  optionalBool("Apply Safe Links protection to messages from internal senders."),
			"deliver_message_after_scan":   optionalBool("Deliver the message while URL scanning completes in the background."),
			"disable_url_rewrite":          optionalBool("Disable URL rewriting in email (use when client compatibility issues arise). Protection still applies via API."),
			"do_not_rewrite_urls":          optionalList("URLs excluded from Safe Links rewriting (e.g. internal tools, trusted partners)."),
			// — Rule conditions —
			"priority":                      optionalInt64("Priority of the associated rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority. Only applies when a recipient condition is set."),
			"enabled":                       optionalBool("Whether the associated rule is enabled."),
			"recipient_domain_is":           optionalList("Apply this policy to messages sent to recipients in these domains."),
			"sent_to_member_of":             optionalList("Apply this policy to messages sent to members of these distribution groups."),
			"except_if_recipient_domain_is": optionalList("Exclude messages sent to recipients in these domains."),
		},
	}
}

func (r *safeLinksPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *safeLinksPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan safeLinksPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg safeLinksPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "safe_links_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Safe Links policy", err.Error())
		return
	}

	var apiResp safeLinksPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, safeLinksPolicyFromAPI(apiResp))...)
}

func (r *safeLinksPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state safeLinksPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "safe_links_policy.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Safe Links policy", err.Error())
		return
	}

	var apiResp safeLinksPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, safeLinksPolicyFromAPI(apiResp))...)
}

func (r *safeLinksPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan safeLinksPolicyModel
	var state safeLinksPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg safeLinksPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "safe_links_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Safe Links policy", err.Error())
		return
	}

	var apiResp safeLinksPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, safeLinksPolicyFromAPI(apiResp))...)
}

func (r *safeLinksPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state safeLinksPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "safe_links_policy.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Safe Links policy", err.Error())
	}
}

func (r *safeLinksPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *safeLinksPolicyResource) buildParams(ctx context.Context, action string, cfg safeLinksPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putBool(p, "EnableSafeLinksForEmail", cfg.EnableSafeLinksForEmail)
	putBool(p, "EnableSafeLinksForTeams", cfg.EnableSafeLinksForTeams)
	putBool(p, "EnableSafeLinksForOffice", cfg.EnableSafeLinksForOffice)
	putBool(p, "TrackClicks", cfg.TrackClicks)
	putBool(p, "AllowClickThrough", cfg.AllowClickThrough)
	putBool(p, "ScanUrls", cfg.ScanUrls)
	putBool(p, "EnableForInternalSenders", cfg.EnableForInternalSenders)
	putBool(p, "DeliverMessageAfterScan", cfg.DeliverMessageAfterScan)
	putBool(p, "DisableUrlRewrite", cfg.DisableUrlRewrite)
	if known(cfg.DoNotRewriteUrls) {
		p["DoNotRewriteUrls"] = listToCSV(ctx, cfg.DoNotRewriteUrls)
	}
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

func safeLinksPolicyFromAPI(a safeLinksPolicyAPIResponse) safeLinksPolicyModel {
	return safeLinksPolicyModel{
		ID:                        types.StringValue(a.Identity),
		Name:                      types.StringValue(a.Name),
		EnableSafeLinksForEmail:   types.BoolValue(a.EnableSafeLinksForEmail),
		EnableSafeLinksForTeams:   types.BoolValue(a.EnableSafeLinksForTeams),
		EnableSafeLinksForOffice:  types.BoolValue(a.EnableSafeLinksForOffice),
		TrackClicks:               types.BoolValue(a.TrackClicks),
		AllowClickThrough:         types.BoolValue(a.AllowClickThrough),
		ScanUrls:                  types.BoolValue(a.ScanUrls),
		EnableForInternalSenders:  types.BoolValue(a.EnableForInternalSenders),
		DeliverMessageAfterScan:   types.BoolValue(a.DeliverMessageAfterScan),
		DisableUrlRewrite:         types.BoolValue(a.DisableUrlRewrite),
		DoNotRewriteUrls:          stringsToList(a.DoNotRewriteUrls),
		Priority:                  types.Int64Value(a.Priority),
		Enabled:                   types.BoolValue(a.Enabled),
		RecipientDomainIs:         stringsToList(a.RecipientDomainIs),
		SentToMemberOf:            stringsToList(a.SentToMemberOf),
		ExceptIfRecipientDomainIs: stringsToList(a.ExceptIfRecipientDomainIs),
	}
}
