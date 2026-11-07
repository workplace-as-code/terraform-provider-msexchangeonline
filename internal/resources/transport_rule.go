package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

// Compile-time checks that transportRuleResource implements the interfaces we rely on.
var (
	_ resource.Resource                   = &transportRuleResource{}
	_ resource.ResourceWithImportState    = &transportRuleResource{}
	_ resource.ResourceWithValidateConfig = &transportRuleResource{}
	_ resource.ResourceWithModifyPlan     = &transportRuleResource{}
)

// transportRuleActions lists the actions the bridge script implements.
var transportRuleActions = []string{"Reject", "Quarantine", "MoveToJmf"}

// transportRuleScopes are the values of SentToScope; FromScope takes the
// first two only.
var transportRuleScopes = []string{"InOrganization", "NotInOrganization", "ExternalPartner", "ExternalNonPartner"}

// optionalStringList is an optional list of strings that the bridge receives
// as base64 JSON (encodeStringList).
func optionalStringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{ElementType: types.StringType, Optional: true, MarkdownDescription: desc}
}

// transportRuleResource is the main resource struct
type transportRuleResource struct {
	client *client.Client
}

// transportRuleModel represents the state of a transport rule in HCL/tfstate
type transportRuleModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Priority               types.Int64  `tfsdk:"priority"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	Action                 types.String `tfsdk:"action"`
	RejectMessage          types.String `tfsdk:"reject_message"`
	FromAddressContains    types.List   `tfsdk:"from_address_contains"`
	SubjectContains        types.List   `tfsdk:"subject_contains"`
	AttachmentIsExecutable types.Bool   `tfsdk:"attachment_is_executable"`

	ApplyRightsProtectionTemplate              types.String `tfsdk:"apply_rights_protection_template"`
	ApplyRightsProtectionCustomizationTemplate types.String `tfsdk:"apply_rights_protection_customization_template"`
	FromScope                                  types.String `tfsdk:"from_scope"`
	SentToScope                                types.String `tfsdk:"sent_to_scope"`
	RecipientDomainIs                          types.List   `tfsdk:"recipient_domain_is"`
	SenderDomainIs                             types.List   `tfsdk:"sender_domain_is"`
	SubjectOrBodyContainsWords                 types.List   `tfsdk:"subject_or_body_contains_words"`
	ExceptIfRecipientDomainIs                  types.List   `tfsdk:"except_if_recipient_domain_is"`
}

// transportRuleAPIResponse represents the JSON response from the PowerShell script
type transportRuleAPIResponse struct {
	Identity               string   `json:"Identity"`
	Name                   string   `json:"Name"`
	Priority               int64    `json:"Priority"`
	Enabled                bool     `json:"Enabled"`
	Action                 string   `json:"Action"`
	RejectMessage          string   `json:"RejectMessage"`
	FromAddressContains    []string `json:"FromAddressContains"`
	SubjectContains        []string `json:"SubjectContains"`
	AttachmentIsExecutable bool     `json:"AttachmentIsExecutable"`

	ApplyRightsProtectionTemplate              string   `json:"ApplyRightsProtectionTemplate"`
	ApplyRightsProtectionCustomizationTemplate string   `json:"ApplyRightsProtectionCustomizationTemplate"`
	FromScope                                  string   `json:"FromScope"`
	SentToScope                                string   `json:"SentToScope"`
	RecipientDomainIs                          []string `json:"RecipientDomainIs"`
	SenderDomainIs                             []string `json:"SenderDomainIs"`
	SubjectOrBodyContainsWords                 []string `json:"SubjectOrBodyContainsWords"`
	ExceptIfRecipientDomainIs                  []string `json:"ExceptIfRecipientDomainIs"`
}

// NewTransportRuleResource is the factory function registered in the provider
func NewTransportRuleResource() resource.Resource {
	return &transportRuleResource{}
}

// Metadata defines the resource name as used in HCL
func (r *transportRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transport_rule"
}

// Schema defines all resource attributes
func (r *transportRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange transport rule (mail flow rule). At least one condition is required.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "The unique identifier of the transport rule.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				MarkdownDescription: "The name of the transport rule. Changing it renames the rule in place.",
			},
			"priority": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				// When not configured, keep whatever Exchange assigned instead of
				// forcing 0 (which would push every other rule down and cause drift).
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				MarkdownDescription: "The priority of the rule. Lower numbers are evaluated first. When omitted, Exchange assigns the lowest priority.",
			},
			"enabled": optionalBool("Whether the rule is enabled. A new rule is enabled unless this is `false`."),
			"action": schema.StringAttribute{
				Optional:   true,
				Validators: []validator.String{stringvalidator.OneOf(transportRuleActions...)},
				MarkdownDescription: "The action to take when conditions are met. Valid values: `Reject`, `Quarantine`, " +
					"`MoveToJmf` (deliver to the Junk Email folder). Optional when `apply_rights_protection_template` is set.",
			},
			"apply_rights_protection_template": optionalString("Action: encrypt the message with this rights " +
				"protection template, for example `Encrypt` or `Do Not Forward`, or the name of a custom template. Needs " +
				"Microsoft Purview Message Encryption, and `internal_licensing_enabled = true` in " +
				"`msexchangeonline_irm_configuration`: otherwise Exchange lists no template."),
			"apply_rights_protection_customization_template": optionalString("Action: the branding template " +
				"(`msexchangeonline_ome_configuration`) of the encrypted message, for example `OME Configuration`. Needs " +
				"`apply_rights_protection_template`."),
			"from_scope": schema.StringAttribute{
				Optional:   true,
				Validators: []validator.String{stringvalidator.OneOf(transportRuleScopes[:2]...)},
				MarkdownDescription: "Condition: the sender is inside (`InOrganization`) or outside (`NotInOrganization`) " +
					"the organization.",
			},
			"sent_to_scope": schema.StringAttribute{
				Optional:   true,
				Validators: []validator.String{stringvalidator.OneOf(transportRuleScopes...)},
				MarkdownDescription: "Condition: where the recipient is. Valid values: `InOrganization`, " +
					"`NotInOrganization`, `ExternalPartner`, `ExternalNonPartner`.",
			},
			"recipient_domain_is": optionalStringList("Condition: the recipient's domain is one of these domains " +
				"(subdomains included)."),
			"sender_domain_is": optionalStringList("Condition: the sender's domain is one of these domains."),
			"subject_or_body_contains_words": optionalStringList("Condition: the subject or the body contains any of " +
				"these words or phrases."),
			"except_if_recipient_domain_is": optionalStringList("Exception: the recipient's domain is one of these " +
				"domains."),
			"reject_message": optionalString("The rejection message shown to senders. Required when `action` is `Reject`."),
			"from_address_contains": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Condition: the sender address contains any of these strings.",
			},
			"subject_contains": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Condition: the subject contains any of these words or phrases.",
			},
			"attachment_is_executable": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Condition: an attachment contains executable content.",
			},
		},
	}
}

// ValidateConfig runs at plan time. It blocks configurations that would be
// dangerous or that the bridge cannot apply, before anything touches Exchange.
func (r *transportRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg transportRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Values that come from other resources are unknown until apply; skip the
	// check rather than reject a config that may turn out valid.
	conditions := []types.List{cfg.FromAddressContains, cfg.SubjectContains, cfg.RecipientDomainIs,
		cfg.SenderDomainIs, cfg.SubjectOrBodyContainsWords}
	scopes := []types.String{cfg.FromScope, cfg.SentToScope}
	for _, l := range conditions {
		if l.IsUnknown() {
			return
		}
	}
	for _, s := range append(scopes, cfg.Action, cfg.ApplyRightsProtectionTemplate) {
		if s.IsUnknown() {
			return
		}
	}
	if cfg.AttachmentIsExecutable.IsUnknown() {
		return
	}

	hasCondition := cfg.AttachmentIsExecutable.ValueBool()
	for _, l := range conditions {
		hasCondition = hasCondition || len(l.Elements()) > 0
	}
	for _, s := range scopes {
		hasCondition = hasCondition || s.ValueString() != ""
	}
	if !hasCondition {
		resp.Diagnostics.AddAttributeError(path.Root("action"), "Transport rule without condition",
			"Set at least one condition: from_address_contains, subject_contains, attachment_is_executable, "+
				"from_scope, sent_to_scope, recipient_domain_is, sender_domain_is or subject_or_body_contains_words. "+
				"A transport rule without conditions applies to every message in the organization.")
	}

	encrypts := cfg.ApplyRightsProtectionTemplate.ValueString() != ""
	if cfg.Action.ValueString() == "" && !encrypts {
		resp.Diagnostics.AddAttributeError(path.Root("action"), "Missing action",
			"Set action or apply_rights_protection_template.")
	}
	if !cfg.ApplyRightsProtectionCustomizationTemplate.IsUnknown() &&
		cfg.ApplyRightsProtectionCustomizationTemplate.ValueString() != "" && !encrypts {
		resp.Diagnostics.AddAttributeError(path.Root("apply_rights_protection_customization_template"),
			"Missing apply_rights_protection_template",
			"A branding template only applies to an encrypted message: set apply_rights_protection_template.")
	}

	if cfg.Action.ValueString() == "Reject" && !cfg.RejectMessage.IsUnknown() && cfg.RejectMessage.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("reject_message"), "Missing reject_message",
			"reject_message is required when action is \"Reject\".")
	}
}

// Configure retrieves the shared client injected by the provider
func (r *transportRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// writeParams converts the plan into bridge parameters for Create and Update.
func (r *transportRuleResource) writeParams(ctx context.Context, action string, plan transportRuleModel) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	from, d := encodeStringList(ctx, plan.FromAddressContains)
	diags.Append(d...)
	subject, d := encodeStringList(ctx, plan.SubjectContains)
	diags.Append(d...)

	params := map[string]string{
		"Action":                 action,
		"Name":                   plan.Name.ValueString(),
		"RuleAction":             plan.Action.ValueString(),
		"RejectMessage":          plan.RejectMessage.ValueString(),
		"FromAddressContains":    from,
		"SubjectContains":        subject,
		"AttachmentIsExecutable": fmt.Sprintf("%t", plan.AttachmentIsExecutable.ValueBool()),

		"ApplyRightsProtectionTemplate":              plan.ApplyRightsProtectionTemplate.ValueString(),
		"ApplyRightsProtectionCustomizationTemplate": plan.ApplyRightsProtectionCustomizationTemplate.ValueString(),
		"FromScope":   plan.FromScope.ValueString(),
		"SentToScope": plan.SentToScope.ValueString(),
	}
	for name, l := range map[string]types.List{
		"RecipientDomainIs":          plan.RecipientDomainIs,
		"SenderDomainIs":             plan.SenderDomainIs,
		"SubjectOrBodyContainsWords": plan.SubjectOrBodyContainsWords,
		"ExceptIfRecipientDomainIs":  plan.ExceptIfRecipientDomainIs,
	} {
		enc, d := encodeStringList(ctx, l)
		diags.Append(d...)
		params[name] = enc
	}
	// Unknown means "not configured": Exchange keeps its value or picks one.
	putBool(params, "Enabled", plan.Enabled)
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() {
		params["Priority"] = fmt.Sprintf("%d", plan.Priority.ValueInt64())
	}
	return params, diags
}

// Create handles `terraform apply` for a new resource
func (r *transportRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan transportRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := r.writeParams(ctx, "Create", plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "transport_rule.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create transport rule", err.Error())
		return
	}

	var apiResp transportRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTransportRuleResponse(&plan, apiResp)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read fetches the real state from Exchange — used for drift detection
func (r *transportRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state transportRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "transport_rule.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read transport rule", err.Error())
		return
	}
	if client.IsNotFound(output) {
		// The bridge returned nothing: the rule was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	var apiResp transportRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTransportRuleResponse(&state, apiResp)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles modifications — triggered when the plan differs from the state
func (r *transportRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state transportRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := r.writeParams(ctx, "Update", plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	params["Identity"] = state.ID.ValueString()

	output, err := r.client.RunScript(ctx, "transport_rule.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update transport rule", err.Error())
		return
	}

	// Store what Exchange actually applied, not just what was planned.
	var apiResp transportRuleAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTransportRuleResponse(&plan, apiResp)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the resource — triggered by `terraform destroy`
func (r *transportRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state transportRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "transport_rule.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete transport rule", err.Error())
	}
}

func (r *transportRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyTransportRuleResponse copies the bridge response into the model.
func applyTransportRuleResponse(m *transportRuleModel, a transportRuleAPIResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(a.Identity)
	m.Name = types.StringValue(a.Name)
	m.Priority = types.Int64Value(a.Priority)
	m.Enabled = types.BoolValue(a.Enabled)
	m.Action = stringOrNull(m.Action, a.Action)
	m.RejectMessage = types.StringValue(a.RejectMessage)
	m.ApplyRightsProtectionTemplate = stringOrNull(m.ApplyRightsProtectionTemplate, a.ApplyRightsProtectionTemplate)
	m.ApplyRightsProtectionCustomizationTemplate = stringOrNull(m.ApplyRightsProtectionCustomizationTemplate,
		a.ApplyRightsProtectionCustomizationTemplate)
	m.FromScope = stringOrNull(m.FromScope, a.FromScope)
	m.SentToScope = stringOrNull(m.SentToScope, a.SentToScope)

	var d diag.Diagnostics
	for _, l := range []struct {
		dst    *types.List
		values []string
	}{
		{&m.FromAddressContains, a.FromAddressContains},
		{&m.SubjectContains, a.SubjectContains},
		{&m.RecipientDomainIs, a.RecipientDomainIs},
		{&m.SenderDomainIs, a.SenderDomainIs},
		{&m.SubjectOrBodyContainsWords, a.SubjectOrBodyContainsWords},
		{&m.ExceptIfRecipientDomainIs, a.ExceptIfRecipientDomainIs},
	} {
		*l.dst, d = stringListOrNull(*l.dst, l.values)
		diags.Append(d...)
	}

	// Keep null when unset in config and false in Exchange, to avoid a
	// perpetual null -> false diff.
	if a.AttachmentIsExecutable || !m.AttachmentIsExecutable.IsNull() {
		m.AttachmentIsExecutable = types.BoolValue(a.AttachmentIsExecutable)
	}
	return diags
}

// stringOrNull returns a null string when Exchange has no value and the
// current value is null, so an unset optional attribute does not show a diff.
func stringOrNull(current types.String, value string) types.String {
	if value == "" && current.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(value)
}

// stringListOrNull returns a null list when Exchange has no values and the
// current value is null, so an unset optional list does not show a diff.
func stringListOrNull(current types.List, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 && current.IsNull() {
		return types.ListNull(types.StringType), nil
	}
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValue(types.StringType, elems)
}

// encodeStringList encodes a list as base64(JSON) for the bridge, so values
// containing commas, quotes or spaces are passed safely on the command line.
// Null or empty lists encode as "".
func encodeStringList(ctx context.Context, l types.List) (string, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() || len(l.Elements()) == 0 {
		return "", nil
	}
	var items []string
	diags := l.ElementsAs(ctx, &items, false)
	if diags.HasError() {
		return "", diags
	}
	raw, err := json.Marshal(items)
	if err != nil {
		diags.AddError("Failed to encode list", err.Error())
		return "", diags
	}
	return base64.StdEncoding.EncodeToString(raw), diags
}

// ModifyPlan marks the id as unknown when the rule is renamed. Exchange uses
// the rule name as its identity, so a rename changes the id; announcing the old
// id in the plan would make Terraform reject the apply result as inconsistent.
func (r *transportRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // create or destroy: nothing to adjust
	}
	var plan, state transportRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Name.IsUnknown() && !plan.Name.Equal(state.Name) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
	}
}
