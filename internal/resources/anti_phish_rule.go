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

var _ resource.Resource = &antiPhishRuleResource{}
var _ resource.ResourceWithImportState = &antiPhishRuleResource{}

// antiPhishRuleResource manages a standalone AntiPhishRule — use it when you need
// multiple scoping rules pointing to the same policy (one policy, different recipient groups).
type antiPhishRuleResource struct {
	client *client.Client
}

// antiPhishRuleModel represents the Terraform state for an anti-phishing rule
type antiPhishRuleModel struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	AntiPhishPolicy           types.String `tfsdk:"anti_phish_policy"`
	Priority                  types.Int64  `tfsdk:"priority"`
	Enabled                   types.Bool   `tfsdk:"enabled"`
	RecipientDomainIs         types.List   `tfsdk:"recipient_domain_is"`
	SentTo                    types.List   `tfsdk:"sent_to"`
	SentToMemberOf            types.List   `tfsdk:"sent_to_member_of"`
	ExceptIfRecipientDomainIs types.List   `tfsdk:"except_if_recipient_domain_is"`
	ExceptIfSentTo            types.List   `tfsdk:"except_if_sent_to"`
}

type antiPhishRuleAPIResponse struct {
	Identity                  string          `json:"Identity"`
	Name                      string          `json:"Name"`
	AntiPhishPolicy           string          `json:"AntiPhishPolicy"`
	Priority                  int64           `json:"Priority"`
	Enabled                   bool            `json:"Enabled"`
	RecipientDomainIs         FlexStringSlice `json:"RecipientDomainIs"`
	SentTo                    FlexStringSlice `json:"SentTo"`
	SentToMemberOf            FlexStringSlice `json:"SentToMemberOf"`
	ExceptIfRecipientDomainIs FlexStringSlice `json:"ExceptIfRecipientDomainIs"`
	ExceptIfSentTo            FlexStringSlice `json:"ExceptIfSentTo"`
}

func NewAntiPhishRuleResource() resource.Resource {
	return &antiPhishRuleResource{}
}

func (r *antiPhishRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anti_phish_rule"
}

func (r *antiPhishRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a standalone Exchange Online anti-phishing rule. Use this resource to link an existing `msexchangeonline_anti_phish_policy` to a specific set of recipients without managing the policy settings. Useful when multiple recipient groups need different scoping on the same policy.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the anti-phishing rule.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the rule. Must be unique. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"anti_phish_policy": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the anti-phishing policy this rule applies. The policy must exist before the rule is created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"priority":                      optionalInt64("Priority of the rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority."),
			"enabled":                       optionalBool("Whether the rule is enabled."),
			"recipient_domain_is":           optionalList("Apply this rule to messages sent to recipients in these domains."),
			"sent_to":                       optionalList("Apply this rule to messages sent to these specific recipients (email addresses)."),
			"sent_to_member_of":             optionalList("Apply this rule to messages sent to members of these distribution groups."),
			"except_if_recipient_domain_is": optionalList("Exclude messages sent to recipients in these domains."),
			"except_if_sent_to":             optionalList("Exclude messages sent to these specific recipients."),
		},
	}
}

func (r *antiPhishRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *antiPhishRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan antiPhishRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg antiPhishRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "anti_phish_rule.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create anti-phishing rule", err.Error())
		return
	}

	var apiResp antiPhishRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, antiPhishRuleFromAPI(apiResp))...)
}

func (r *antiPhishRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state antiPhishRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "anti_phish_rule.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read anti-phishing rule", err.Error())
		return
	}

	var apiResp antiPhishRuleAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, antiPhishRuleFromAPI(apiResp))...)
}

func (r *antiPhishRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state antiPhishRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg antiPhishRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "anti_phish_rule.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update anti-phishing rule", err.Error())
		return
	}

	var apiResp antiPhishRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, antiPhishRuleFromAPI(apiResp))...)
}

func (r *antiPhishRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state antiPhishRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "anti_phish_rule.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete anti-phishing rule", err.Error())
	}
}

func (r *antiPhishRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *antiPhishRuleResource) buildParams(ctx context.Context, action string, cfg antiPhishRuleModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString(), "AntiPhishPolicy": cfg.AntiPhishPolicy.ValueString()}
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

func antiPhishRuleFromAPI(a antiPhishRuleAPIResponse) antiPhishRuleModel {
	return antiPhishRuleModel{
		ID:                        types.StringValue(a.Identity),
		Name:                      types.StringValue(a.Name),
		AntiPhishPolicy:           types.StringValue(a.AntiPhishPolicy),
		Priority:                  types.Int64Value(a.Priority),
		Enabled:                   types.BoolValue(a.Enabled),
		RecipientDomainIs:         stringsToList(a.RecipientDomainIs),
		SentTo:                    stringsToList(a.SentTo),
		SentToMemberOf:            stringsToList(a.SentToMemberOf),
		ExceptIfRecipientDomainIs: stringsToList(a.ExceptIfRecipientDomainIs),
		ExceptIfSentTo:            stringsToList(a.ExceptIfSentTo),
	}
}
