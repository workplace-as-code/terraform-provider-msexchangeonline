package resources

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

var (
	_ resource.Resource                = &applicationAccessPolicyResource{}
	_ resource.ResourceWithImportState = &applicationAccessPolicyResource{}
)

// applicationAccessPolicyResource limits the mailboxes an app with Microsoft
// Graph application permissions can reach (New-ApplicationAccessPolicy). Only
// the description can be changed in place.
type applicationAccessPolicyResource struct {
	client *client.Client
}

type applicationAccessPolicyModel struct {
	ID                 types.String `tfsdk:"id"`
	AppID              types.String `tfsdk:"app_id"`
	PolicyScopeGroupID types.String `tfsdk:"policy_scope_group_id"`
	AccessRight        types.String `tfsdk:"access_right"`
	Description        types.String `tfsdk:"description"`
}

type applicationAccessPolicyAPIResponse struct {
	Identity    string `json:"Identity"`
	AppID       string `json:"AppId"`
	AccessRight string `json:"AccessRight"`
	Description string `json:"Description"`
	// ScopeGroupEmail is only returned on the read that follows an import.
	ScopeGroupEmail string `json:"ScopeGroupEmail"`
}

func NewApplicationAccessPolicyResource() resource.Resource {
	return &applicationAccessPolicyResource{}
}

func (r *applicationAccessPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_access_policy"
}

func (r *applicationAccessPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an application access policy (`New-ApplicationAccessPolicy`), which limits the mailboxes an app with " +
			"Microsoft Graph application permissions (Mail.Read, Calendars.ReadWrite, ...) can access. Exchange Online only. " +
			"Changes can take up to an hour to apply. Microsoft recommends RBAC for Applications for new deployments.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The policy identity, `<tenant id>\\<policy id>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application (client) ID of the app registration. Changing it creates a new policy.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       replace,
			},
			"policy_scope_group_id": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Mail-enabled security group whose members the policy applies to: email address, name or object ID. " +
					"Changing it creates a new policy. An imported policy reads it as the group's primary email address: use that form " +
					"in the configuration, or the plan replaces the policy.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"access_right": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "`RestrictAccess`: the app can only access the group members. `DenyAccess`: the app can access every " +
					"mailbox except the group members. Changing it creates a new policy.",
				Validators:    []validator.String{stringvalidator.OneOfCaseInsensitive("RestrictAccess", "DenyAccess")},
				PlanModifiers: replace,
			},
			"description": optionalString("Description of the policy."),
		},
	}
}

func (r *applicationAccessPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

func (r *applicationAccessPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg applicationAccessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := map[string]string{
		"Action":             "Create",
		"AppId":              plan.AppID.ValueString(),
		"PolicyScopeGroupId": plan.PolicyScopeGroupID.ValueString(),
		"AccessRight":        plan.AccessRight.ValueString(),
	}
	putString(p, "Description", cfg.Description)
	r.write(ctx, p, plan, &resp.State, &resp.Diagnostics)
}

func (r *applicationAccessPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, cfg, state applicationAccessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := map[string]string{"Action": "Update", "Identity": state.ID.ValueString()}
	putString(p, "Description", cfg.Description)
	r.write(ctx, p, plan, &resp.State, &resp.Diagnostics)
}

func (r *applicationAccessPolicyResource) write(ctx context.Context, p map[string]string, plan applicationAccessPolicyModel, state stateSetter, diags diagAppender) {
	output, err := r.client.RunScript(ctx, "application_access_policy.ps1", p)
	if err != nil {
		diags.AddError("Failed to write application access policy", err.Error())
		return
	}
	m, err := applicationAccessPolicyFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *applicationAccessPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior applicationAccessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := map[string]string{"Action": "Get", "Identity": prior.ID.ValueString()}
	if prior.PolicyScopeGroupID.ValueString() == "" {
		params["ResolveScope"] = "true" // after an import: read the scope group
	}
	output, err := r.client.RunScript(ctx, "application_access_policy.ps1", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read application access policy", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := applicationAccessPolicyFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *applicationAccessPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationAccessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "application_access_policy.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete application access policy", err.Error())
	}
}

// ImportState accepts the policy identity, as shown by Get-ApplicationAccessPolicy.
// The read that follows records the scope group as its primary email address.
func (r *applicationAccessPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applicationAccessPolicyFromAPI builds the state from the bridge response.
// Exchange does not return the scope group in the form it was given, so the
// configured value is kept; after an import, the group's primary email address
// stands for it.
func applicationAccessPolicyFromAPI(prior applicationAccessPolicyModel, output []byte) (applicationAccessPolicyModel, error) {
	var a applicationAccessPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.ID = types.StringValue(a.Identity)
	m.AppID = foldString(prior.AppID, a.AppID)
	m.AccessRight = foldString(prior.AccessRight, a.AccessRight)
	m.Description = types.StringValue(a.Description)
	if m.PolicyScopeGroupID.ValueString() == "" {
		m.PolicyScopeGroupID = types.StringValue(a.ScopeGroupEmail)
	}
	return m, nil
}
