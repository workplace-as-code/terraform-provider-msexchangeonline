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

var _ resource.Resource = &dkimSigningConfigResource{}
var _ resource.ResourceWithImportState = &dkimSigningConfigResource{}

type dkimSigningConfigResource struct{ client *client.Client }

type dkimSigningConfigModel struct {
	ID                     types.String `tfsdk:"id"`
	DomainName             types.String `tfsdk:"domain_name"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	KeySize                types.Int64  `tfsdk:"key_size"`
	HeaderCanonicalization types.String `tfsdk:"header_canonicalization"`
	BodyCanonicalization   types.String `tfsdk:"body_canonicalization"`
	Selector1CNAME         types.String `tfsdk:"selector1_cname"`
	Selector2CNAME         types.String `tfsdk:"selector2_cname"`
}

type dkimSigningConfigAPIResponse struct {
	Identity               string `json:"Identity"`
	DomainName             string `json:"DomainName"`
	Enabled                bool   `json:"Enabled"`
	KeySize                int64  `json:"KeySize"`
	HeaderCanonicalization string `json:"HeaderCanonicalization"`
	BodyCanonicalization   string `json:"BodyCanonicalization"`
	Selector1CNAME         string `json:"Selector1CNAME"`
	Selector2CNAME         string `json:"Selector2CNAME"`
}

func NewDkimSigningConfigResource() resource.Resource { return &dkimSigningConfigResource{} }

func (r *dkimSigningConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dkim_signing_config"
}

func (r *dkimSigningConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages DKIM signing configuration for an Exchange Online accepted domain. A DKIM configuration cannot be deleted: creating the resource adopts the " +
			"existing configuration of the domain when there is one, and destroying it disables DKIM signing for the domain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{MarkdownDescription: "The unique identifier (same as `domain_name`).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain_name": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "The accepted domain to configure DKIM signing for (e.g. `contoso.com`).",
			},
			"enabled":                 optionalBool("Whether DKIM signing is enabled for the domain."),
			"key_size":                optionalInt64("DKIM key size in bits. Valid values: `1024`, `2048`. Reads as the key size of the selector in use after the last rotation, `0` when Exchange reports none. Only applies when the provider creates the configuration: Exchange cannot change the key size of an existing one, so a different value fails (rotate the key with `Rotate-DkimSigningConfig -KeySize`)."),
			"header_canonicalization": optionalString("Header canonicalization algorithm. Valid values: `Relaxed`, `Simple`."),
			"body_canonicalization":   optionalString("Body canonicalization algorithm. Valid values: `Relaxed`, `Simple`."),
			"selector1_cname": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "CNAME value to publish in DNS for selector 1 (read-only, set by Exchange Online).",
			},
			"selector2_cname": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "CNAME value to publish in DNS for selector 2 (read-only, set by Exchange Online).",
			},
		},
	}
}

func (r *dkimSigningConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dkimSigningConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg dkimSigningConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := map[string]string{"Action": "Create", "DomainName": cfg.DomainName.ValueString()}
	putBool(p, "Enabled", cfg.Enabled)
	putInt64(p, "KeySize", cfg.KeySize)
	putString(p, "HeaderCanonicalization", cfg.HeaderCanonicalization)
	putString(p, "BodyCanonicalization", cfg.BodyCanonicalization)
	output, err := r.client.RunScript(ctx, "dkim_signing_config.ps1", p)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create DKIM signing config", err.Error())
		return
	}
	var a dkimSigningConfigAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, dkimFromAPI(a))...)
}

func (r *dkimSigningConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dkimSigningConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "dkim_signing_config.ps1", map[string]string{
		"Action": "Get", "Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read DKIM signing config", err.Error())
		return
	}
	var a dkimSigningConfigAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, dkimFromAPI(a))...)
}

func (r *dkimSigningConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, state dkimSigningConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := map[string]string{"Action": "Update", "Identity": state.ID.ValueString()}
	putBool(p, "Enabled", cfg.Enabled)
	putString(p, "HeaderCanonicalization", cfg.HeaderCanonicalization)
	putString(p, "BodyCanonicalization", cfg.BodyCanonicalization)
	output, err := r.client.RunScript(ctx, "dkim_signing_config.ps1", p)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update DKIM signing config", err.Error())
		return
	}
	var a dkimSigningConfigAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		resp.Diagnostics.AddError("Failed to parse response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, dkimFromAPI(a))...)
}

func (r *dkimSigningConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dkimSigningConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "dkim_signing_config.ps1", map[string]string{
		"Action": "Delete", "Identity": state.ID.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to disable DKIM signing config", err.Error())
	}
}

func (r *dkimSigningConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func dkimFromAPI(a dkimSigningConfigAPIResponse) dkimSigningConfigModel {
	return dkimSigningConfigModel{
		ID:                     types.StringValue(a.Identity),
		DomainName:             types.StringValue(a.DomainName),
		Enabled:                types.BoolValue(a.Enabled),
		KeySize:                types.Int64Value(a.KeySize),
		HeaderCanonicalization: types.StringValue(a.HeaderCanonicalization),
		BodyCanonicalization:   types.StringValue(a.BodyCanonicalization),
		Selector1CNAME:         types.StringValue(a.Selector1CNAME),
		Selector2CNAME:         types.StringValue(a.Selector2CNAME),
	}
}
