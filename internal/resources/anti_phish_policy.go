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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

var _ resource.Resource = &antiPhishPolicyResource{}
var _ resource.ResourceWithImportState = &antiPhishPolicyResource{}

type antiPhishPolicyResource struct {
	client *client.Client
}

// antiPhishPolicyModel represents the Terraform state — policy settings + rule conditions merged
type antiPhishPolicyModel struct {
	// AntiPhishPolicy settings
	ID                                  types.String `tfsdk:"id"`
	Name                                types.String `tfsdk:"name"`
	PhishThresholdLevel                 types.Int64  `tfsdk:"phish_threshold_level"`
	EnableSpoofIntelligence             types.Bool   `tfsdk:"enable_spoof_intelligence"`
	AuthenticationFailAction            types.String `tfsdk:"authentication_fail_action"`
	EnableMailboxIntelligence           types.Bool   `tfsdk:"enable_mailbox_intelligence"`
	EnableMailboxIntelligenceProtection types.Bool   `tfsdk:"enable_mailbox_intelligence_protection"`
	MailboxIntelligenceProtectionAction types.String `tfsdk:"mailbox_intelligence_protection_action"`
	EnableOrganizationDomainsProtection types.Bool   `tfsdk:"enable_organization_domains_protection"`
	EnableUnauthenticatedSender         types.Bool   `tfsdk:"enable_unauthenticated_sender"`
	EnableViaTag                        types.Bool   `tfsdk:"enable_via_tag"`
	EnableFirstContactSafetyTips        types.Bool   `tfsdk:"enable_first_contact_safety_tips"`
	// AntiPhishRule conditions
	Priority                  types.Int64 `tfsdk:"priority"`
	Enabled                   types.Bool  `tfsdk:"enabled"`
	RecipientDomainIs         types.List  `tfsdk:"recipient_domain_is"`
	SentToMemberOf            types.List  `tfsdk:"sent_to_member_of"`
	ExceptIfRecipientDomainIs types.List  `tfsdk:"except_if_recipient_domain_is"`
}

// antiPhishPolicyAPIResponse maps the merged JSON returned by the PowerShell bridge
type antiPhishPolicyAPIResponse struct {
	Identity                            string          `json:"Identity"`
	Name                                string          `json:"Name"`
	PhishThresholdLevel                 int64           `json:"PhishThresholdLevel"`
	EnableSpoofIntelligence             bool            `json:"EnableSpoofIntelligence"`
	AuthenticationFailAction            string          `json:"AuthenticationFailAction"`
	EnableMailboxIntelligence           bool            `json:"EnableMailboxIntelligence"`
	EnableMailboxIntelligenceProtection bool            `json:"EnableMailboxIntelligenceProtection"`
	MailboxIntelligenceProtectionAction string          `json:"MailboxIntelligenceProtectionAction"`
	EnableOrganizationDomainsProtection bool            `json:"EnableOrganizationDomainsProtection"`
	EnableUnauthenticatedSender         bool            `json:"EnableUnauthenticatedSender"`
	EnableViaTag                        bool            `json:"EnableViaTag"`
	EnableFirstContactSafetyTips        bool            `json:"EnableFirstContactSafetyTips"`
	Priority                            int64           `json:"Priority"`
	Enabled                             bool            `json:"Enabled"`
	RecipientDomainIs                   FlexStringSlice `json:"RecipientDomainIs"`
	SentToMemberOf                      FlexStringSlice `json:"SentToMemberOf"`
	ExceptIfRecipientDomainIs           FlexStringSlice `json:"ExceptIfRecipientDomainIs"`
}

// FlexStringSlice unmarshals either a JSON string or a JSON array of strings.
// PowerShell's ConvertTo-Json collapses single-element arrays to scalars in some versions,
// so the bridge may return "domain.com" instead of ["domain.com"]. This type handles both.
type FlexStringSlice []string

func (s *FlexStringSlice) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if single == "" {
			*s = FlexStringSlice{}
		} else {
			*s = FlexStringSlice{single}
		}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*s = FlexStringSlice(arr)
	return nil
}

