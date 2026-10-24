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

var _ resource.Resource = &outboundSpamFilterPolicyResource{}
var _ resource.ResourceWithImportState = &outboundSpamFilterPolicyResource{}

type outboundSpamFilterPolicyResource struct{ client *client.Client }

type outboundSpamFilterPolicyModel struct {
	ID                                        types.String `tfsdk:"id"`
	Name                                      types.String `tfsdk:"name"`
	RecipientLimitExternalPerHour             types.Int64  `tfsdk:"recipient_limit_external_per_hour"`
	RecipientLimitInternalPerHour             types.Int64  `tfsdk:"recipient_limit_internal_per_hour"`
	RecipientLimitPerDay                      types.Int64  `tfsdk:"recipient_limit_per_day"`
	ActionWhenThresholdReached                types.String `tfsdk:"action_when_threshold_reached"`
	AutoForwardingMode                        types.String `tfsdk:"auto_forwarding_mode"`
	BccSuspiciousOutboundMail                 types.Bool   `tfsdk:"bcc_suspicious_outbound_mail"`
	BccSuspiciousOutboundAdditionalRecipients types.List   `tfsdk:"bcc_suspicious_outbound_additional_recipients"`
	NotifyOutboundSpam                        types.Bool   `tfsdk:"notify_outbound_spam"`
	NotifyOutboundSpamRecipients              types.List   `tfsdk:"notify_outbound_spam_recipients"`
	Priority                                  types.Int64  `tfsdk:"priority"`
	Enabled                                   types.Bool   `tfsdk:"enabled"`
	SenderDomainIs                            types.List   `tfsdk:"sender_domain_is"`
	From                                      types.List   `tfsdk:"from"`
	FromMemberOf                              types.List   `tfsdk:"from_member_of"`
}

type outboundSpamFilterPolicyAPIResponse struct {
	Identity                                  string          `json:"Identity"`
	Name                                      string          `json:"Name"`
	RecipientLimitExternalPerHour             int64           `json:"RecipientLimitExternalPerHour"`
	RecipientLimitInternalPerHour             int64           `json:"RecipientLimitInternalPerHour"`
	RecipientLimitPerDay                      int64           `json:"RecipientLimitPerDay"`
	ActionWhenThresholdReached                string          `json:"ActionWhenThresholdReached"`
	AutoForwardingMode                        string          `json:"AutoForwardingMode"`
	BccSuspiciousOutboundMail                 bool            `json:"BccSuspiciousOutboundMail"`
	BccSuspiciousOutboundAdditionalRecipients FlexStringSlice `json:"BccSuspiciousOutboundAdditionalRecipients"`
	NotifyOutboundSpam                        bool            `json:"NotifyOutboundSpam"`
	NotifyOutboundSpamRecipients              FlexStringSlice `json:"NotifyOutboundSpamRecipients"`
	Priority                                  int64           `json:"Priority"`
	Enabled                                   bool            `json:"Enabled"`
	SenderDomainIs                            FlexStringSlice `json:"SenderDomainIs"`
	From                                      FlexStringSlice `json:"From"`
	FromMemberOf                              FlexStringSlice `json:"FromMemberOf"`
}

func NewOutboundSpamFilterPolicyResource() resource.Resource {
	return &outboundSpamFilterPolicyResource{}
}

func (r *outboundSpamFilterPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outbound_spam_filter_policy"
}

func (r *outboundSpamFilterPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Online outbound spam filter policy and its associated rule. It controls outbound sending limits and automatic forwarding; the rule defines which senders it applies to." +
			" The rule exists only while one of `sender_domain_is`, `from` or `from_member_of` is set. Without one, the policy has no rule of its own and can be scoped by a standalone `msexchangeonline_hosted_outbound_spam_filter_rule` resource instead: Exchange links a policy to at most one rule.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier of the outbound spam filter policy.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Name of the outbound spam filter policy.",
			},
			"recipient_limit_external_per_hour":             optionalInt64("Maximum recipients per hour for external messages. `0` = service default."),
			"recipient_limit_internal_per_hour":             optionalInt64("Maximum recipients per hour for internal messages. `0` = service default."),
			"recipient_limit_per_day":                       optionalInt64("Maximum total recipients per day. `0` = service default."),
			"action_when_threshold_reached":                 optionalString("Action when a sending limit is exceeded. Valid values: `Alert`, `BlockUser`, `BlockUserForToday`."),
			"auto_forwarding_mode":                          optionalString("Controls automatic email forwarding. Valid values: `Automatic`, `On`, `Off`."),
			"bcc_suspicious_outbound_mail":                  optionalBool("BCC suspicious outbound messages to the specified recipients."),
			"bcc_suspicious_outbound_additional_recipients": optionalList("BCC recipients for suspicious outbound mail."),
			"notify_outbound_spam":                          optionalBool("Send notifications when a sender is blocked for outbound spam."),
			"notify_outbound_spam_recipients":               optionalList("Email addresses to notify when a sender is blocked."),
			"priority":                                      optionalInt64("Priority of the associated rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority. Only applies when a sender condition is set."),
			"enabled":                                       optionalBool("Whether the associated rule is enabled."),
			"sender_domain_is":                              optionalList("Rule condition: apply this policy to senders in these domains."),
			"from":                                          optionalList("Rule condition: apply this policy to these senders (email addresses or mailbox identities)."),
			"from_member_of":                                optionalList("Rule condition: apply this policy to senders who are members of these groups."),
		},
	}
}

