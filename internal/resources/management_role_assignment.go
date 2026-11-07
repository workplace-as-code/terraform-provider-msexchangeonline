package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

var (
	_ resource.Resource                     = &managementRoleAssignmentResource{}
	_ resource.ResourceWithImportState      = &managementRoleAssignmentResource{}
	_ resource.ResourceWithValidateConfig   = &managementRoleAssignmentResource{}
	_ resource.ResourceWithConfigValidators = &managementRoleAssignmentResource{}
)

// managementRoleAssignmentResource gives a management role to a role group
// or to an application (RBAC for Applications), optionally limited by a
// scope. The scopes are authoritative: a scope added outside Terraform shows
// as a change, since the scope is what limits the assignment.
type managementRoleAssignmentResource struct {
	client *client.Client
}

type managementRoleAssignmentModel struct {
	ID                               types.String `tfsdk:"id"`
	Name                             types.String `tfsdk:"name"`
	Role                             types.String `tfsdk:"role"`
	RoleGroup                        types.String `tfsdk:"role_group"`
	App                              types.String `tfsdk:"app"`
	CustomRecipientWriteScope        types.String `tfsdk:"custom_recipient_write_scope"`
	ExclusiveRecipientWriteScope     types.String `tfsdk:"exclusive_recipient_write_scope"`
	CustomResourceScope              types.String `tfsdk:"custom_resource_scope"`
	RecipientAdministrativeUnitScope types.String `tfsdk:"recipient_administrative_unit_scope"`
	RecipientGroupScope              types.String `tfsdk:"recipient_group_scope"`
	RecipientRelativeWriteScope      types.String `tfsdk:"recipient_relative_write_scope"`
	Enabled                          types.Bool   `tfsdk:"enabled"`
}

type managementRoleAssignmentAPIResponse struct {
	Name                             string `json:"Name"`
	Role                             string `json:"Role"`
	RoleGroup                        string `json:"RoleGroup"`
	App                              string `json:"App"`
	CustomRecipientWriteScope        string `json:"CustomRecipientWriteScope"`
	ExclusiveRecipientWriteScope     string `json:"ExclusiveRecipientWriteScope"`
	CustomResourceScope              string `json:"CustomResourceScope"`
	RecipientAdministrativeUnitScope string `json:"RecipientAdministrativeUnitScope"`
	RecipientGroupScope              string `json:"RecipientGroupScope"`
	RecipientRelativeWriteScope      string `json:"RecipientRelativeWriteScope"`
	Enabled                          bool   `json:"Enabled"`
}

func NewManagementRoleAssignmentResource() resource.Resource {
	return &managementRoleAssignmentResource{}
}

func (r *managementRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_management_role_assignment"
}

func (r *managementRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()}
	scope := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, MarkdownDescription: desc,
			Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a management role assignment (`New-ManagementRoleAssignment`): gives a role to a " +
			"role group or to an application, optionally limited to the recipients of a scope. The scopes are " +
			"authoritative: a scope added outside Terraform shows as a change.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The assignment name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Name of the assignment. Exchange names it `<role>-<role group or app>` when it is " +
					"not set. Changing it creates a new assignment.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"role": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The management role: a built-in role, a `msexchangeonline_management_role`, or for an " +
					"application an `Application *` role such as `Application Mail.Read`. Changing it creates a new assignment.",
				PlanModifiers: replace,
			},
			"role_group": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Name of the role group that gets the role. Changing it creates a new assignment.",
				PlanModifiers:       replace,
			},
			"app": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Application (client) ID of the `msexchangeonline_service_principal` that gets the " +
					"role (RBAC for Applications). Changing it creates a new assignment.",
				PlanModifiers: replace,
			},
			"custom_recipient_write_scope": scope("Management scope (`msexchangeonline_management_scope`) that limits " +
				"the recipients the role can change."),
			"exclusive_recipient_write_scope": scope("Exclusive management scope that limits the recipients the role can change."),
			"custom_resource_scope": scope("Management scope that limits the mailboxes an application can reach. " +
				"Only with `app`."),
			"recipient_administrative_unit_scope": scope("Microsoft Entra administrative unit, by ID, that limits the " +
				"recipients the role can change."),
			"recipient_group_scope": scope("Group whose members are the only recipients the role can change."),
			"recipient_relative_write_scope": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Built-in recipient scope, for example `Organization` or `MyGAL`, when no other scope " +
					"is set. Exchange sets it to the type of the scope otherwise.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the assignment applies. A new assignment is enabled unless this is `false`.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *managementRoleAssignmentResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("role_group"), path.MatchRoot("app")),
	}
}