func NewAntiPhishPolicyResource() resource.Resource {
	return &antiPhishPolicyResource{}
}

func (r *antiPhishPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anti_phish_policy"
}

func (r *antiPhishPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Online anti-phishing policy and its associated rule (Exchange Online Protection; the impersonation and mailbox intelligence settings need Defender for Office 365). The policy holds the security settings; the rule defines which recipients it applies to." +
			" The rule exists only while one of `recipient_domain_is` or `sent_to_member_of` is set. Without one, the policy has no rule of its own and can be scoped by a standalone `msexchangeonline_anti_phish_rule` resource instead: Exchange links a policy to at most one rule.",
		Attributes: map[string]schema.Attribute{
			// — Policy settings —
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the anti-phishing policy.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the anti-phishing policy. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"phish_threshold_level":                  optionalInt64("Aggressiveness of phishing detection. `1` = Standard, `2` = Aggressive, `3` = More aggressive, `4` = Most aggressive. Needs Microsoft Defender for Office 365."),
			"enable_spoof_intelligence":              optionalBool("Enable spoof intelligence to detect and block spoofed senders."),
			"authentication_fail_action":             optionalString("Action for messages that fail authentication checks. Valid values: `MoveToJmf`, `Redirect`, `Quarantine`, `Delete`, `BccMessage`."),
			"enable_mailbox_intelligence":            optionalBool("Enable mailbox intelligence to improve impersonation detection based on user email patterns. Needs Microsoft Defender for Office 365."),
			"enable_mailbox_intelligence_protection": optionalBool("Enable mailbox intelligence protection actions. Requires `enable_mailbox_intelligence = true`. Needs Microsoft Defender for Office 365."),
			"mailbox_intelligence_protection_action": optionalString("Action for messages detected via mailbox intelligence. Valid values: `NoAction`, `MoveToJmf`, `Redirect`, `Quarantine`, `Delete`, `BccMessage`. Needs Microsoft Defender for Office 365."),
			"enable_organization_domains_protection": optionalBool("Protect the organization's own domains from impersonation attacks. Needs Microsoft Defender for Office 365."),
			"enable_unauthenticated_sender":          optionalBool("Show a `?` symbol in Outlook when the sender cannot be authenticated."),
			"enable_via_tag":                         optionalBool("Show a `via` tag in Outlook when the sender address differs from the DKIM signature or MAIL FROM address."),
			"enable_first_contact_safety_tips":       optionalBool("Show a safety tip when a user receives an email from a sender they haven't contacted before."),
			// — Rule conditions —
			"priority":                      optionalInt64("Priority of the associated rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority. Only applies when a recipient condition is set."),
			"enabled":                       optionalBool("Whether the associated rule is enabled."),
			"recipient_domain_is":           optionalList("Apply this policy to messages sent to recipients in these domains."),
			"sent_to_member_of":             optionalList("Apply this policy to messages sent to members of these distribution groups."),
			"except_if_recipient_domain_is": optionalList("Exclude messages sent to recipients in these domains."),
		},
	}
}

func (r *antiPhishPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *antiPhishPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan antiPhishPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg antiPhishPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "anti_phish_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create anti-phishing policy", err.Error())
		return
	}

	var apiResp antiPhishPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, antiPhishPolicyFromAPI(apiResp))...)
}

func (r *antiPhishPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state antiPhishPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "anti_phish_policy.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read anti-phishing policy", err.Error())
		return
	}

	var apiResp antiPhishPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, antiPhishPolicyFromAPI(apiResp))...)
}

func (r *antiPhishPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan antiPhishPolicyModel
	var state antiPhishPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg antiPhishPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "anti_phish_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update anti-phishing policy", err.Error())
		return
	}

	var apiResp antiPhishPolicyAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, antiPhishPolicyFromAPI(apiResp))...)
}

func (r *antiPhishPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state antiPhishPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "anti_phish_policy.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete anti-phishing policy", err.Error())
	}
}