func (r *outboundSpamFilterPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *outboundSpamFilterPolicyResource) buildParams(ctx context.Context, action string, cfg outboundSpamFilterPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putInt64(p, "RecipientLimitExternalPerHour", cfg.RecipientLimitExternalPerHour)
	putInt64(p, "RecipientLimitInternalPerHour", cfg.RecipientLimitInternalPerHour)
	putInt64(p, "RecipientLimitPerDay", cfg.RecipientLimitPerDay)
	putString(p, "ActionWhenThresholdReached", cfg.ActionWhenThresholdReached)
	putString(p, "AutoForwardingMode", cfg.AutoForwardingMode)
	putBool(p, "BccSuspiciousOutboundMail", cfg.BccSuspiciousOutboundMail)
	if known(cfg.BccSuspiciousOutboundAdditionalRecipients) {
		p["BccSuspiciousOutboundAdditionalRecipients"] = listToCSV(ctx, cfg.BccSuspiciousOutboundAdditionalRecipients)
	}
	putBool(p, "NotifyOutboundSpam", cfg.NotifyOutboundSpam)
	if known(cfg.NotifyOutboundSpamRecipients) {
		p["NotifyOutboundSpamRecipients"] = listToCSV(ctx, cfg.NotifyOutboundSpamRecipients)
	}
	if known(cfg.Priority) {
		p["Priority"] = int64OrEmpty(cfg.Priority)
	}
	putBool(p, "Enabled", cfg.Enabled)
	if known(cfg.SenderDomainIs) {
		p["SenderDomainIs"] = listToCSV(ctx, cfg.SenderDomainIs)
	}
	if known(cfg.From) {
		p["From"] = listToCSV(ctx, cfg.From)
	}
	if known(cfg.FromMemberOf) {
		p["FromMemberOf"] = listToCSV(ctx, cfg.FromMemberOf)
	}
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func (r *outboundSpamFilterPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan outboundSpamFilterPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg outboundSpamFilterPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "outbound_spam_filter_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create outbound spam filter policy", err.Error())
		return
	}
	var a outboundSpamFilterPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, outboundSpamFromAPI(a))...)
}

func (r *outboundSpamFilterPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state outboundSpamFilterPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "outbound_spam_filter_policy.ps1", map[string]string{
		"Action": "Get", "Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read outbound spam filter policy", err.Error())
		return
	}
	var a outboundSpamFilterPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, outboundSpamFromAPI(a))...)
}

func (r *outboundSpamFilterPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state outboundSpamFilterPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg outboundSpamFilterPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "outbound_spam_filter_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update outbound spam filter policy", err.Error())
		return
	}
	var a outboundSpamFilterPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, outboundSpamFromAPI(a))...)
}

func (r *outboundSpamFilterPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state outboundSpamFilterPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "outbound_spam_filter_policy.ps1", map[string]string{
		"Action": "Delete", "Identity": state.ID.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete outbound spam filter policy", err.Error())
	}
}

func (r *outboundSpamFilterPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func outboundSpamFromAPI(a outboundSpamFilterPolicyAPIResponse) outboundSpamFilterPolicyModel {
	return outboundSpamFilterPolicyModel{
		ID:                            types.StringValue(a.Identity),
		Name:                          types.StringValue(a.Name),
		RecipientLimitExternalPerHour: types.Int64Value(a.RecipientLimitExternalPerHour),
		RecipientLimitInternalPerHour: types.Int64Value(a.RecipientLimitInternalPerHour),
		RecipientLimitPerDay:          types.Int64Value(a.RecipientLimitPerDay),
		ActionWhenThresholdReached:    types.StringValue(a.ActionWhenThresholdReached),
		AutoForwardingMode:            types.StringValue(a.AutoForwardingMode),
		BccSuspiciousOutboundMail:     types.BoolValue(a.BccSuspiciousOutboundMail),
		BccSuspiciousOutboundAdditionalRecipients: stringsToList(a.BccSuspiciousOutboundAdditionalRecipients),
		NotifyOutboundSpam:                        types.BoolValue(a.NotifyOutboundSpam),
		NotifyOutboundSpamRecipients:              stringsToList(a.NotifyOutboundSpamRecipients),
		Priority:                                  types.Int64Value(a.Priority),
		Enabled:                                   types.BoolValue(a.Enabled),
		SenderDomainIs:                            stringsToList(a.SenderDomainIs),
		From:                                      stringsToList(a.From),
		FromMemberOf:                              stringsToList(a.FromMemberOf),
	}
}