// ValidateConfig refuses scopes that do not fit the assignee.
func (r *managementRoleAssignmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg managementRoleAssignmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.CustomResourceScope.IsNull() && !cfg.RoleGroup.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("custom_resource_scope"), "custom_resource_scope needs an app",
			"custom_resource_scope limits the mailboxes of an application: set app instead of role_group.")
	}
	set := 0
	for _, s := range []types.String{cfg.CustomRecipientWriteScope, cfg.ExclusiveRecipientWriteScope,
		cfg.RecipientAdministrativeUnitScope, cfg.RecipientGroupScope} {
		if !s.IsNull() {
			set++
		}
	}
	if set > 1 {
		resp.Diagnostics.AddAttributeError(path.Root("custom_recipient_write_scope"), "More than one write scope",
			"Set at most one of custom_recipient_write_scope, exclusive_recipient_write_scope, "+
				"recipient_administrative_unit_scope and recipient_group_scope.")
	}
}

func (r *managementRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// values encodes the assignment for the script as base64 JSON. A scope left
// out is sent as "": the script removes it.
func (r *managementRoleAssignmentResource) values(m managementRoleAssignmentModel) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	v := map[string]any{
		"Role":                             m.Role.ValueString(),
		"RoleGroup":                        m.RoleGroup.ValueString(),
		"App":                              m.App.ValueString(),
		"CustomRecipientWriteScope":        m.CustomRecipientWriteScope.ValueString(),
		"ExclusiveRecipientWriteScope":     m.ExclusiveRecipientWriteScope.ValueString(),
		"CustomResourceScope":              m.CustomResourceScope.ValueString(),
		"RecipientAdministrativeUnitScope": m.RecipientAdministrativeUnitScope.ValueString(),
		"RecipientGroupScope":              m.RecipientGroupScope.ValueString(),
	}
	if !m.Name.IsUnknown() && !m.Name.IsNull() {
		v["Name"] = m.Name.ValueString()
	}
	if !m.RecipientRelativeWriteScope.IsUnknown() && !m.RecipientRelativeWriteScope.IsNull() {
		v["RecipientRelativeWriteScope"] = m.RecipientRelativeWriteScope.ValueString()
	}
	if !m.Enabled.IsUnknown() && !m.Enabled.IsNull() {
		v["Enabled"] = m.Enabled.ValueBool()
	}
	raw, err := json.Marshal(v)
	if err != nil {
		diags.AddError("Failed to encode the assignment", err.Error())
	}
	return base64.StdEncoding.EncodeToString(raw), diags
}

func (r *managementRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan managementRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Create", "", plan, &resp.State, &resp.Diagnostics)
}

func (r *managementRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state managementRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Update", state.ID.ValueString(), plan, &resp.State, &resp.Diagnostics)
}

func (r *managementRoleAssignmentResource) write(ctx context.Context, action, id string, plan managementRoleAssignmentModel, state stateSetter, diags diagAppender) {
	values, d := r.values(plan)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "management_role_assignment.ps1", map[string]string{
		"Action": action, "Identity": id, "Values": values,
	})
	if err != nil {
		diags.AddError("Failed to write management role assignment", err.Error())
		return
	}
	m, err := managementRoleAssignmentFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *managementRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior managementRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "management_role_assignment.ps1", map[string]string{
		"Action": "Get", "Identity": prior.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read management role assignment", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := managementRoleAssignmentFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *managementRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state managementRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "management_role_assignment.ps1", map[string]string{
		"Action": "Delete", "Identity": state.ID.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete management role assignment", err.Error())
	}
}

func (r *managementRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// managementRoleAssignmentFromAPI builds the state from the bridge response;
// prior keeps the configured case of the role and the assignee.
func managementRoleAssignmentFromAPI(prior managementRoleAssignmentModel, output []byte) (managementRoleAssignmentModel, error) {
	var a managementRoleAssignmentAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.ID = types.StringValue(a.Name)
	m.Name = types.StringValue(a.Name)
	m.Role = foldString(prior.Role, a.Role)
	m.RoleGroup = foldOrNull(prior.RoleGroup, a.RoleGroup)
	m.App = foldOrNull(prior.App, a.App)
	m.CustomRecipientWriteScope = foldOrNull(prior.CustomRecipientWriteScope, a.CustomRecipientWriteScope)
	m.ExclusiveRecipientWriteScope = foldOrNull(prior.ExclusiveRecipientWriteScope, a.ExclusiveRecipientWriteScope)
	m.CustomResourceScope = foldOrNull(prior.CustomResourceScope, a.CustomResourceScope)
	m.RecipientAdministrativeUnitScope = foldOrNull(prior.RecipientAdministrativeUnitScope, a.RecipientAdministrativeUnitScope)
	m.RecipientGroupScope = foldOrNull(prior.RecipientGroupScope, a.RecipientGroupScope)
	m.RecipientRelativeWriteScope = foldString(prior.RecipientRelativeWriteScope, a.RecipientRelativeWriteScope)
	m.Enabled = types.BoolValue(a.Enabled)
	return m, nil
}

// foldOrNull is foldString for an optional attribute: no value from Exchange
// gives null.
func foldOrNull(prior types.String, api string) types.String {
	if api == "" {
		return types.StringNull()
	}
	return foldString(prior, api)
}
