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

var _ resource.Resource = &hostedContentFilterRuleResource{}
var _ resource.ResourceWithImportState = &hostedContentFilterRuleResource{}

type hostedContentFilterRuleResource struct{ client *client.Client }

type hostedContentFilterRuleModel struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	HostedContentFilterPolicy types.String `tfsdk:"hosted_content_filter_policy"`
	Priority                  types.Int64  `tfsdk:"priority"`
	Enabled                   types.Bool   `tfsdk:"enabled"`
	RecipientDomainIs         types.List   `tfsdk:"recipient_domain_is"`
	SentToMemberOf            types.List   `tfsdk:"sent_to_member_of"`
	ExceptIfRecipientDomainIs types.List   `tfsdk:"except_if_recipient_domain_is"`
}

type hostedContentFilterRuleAPIResponse struct {
	Identity                  string          `json:"Identity"`
	Name                      string          `json:"Name"`
	HostedContentFilterPolicy string          `json:"HostedContentFilterPolicy"`
	Priority                  int64           `json:"Priority"`
	Enabled                   bool            `json:"Enabled"`
	RecipientDomainIs         FlexStringSlice `json:"RecipientDomainIs"`
	SentToMemberOf            FlexStringSlice `json:"SentToMemberOf"`
	ExceptIfRecipientDomainIs FlexStringSlice `json:"ExceptIfRecipientDomainIs"`
}

func NewHostedContentFilterRuleResource() resource.Resource {
	return &hostedContentFilterRuleResource{}
}

func (r *hostedContentFilterRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hosted_content_filter_rule"
}

func (r *hostedContentFilterRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a standalone hosted content filter rule (spam filter rule). Use this when the policy is managed separately via `msexchangeonline_hosted_content_filter_policy`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier of the spam filter rule.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Name of the hosted content filter rule."},
			"hosted_content_filter_policy": schema.StringAttribute{Required: true,
				MarkdownDescription: "Name of the hosted content filter policy this rule applies."},
			"priority":                      optionalInt64("Priority of the rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority."),
			"enabled":                       optionalBool("Whether the rule is enabled."),
			"recipient_domain_is":           optionalList("Apply this rule to recipients in these domains."),
			"sent_to_member_of":             optionalList("Apply this rule to members of these groups."),
			"except_if_recipient_domain_is": optionalList("Exclude recipients in these domains."),
		},
	}
}

func (r *hostedContentFilterRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *hostedContentFilterRuleResource) buildParams(ctx context.Context, action string, cfg hostedContentFilterRuleModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString(), "HostedContentFilterPolicy": cfg.HostedContentFilterPolicy.ValueString()}
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

func (r *hostedContentFilterRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostedContentFilterRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg hostedContentFilterRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "hosted_content_filter_rule.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create hosted content filter rule", err.Error())
		return
	}
	var a hostedContentFilterRuleAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, hostedContentFilterRuleFromAPI(a))...)
}

func (r *hostedContentFilterRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostedContentFilterRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "hosted_content_filter_rule.ps1", map[string]string{"Action": "Get", "Identity": state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read hosted content filter rule", err.Error())
		return
	}
	var a hostedContentFilterRuleAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, hostedContentFilterRuleFromAPI(a))...)
}

func (r *hostedContentFilterRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state hostedContentFilterRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg hostedContentFilterRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "hosted_content_filter_rule.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update hosted content filter rule", err.Error())
		return
	}
	var a hostedContentFilterRuleAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, hostedContentFilterRuleFromAPI(a))...)
}

func (r *hostedContentFilterRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostedContentFilterRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "hosted_content_filter_rule.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete hosted content filter rule", err.Error())
	}
}

func (r *hostedContentFilterRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func hostedContentFilterRuleFromAPI(a hostedContentFilterRuleAPIResponse) hostedContentFilterRuleModel {
	return hostedContentFilterRuleModel{
		ID: types.StringValue(a.Identity), Name: types.StringValue(a.Name),
		HostedContentFilterPolicy: types.StringValue(a.HostedContentFilterPolicy),
		Priority:                  types.Int64Value(a.Priority),
		Enabled:                   types.BoolValue(a.Enabled),
		RecipientDomainIs:         stringsToList(a.RecipientDomainIs),
		SentToMemberOf:            stringsToList(a.SentToMemberOf),
		ExceptIfRecipientDomainIs: stringsToList(a.ExceptIfRecipientDomainIs),
	}
}
