package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &managementRoleResource{}
	_ resource.ResourceWithImportState = &managementRoleResource{}
)

// managementRoleResource manages a custom management role: a copy of a
// built-in (parent) role that keeps only some of its cmdlets and parameters.
// The script works out which role entries to add, remove or restrict.
type managementRoleResource struct {
	client *client.Client
}

type managementRoleModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Parent           types.String `tfsdk:"parent"`
	Description      types.String `tfsdk:"description"`
	Cmdlets          types.Set    `tfsdk:"cmdlets"`
	CmdletParameters types.Map    `tfsdk:"cmdlet_parameters"`
}

type managementRoleAPIResponse struct {
	Name             string              `json:"Name"`
	Parent           string              `json:"Parent"`
	Description      string              `json:"Description"`
	Cmdlets          []string            `json:"Cmdlets"`
	CmdletParameters map[string][]string `json:"CmdletParameters"`
}

// cmdletParametersType is the type of cmdlet_parameters: cmdlet -> set of parameters.
var cmdletParametersType = types.SetType{ElemType: types.StringType}

func NewManagementRoleResource() resource.Resource {
	return &managementRoleResource{}
}

func (r *managementRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_management_role"
}

func (r *managementRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom management role (`New-ManagementRole -Parent`): a copy of a built-in role " +
			"that keeps only some of its cmdlets, and optionally only some of their parameters. Assign it with " +
			"`msexchangeonline_management_role_assignment`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The role name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the role. Changing it creates a new role.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"parent": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Role the custom role is copied from, for example `Mail Recipients`: a built-in role or " +
					"another custom role. Changing it creates a new role.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Description of the role.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cmdlets": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				MarkdownDescription: "Cmdlets the role keeps from its parent, for example `Get-Mailbox`. A role can only " +
					"keep cmdlets of its parent: it never grants more.",
				Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"cmdlet_parameters": schema.MapAttribute{
				ElementType: cmdletParametersType,
				Optional:    true,
				MarkdownDescription: "Parameters kept for some cmdlets, by cmdlet, for example " +
					"`{ \"Set-User\" = [\"Identity\", \"Phone\"] }`. Include the parameters the cmdlet needs to run, such as " +
					"`Identity`. The cmdlets of `cmdlets` that are not in the map keep all the parameters of the parent.",
			},
		},
	}
}

func (r *managementRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// values encodes the role for the script: base64 JSON of Parent, Description,
// Cmdlets and CmdletParameters.
func (r *managementRoleResource) values(ctx context.Context, m managementRoleModel) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	v := map[string]any{"Parent": m.Parent.ValueString()}
	if !m.Description.IsUnknown() && !m.Description.IsNull() {
		v["Description"] = m.Description.ValueString()
	}
	var cmdlets []string
	diags.Append(m.Cmdlets.ElementsAs(ctx, &cmdlets, false)...)
	sort.Strings(cmdlets)
	v["Cmdlets"] = cmdlets
	params := map[string][]string{}
	if !m.CmdletParameters.IsNull() && !m.CmdletParameters.IsUnknown() {
		diags.Append(m.CmdletParameters.ElementsAs(ctx, &params, false)...)
	}
	v["CmdletParameters"] = params
	raw, err := json.Marshal(v)
	if err != nil {
		diags.AddError("Failed to encode the role", err.Error())
	}
	return base64.StdEncoding.EncodeToString(raw), diags
}

func (r *managementRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan managementRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Create", plan, &resp.State, &resp.Diagnostics)
}

func (r *managementRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan managementRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Update", plan, &resp.State, &resp.Diagnostics)
}

func (r *managementRoleResource) write(ctx context.Context, action string, plan managementRoleModel, state stateSetter, diags diagAppender) {
	values, d := r.values(ctx, plan)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "management_role.ps1", map[string]string{
		"Action": action, "Identity": plan.Name.ValueString(), "Values": values,
	})
	if err != nil {
		diags.AddError("Failed to write management role", err.Error())
		return
	}
	m, err := managementRoleFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *managementRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior managementRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "management_role.ps1", map[string]string{
		"Action": "Get", "Identity": prior.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read management role", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := managementRoleFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *managementRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state managementRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "management_role.ps1", map[string]string{
		"Action": "Delete", "Identity": state.ID.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete management role", err.Error())
	}
}

func (r *managementRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// managementRoleFromAPI builds the state from the bridge response. Exchange
// spells cmdlets, parameters and the parent its own way: prior keeps the
// configured spelling when only the case differs.
func managementRoleFromAPI(prior managementRoleModel, output []byte) (managementRoleModel, error) {
	var a managementRoleAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.ID = types.StringValue(a.Name)
	m.Name = types.StringValue(a.Name)
	m.Parent = foldString(prior.Parent, a.Parent)
	m.Description = types.StringValue(a.Description)
	m.Cmdlets = foldSet(prior.Cmdlets, a.Cmdlets)

	if len(a.CmdletParameters) == 0 && (prior.CmdletParameters.IsNull() || prior.CmdletParameters.IsUnknown()) {
		m.CmdletParameters = types.MapNull(cmdletParametersType)
		return m, nil
	}
	priorParams := map[string]types.Set{}
	if !prior.CmdletParameters.IsNull() && !prior.CmdletParameters.IsUnknown() {
		for k, v := range prior.CmdletParameters.Elements() {
			if s, ok := v.(types.Set); ok {
				priorParams[k] = s
			}
		}
	}
	elems := map[string]attr.Value{}
	for cmdlet, params := range a.CmdletParameters {
		key, set := cmdlet, types.SetNull(types.StringType)
		for k, s := range priorParams {
			if strings.EqualFold(k, cmdlet) {
				key, set = k, s
			}
		}
		elems[key] = foldSet(set, params)
	}
	m.CmdletParameters = types.MapValueMust(cmdletParametersType, elems)
	return m, nil
}

// foldSet returns the values from Exchange as a set, spelled as in prior when
// only the case differs.
func foldSet(prior types.Set, values []string) types.Set {
	known := map[string]string{}
	if !prior.IsNull() && !prior.IsUnknown() {
		for _, v := range prior.Elements() {
			if s, ok := v.(types.String); ok {
				known[strings.ToLower(s.ValueString())] = s.ValueString()
			}
		}
	}
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		if k, ok := known[strings.ToLower(v)]; ok {
			v = k
		}
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}
