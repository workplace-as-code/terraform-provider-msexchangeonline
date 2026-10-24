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

var _ resource.Resource = &emailAddressPolicyResource{}
var _ resource.ResourceWithImportState = &emailAddressPolicyResource{}

type emailAddressPolicyResource struct{ client *client.Client }

type emailAddressPolicyModel struct {
	ID                           types.String `tfsdk:"id"`
	Name                         types.String `tfsdk:"name"`
	EnabledEmailAddressTemplates types.List   `tfsdk:"enabled_email_address_templates"`
	Priority                     types.Int64  `tfsdk:"priority"`
}

type emailAddressPolicyAPIResponse struct {
	Identity                     string          `json:"Identity"`
	Name                         string          `json:"Name"`
	EnabledEmailAddressTemplates FlexStringSlice `json:"EnabledEmailAddressTemplates"`
	Priority                     int64           `json:"Priority"`
}

func NewEmailAddressPolicyResource() resource.Resource { return &emailAddressPolicyResource{} }

func (r *emailAddressPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_address_policy"
}

func (r *emailAddressPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange email address policy, which defines how email addresses are generated for recipients. In Exchange Online, email address policies only apply to Microsoft 365 Groups.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier of the email address policy.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Name of the email address policy."},
			"enabled_email_address_templates": optionalList("SMTP address templates (e.g. `SMTP:%g.%s@contoso.com`). The primary address uses `SMTP:` (uppercase)."),
			"priority":                        optionalInt64("Priority of the policy. Lower numbers take precedence. The default policy always comes last and reads as `0`."),
		},
	}
}

func (r *emailAddressPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *emailAddressPolicyResource) buildParams(ctx context.Context, action string, cfg emailAddressPolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	if known(cfg.EnabledEmailAddressTemplates) {
		p["EnabledEmailAddressTemplates"] = listToCSV(ctx, cfg.EnabledEmailAddressTemplates)
	}
	putInt64(p, "Priority", cfg.Priority)
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func (r *emailAddressPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan emailAddressPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg emailAddressPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "email_address_policy.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create email address policy", err.Error())
		return
	}
	var a emailAddressPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, emailAddressPolicyFromAPI(a))...)
}

func (r *emailAddressPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state emailAddressPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "email_address_policy.ps1", map[string]string{"Action": "Get", "Identity": state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read email address policy", err.Error())
		return
	}
	var a emailAddressPolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, emailAddressPolicyFromAPI(a))...)
}

func (r *emailAddressPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state emailAddressPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg emailAddressPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "email_address_policy.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update email address policy", err.Error())
		return
	}
	var a emailAddressPolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, emailAddressPolicyFromAPI(a))...)
}

func (r *emailAddressPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state emailAddressPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "email_address_policy.ps1", map[string]string{"Action": "Delete", "Identity": state.ID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete email address policy", err.Error())
	}
}

func (r *emailAddressPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func emailAddressPolicyFromAPI(a emailAddressPolicyAPIResponse) emailAddressPolicyModel {
	return emailAddressPolicyModel{
		ID:                           types.StringValue(a.Identity),
		Name:                         types.StringValue(a.Name),
		EnabledEmailAddressTemplates: stringsToList(a.EnabledEmailAddressTemplates),
		Priority:                     types.Int64Value(a.Priority),
	}
}
