package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"

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
	_ resource.Resource                = &roleAssignmentPolicyResource{}
	_ resource.ResourceWithImportState = &roleAssignmentPolicyResource{}
)

// roleAssignmentPolicyResource manages a role assignment policy: the end-user
// roles (MyBaseOptions, ...) that set what users can change on their own
// mailbox. The policy owns its role assignments: the script adds and removes
// them to match roles.
type roleAssignmentPolicyResource struct {
	client *client.Client
}

type roleAssignmentPolicyModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsDefault   types.Bool   `tfsdk:"is_default"`
	Roles       types.Set    `tfsdk:"roles"`
}

type roleAssignmentPolicyAPIResponse struct {
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	IsDefault   bool     `json:"IsDefault"`
	Roles       []string `json:"Roles"`
}

func NewRoleAssignmentPolicyResource() resource.Resource {
	return &roleAssignmentPolicyResource{}
}

func (r *roleAssignmentPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_assignment_policy"
}

func (r *roleAssignmentPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a role assignment policy (`New-RoleAssignmentPolicy`): the end-user roles that set what " +
			"users can change on their own mailbox, such as their contact information or their distribution groups. " +
			"Assigning the policy to mailboxes is not managed; mailboxes without a policy of their own use the default one.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The policy name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the policy, for example `Default Role Assignment Policy`. Changing it creates a new policy.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Description of the policy.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"is_default": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Whether new mailboxes get this policy. Making a policy the default removes the flag " +
					"from the previous default policy.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"roles": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				MarkdownDescription: "End-user roles of the policy, for example `MyBaseOptions`, `MyContactInformation`, " +
					"`MyDistributionGroupMembership`. A role left out is removed from the policy.",
			},
		},
	}
}

func (r *roleAssignmentPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

func (r *roleAssignmentPolicyResource) values(ctx context.Context, m roleAssignmentPolicyModel) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	var roles []string
	diags.Append(m.Roles.ElementsAs(ctx, &roles, false)...)
	sort.Strings(roles)
	v := map[string]any{"Roles": roles}
	if !m.Description.IsUnknown() && !m.Description.IsNull() {
		v["Description"] = m.Description.ValueString()
	}
	if !m.IsDefault.IsUnknown() && !m.IsDefault.IsNull() {
		v["IsDefault"] = m.IsDefault.ValueBool()
	}
	raw, err := json.Marshal(v)
	if err != nil {
		diags.AddError("Failed to encode the policy", err.Error())
	}
	return base64.StdEncoding.EncodeToString(raw), diags
}

func (r *roleAssignmentPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleAssignmentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Create", plan, &resp.State, &resp.Diagnostics)
}

func (r *roleAssignmentPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan roleAssignmentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Update", plan, &resp.State, &resp.Diagnostics)
}

func (r *roleAssignmentPolicyResource) write(ctx context.Context, action string, plan roleAssignmentPolicyModel, state stateSetter, diags diagAppender) {
	values, d := r.values(ctx, plan)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "role_assignment_policy.ps1", map[string]string{
		"Action": action, "Identity": plan.Name.ValueString(), "Values": values,
	})
	if err != nil {
		diags.AddError("Failed to write role assignment policy", err.Error())
		return
	}
	m, err := roleAssignmentPolicyFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *roleAssignmentPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior roleAssignmentPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "role_assignment_policy.ps1", map[string]string{"Action": "Get", "Identity": prior.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read role assignment policy", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := roleAssignmentPolicyFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *roleAssignmentPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleAssignmentPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "role_assignment_policy.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete role assignment policy", err.Error())
	}
}

func (r *roleAssignmentPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// roleAssignmentPolicyFromAPI builds the state; prior keeps the configured case.
func roleAssignmentPolicyFromAPI(prior roleAssignmentPolicyModel, output []byte) (roleAssignmentPolicyModel, error) {
	var a roleAssignmentPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.Name = foldString(prior.Name, a.Name)
	m.ID = m.Name
	m.Description = types.StringValue(a.Description)
	m.IsDefault = types.BoolValue(a.IsDefault)
	m.Roles = foldSet(prior.Roles, a.Roles)
	return m, nil
}