func (r *antiPhishPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildParams assembles the PowerShell parameter map for Create and Update.
// Lists are serialized as comma-separated strings since the client passes scalar args.
func (r *antiPhishPolicyResource) buildParams(ctx context.Context, action string, cfg antiPhishPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putInt64(p, "PhishThresholdLevel", cfg.PhishThresholdLevel)
	putBool(p, "EnableSpoofIntelligence", cfg.EnableSpoofIntelligence)
	putString(p, "AuthenticationFailAction", cfg.AuthenticationFailAction)
	putBool(p, "EnableMailboxIntelligence", cfg.EnableMailboxIntelligence)
	putBool(p, "EnableMailboxIntelligenceProtection", cfg.EnableMailboxIntelligenceProtection)
	putString(p, "MailboxIntelligenceProtectionAction", cfg.MailboxIntelligenceProtectionAction)
	putBool(p, "EnableOrganizationDomainsProtection", cfg.EnableOrganizationDomainsProtection)
	putBool(p, "EnableUnauthenticatedSender", cfg.EnableUnauthenticatedSender)
	putBool(p, "EnableViaTag", cfg.EnableViaTag)
	putBool(p, "EnableFirstContactSafetyTips", cfg.EnableFirstContactSafetyTips)
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

// antiPhishPolicyFromAPI converts the merged PowerShell JSON response to the Terraform state model
func antiPhishPolicyFromAPI(a antiPhishPolicyAPIResponse) antiPhishPolicyModel {
	return antiPhishPolicyModel{
		ID:                                  types.StringValue(a.Identity),
		Name:                                types.StringValue(a.Name),
		PhishThresholdLevel:                 types.Int64Value(a.PhishThresholdLevel),
		EnableSpoofIntelligence:             types.BoolValue(a.EnableSpoofIntelligence),
		AuthenticationFailAction:            types.StringValue(a.AuthenticationFailAction),
		EnableMailboxIntelligence:           types.BoolValue(a.EnableMailboxIntelligence),
		EnableMailboxIntelligenceProtection: types.BoolValue(a.EnableMailboxIntelligenceProtection),
		MailboxIntelligenceProtectionAction: types.StringValue(a.MailboxIntelligenceProtectionAction),
		EnableOrganizationDomainsProtection: types.BoolValue(a.EnableOrganizationDomainsProtection),
		EnableUnauthenticatedSender:         types.BoolValue(a.EnableUnauthenticatedSender),
		EnableViaTag:                        types.BoolValue(a.EnableViaTag),
		EnableFirstContactSafetyTips:        types.BoolValue(a.EnableFirstContactSafetyTips),
		Priority:                            types.Int64Value(a.Priority),
		Enabled:                             types.BoolValue(a.Enabled),
		RecipientDomainIs:                   stringsToList(a.RecipientDomainIs),
		SentToMemberOf:                      stringsToList(a.SentToMemberOf),
		ExceptIfRecipientDomainIs:           stringsToList(a.ExceptIfRecipientDomainIs),
	}
}

// listToCSV serializes a types.List into a comma-separated string for PowerShell
func listToCSV(ctx context.Context, list types.List) string {
	if list.IsNull() || list.IsUnknown() {
		return ""
	}
	var items []string
	list.ElementsAs(ctx, &items, false)
	return strings.Join(items, ",")
}

// stringsToList converts a Go string slice into a Terraform types.List
func stringsToList(items []string) types.List {
	if len(items) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}
	vals := make([]attr.Value, len(items))
	for i, s := range items {
		vals[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, vals)
}

// stringsToSet converts a Go string slice into a Terraform types.Set (order-independent)
func stringsToSet(items []string) types.Set {
	if len(items) == 0 {
		return types.SetValueMust(types.StringType, []attr.Value{})
	}
	vals := make([]attr.Value, len(items))
	for i, s := range items {
		vals[i] = types.StringValue(s)
	}
	return types.SetValueMust(types.StringType, vals)
}

// setToCSV serializes a types.Set into a comma-separated string for PowerShell
func setToCSV(ctx context.Context, set types.Set) string {
	if set.IsNull() || set.IsUnknown() {
		return ""
	}
	var items []string
	set.ElementsAs(ctx, &items, false)
	return strings.Join(items, ",")
}
