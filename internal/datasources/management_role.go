package datasources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

var _ datasource.DataSource = &managementRoleDataSource{}

// managementRoleDataSource reads a management role, built-in roles included:
// the management_role resource refuses them.
type managementRoleDataSource struct {
	client *client.Client
}

type managementRoleDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Parent      types.String `tfsdk:"parent"`
	BuiltIn     types.Bool   `tfsdk:"built_in"`
	Cmdlets     types.Set    `tfsdk:"cmdlets"`
}

type managementRoleAPIResponse struct {
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	Parent      string   `json:"Parent"`
	BuiltIn     bool     `json:"BuiltIn"`
	Cmdlets     []string `json:"Cmdlets"`
}

func NewManagementRoleDataSource() datasource.DataSource {
	return &managementRoleDataSource{}
}

func (d *managementRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_management_role"
}

func (d *managementRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an Exchange management role, built-in or custom, and the cmdlets it contains (`Get-ManagementRole`, `Get-ManagementRoleEntry`).",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "The name of the role."},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "The name of the role to read."},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "The description of the role."},
			"parent":      schema.StringAttribute{Computed: true, MarkdownDescription: "The role this role derives from. Empty for a built-in root role."},
			"built_in":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether Microsoft provides the role."},
			"cmdlets": schema.SetAttribute{
				ElementType: types.StringType, Computed: true,
				MarkdownDescription: "The cmdlets the role contains.",
			},
		},
	}
}

func (d *managementRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = apiClient
}

func (d *managementRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config managementRoleDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := d.client.RunScript(ctx, "management_role_info.ps1", map[string]string{"Name": config.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read management role", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.Diagnostics.AddError("Management role not found",
			fmt.Sprintf("No management role named %q exists in Exchange Online.", config.Name.ValueString()))
		return
	}
	var api managementRoleAPIResponse
	if err := json.Unmarshal(output, &api); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	cmdlets, diags := types.SetValueFrom(ctx, types.StringType, api.Cmdlets)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, managementRoleDataModel{
		ID: types.StringValue(api.Name), Name: types.StringValue(api.Name), Description: types.StringValue(api.Description),
		Parent: types.StringValue(api.Parent), BuiltIn: types.BoolValue(api.BuiltIn), Cmdlets: cmdlets,
	})...)
}
