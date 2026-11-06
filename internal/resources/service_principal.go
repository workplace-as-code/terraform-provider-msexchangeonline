package resources

import (
	"context"
	"encoding/base64"
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
	_ resource.Resource                = &servicePrincipalResource{}
	_ resource.ResourceWithImportState = &servicePrincipalResource{}
)

// servicePrincipalResource registers a Microsoft Entra application in Exchange
// Online (New-ServicePrincipal), so that it can get management roles: RBAC
// for Applications. The resource is identified by the app ID.
type servicePrincipalResource struct {
	client *client.Client
}

type servicePrincipalModel struct {
	ID          types.String `tfsdk:"id"`
	AppId       types.String `tfsdk:"app_id"`
	ObjectId    types.String `tfsdk:"object_id"`
	DisplayName types.String `tfsdk:"display_name"`
}

type servicePrincipalAPIResponse struct {
	AppId       string `json:"AppId"`
	ObjectId    string `json:"ObjectId"`
	DisplayName string `json:"DisplayName"`
}

func NewServicePrincipalResource() resource.Resource {
	return &servicePrincipalResource{}
}

func (r *servicePrincipalResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_principal"
}

func (r *servicePrincipalResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()}
	guid := []validator.String{stringvalidator.LengthAtLeast(1)}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers a Microsoft Entra application in Exchange Online (`New-ServicePrincipal`), so that " +
			"`msexchangeonline_management_role_assignment` can give it roles limited to a scope: RBAC for Applications. " +
			"The app registration itself, and its consent, are managed in Microsoft Entra.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The app ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application (client) ID of the app registration. Changing it creates a new service principal.",
				Validators:          guid,
				PlanModifiers:       replace,
			},
			"object_id": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Object ID of the app's enterprise application (service principal) in Microsoft Entra, " +
					"not the object ID of the app registration. Changing it creates a new service principal.",
				Validators:    guid,
				PlanModifiers: replace,
			},
			"display_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name shown in Exchange Online.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *servicePrincipalResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

func (r *servicePrincipalResource) run(ctx context.Context, action string, m servicePrincipalModel) ([]byte, error) {
	v := map[string]string{"ObjectId": m.ObjectId.ValueString()}
	if !m.DisplayName.IsUnknown() && !m.DisplayName.IsNull() {
		v["DisplayName"] = m.DisplayName.ValueString()
	}
	raw, _ := json.Marshal(v)
	return r.client.RunScript(ctx, "service_principal.ps1", map[string]string{
		"Action": action, "Identity": m.AppId.ValueString(), "Values": base64.StdEncoding.EncodeToString(raw),
	})
}

func (r *servicePrincipalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan servicePrincipalModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Create", plan, &resp.State, &resp.Diagnostics)
}

func (r *servicePrincipalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan servicePrincipalModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Update", plan, &resp.State, &resp.Diagnostics)
}

func (r *servicePrincipalResource) write(ctx context.Context, action string, plan servicePrincipalModel, state stateSetter, diags diagAppender) {
	output, err := r.run(ctx, action, plan)
	if err != nil {
		diags.AddError("Failed to write service principal", err.Error())
		return
	}
	m, err := servicePrincipalFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *servicePrincipalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior servicePrincipalModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "service_principal.ps1", map[string]string{"Action": "Get", "Identity": prior.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read service principal", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := servicePrincipalFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *servicePrincipalResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state servicePrincipalModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "service_principal.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete service principal", err.Error())
	}
}

// ImportState accepts the app ID.
func (r *servicePrincipalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), req.ID)...)
}

// servicePrincipalFromAPI builds the state; prior keeps the configured case of the IDs.
func servicePrincipalFromAPI(prior servicePrincipalModel, output []byte) (servicePrincipalModel, error) {
	var a servicePrincipalAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.AppId = foldString(prior.AppId, a.AppId)
	m.ID = m.AppId
	m.ObjectId = foldString(prior.ObjectId, a.ObjectId)
	m.DisplayName = types.StringValue(a.DisplayName)
	return m, nil
}
