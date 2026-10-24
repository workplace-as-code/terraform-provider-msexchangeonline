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

var _ resource.Resource = &quarantinePolicyResource{}
var _ resource.ResourceWithImportState = &quarantinePolicyResource{}

type quarantinePolicyResource struct{ client *client.Client }

type quarantinePolicyModel struct {
	ID                                types.String `tfsdk:"id"`
	Name                              types.String `tfsdk:"name"`
	EndUserQuarantinePermissionsValue types.Int64  `tfsdk:"end_user_quarantine_permissions_value"`
	ESNEnabled                        types.Bool   `tfsdk:"esn_enabled"`
	QuarantineRetentionDays           types.Int64  `tfsdk:"quarantine_retention_days"`
}

type quarantinePolicyAPIResponse struct {
	Identity                          string `json:"Identity"`
	Name                              string `json:"Name"`
	EndUserQuarantinePermissionsValue int64  `json:"EndUserQuarantinePermissionsValue"`
	ESNEnabled                        bool   `json:"ESNEnabled"`
	QuarantineRetentionDays           int64  `json:"QuarantineRetentionDays"`
}

func NewQuarantinePolicyResource() resource.Resource { return &quarantinePolicyResource{} }

func (r *quarantinePolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quarantine_policy"
}

func (r *quarantinePolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Online quarantine policy. Quarantine policies define what end users can do with quarantined messages.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier of the quarantine policy.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Name of the quarantine policy.",
			},
			"end_user_quarantine_permissions_value": optionalInt64("Bitmask integer defining end-user permissions. See Microsoft docs for values (e.g. `17` = view + release request)."),
			"esn_enabled":                           optionalBool("Enable end-user spam notifications for this quarantine policy."),
			"quarantine_retention_days":             optionalInt64("Number of days messages are held in quarantine (1–30)."),
		},
	}
}

func (r *quarantinePolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *quarantinePolicyResource) buildParams(action string, cfg quarantinePolicyModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putInt64(p, "EndUserQuarantinePermissionsValue", cfg.EndUserQuarantinePermissionsValue)
	putBool(p, "ESNEnabled", cfg.ESNEnabled)
	putInt64(p, "QuarantineRetentionDays", cfg.QuarantineRetentionDays)
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func (r *quarantinePolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan quarantinePolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg quarantinePolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "quarantine_policy.ps1", r.buildParams("Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create quarantine policy", err.Error())
		return
	}
	var a quarantinePolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, quarantineFromAPI(a))...)
}

func (r *quarantinePolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state quarantinePolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "quarantine_policy.ps1", map[string]string{
		"Action": "Get", "Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read quarantine policy", err.Error())
		return
	}
	var a quarantinePolicyAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, quarantineFromAPI(a))...)
}

func (r *quarantinePolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state quarantinePolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg quarantinePolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "quarantine_policy.ps1", r.buildParams("Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update quarantine policy", err.Error())
		return
	}
	var a quarantinePolicyAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, quarantineFromAPI(a))...)
}

func (r *quarantinePolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state quarantinePolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "quarantine_policy.ps1", map[string]string{
		"Action": "Delete", "Identity": state.ID.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete quarantine policy", err.Error())
	}
}

func (r *quarantinePolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func quarantineFromAPI(a quarantinePolicyAPIResponse) quarantinePolicyModel {
	return quarantinePolicyModel{
		ID:                                types.StringValue(a.Identity),
		Name:                              types.StringValue(a.Name),
		EndUserQuarantinePermissionsValue: types.Int64Value(a.EndUserQuarantinePermissionsValue),
		ESNEnabled:                        types.BoolValue(a.ESNEnabled),
		QuarantineRetentionDays:           types.Int64Value(a.QuarantineRetentionDays),
	}
}
