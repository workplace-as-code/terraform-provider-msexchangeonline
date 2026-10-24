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

var _ resource.Resource = &inboundConnectorResource{}
var _ resource.ResourceWithImportState = &inboundConnectorResource{}

// inboundConnectorResource manages Exchange Online inbound mail flow connectors.
type inboundConnectorResource struct {
	client *client.Client
}

type inboundConnectorModel struct {
	ID                           types.String `tfsdk:"id"`
	Name                         types.String `tfsdk:"name"`
	Enabled                      types.Bool   `tfsdk:"enabled"`
	ConnectorType                types.String `tfsdk:"connector_type"`
	ConnectorSource              types.String `tfsdk:"connector_source"`
	SenderDomains                types.List   `tfsdk:"sender_domains"`
	SenderIPAddresses            types.List   `tfsdk:"sender_ip_addresses"`
	RequireTls                   types.Bool   `tfsdk:"require_tls"`
	RestrictDomainsToCertificate types.Bool   `tfsdk:"restrict_domains_to_certificate"`
	TlsSenderCertificateName     types.String `tfsdk:"tls_sender_certificate_name"`
	Comment                      types.String `tfsdk:"comment"`
}

type inboundConnectorAPIResponse struct {
	Identity                     string          `json:"Identity"`
	Name                         string          `json:"Name"`
	Enabled                      bool            `json:"Enabled"`
	ConnectorType                string          `json:"ConnectorType"`
	ConnectorSource              string          `json:"ConnectorSource"`
	SenderDomains                FlexStringSlice `json:"SenderDomains"`
	SenderIPAddresses            FlexStringSlice `json:"SenderIPAddresses"`
	RequireTls                   bool            `json:"RequireTls"`
	RestrictDomainsToCertificate bool            `json:"RestrictDomainsToCertificate"`
	TlsSenderCertificateName     string          `json:"TlsSenderCertificateName"`
	Comment                      string          `json:"Comment"`
}

func NewInboundConnectorResource() resource.Resource {
	return &inboundConnectorResource{}
}

func (r *inboundConnectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inbound_connector"
}

func (r *inboundConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Online inbound connector. Inbound connectors control how Exchange Online receives email from external systems such as on-premises Exchange servers or partner organizations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the inbound connector.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the inbound connector. Immutable after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": optionalBool("Whether the connector is enabled."),
			"connector_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The type of the connector. Valid values: `OnPremises`, `Partner`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"connector_source": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Read-only. How the connector was created. Values: `Default`, `HybridWizard`, `AdminUI`.",
			},
			"sender_domains":                  optionalList("Domain names from which this connector accepts messages (e.g. `contoso.com`). Required for `Partner` connectors."),
			"sender_ip_addresses":             optionalList("IP addresses or ranges from which this connector accepts messages (e.g. `203.0.113.0/24`). Typically used for `OnPremises` connectors."),
			"require_tls":                     optionalBool("Require TLS for inbound connections."),
			"restrict_domains_to_certificate": optionalBool("Verify that the TLS certificate used by the sender matches `tls_sender_certificate_name`."),
			"tls_sender_certificate_name":     optionalString("The certificate subject expected from the sending server when `restrict_domains_to_certificate = true`."),
			"comment":                         optionalString("Optional description of the connector."),
		},
	}
}

func (r *inboundConnectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *inboundConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan inboundConnectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg inboundConnectorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "inbound_connector.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create inbound connector", err.Error())
		return
	}

	var apiResp inboundConnectorAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, inboundConnectorFromAPI(apiResp))...)
}

func (r *inboundConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state inboundConnectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "inbound_connector.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read inbound connector", err.Error())
		return
	}

	var apiResp inboundConnectorAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, inboundConnectorFromAPI(apiResp))...)
}

func (r *inboundConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state inboundConnectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg inboundConnectorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "inbound_connector.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update inbound connector", err.Error())
		return
	}

	var apiResp inboundConnectorAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, inboundConnectorFromAPI(apiResp))...)
}

func (r *inboundConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state inboundConnectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "inbound_connector.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete inbound connector", err.Error())
	}
}

func (r *inboundConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *inboundConnectorResource) buildParams(ctx context.Context, action string, cfg inboundConnectorModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putBool(p, "Enabled", cfg.Enabled)
	putString(p, "ConnectorType", cfg.ConnectorType)
	putBool(p, "RequireTls", cfg.RequireTls)
	putBool(p, "RestrictDomainsToCertificate", cfg.RestrictDomainsToCertificate)
	putString(p, "TlsSenderCertificateName", cfg.TlsSenderCertificateName)
	putString(p, "Comment", cfg.Comment)
	if known(cfg.SenderDomains) {
		p["SenderDomains"] = listToCSV(ctx, cfg.SenderDomains)
	}
	if known(cfg.SenderIPAddresses) {
		p["SenderIPAddresses"] = listToCSV(ctx, cfg.SenderIPAddresses)
	}
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func inboundConnectorFromAPI(a inboundConnectorAPIResponse) inboundConnectorModel {
	return inboundConnectorModel{
		ID:                           types.StringValue(a.Identity),
		Name:                         types.StringValue(a.Name),
		Enabled:                      types.BoolValue(a.Enabled),
		ConnectorType:                types.StringValue(a.ConnectorType),
		ConnectorSource:              types.StringValue(a.ConnectorSource),
		SenderDomains:                stringsToList(a.SenderDomains),
		SenderIPAddresses:            stringsToList(a.SenderIPAddresses),
		RequireTls:                   types.BoolValue(a.RequireTls),
		RestrictDomainsToCertificate: types.BoolValue(a.RestrictDomainsToCertificate),
		TlsSenderCertificateName:     types.StringValue(a.TlsSenderCertificateName),
		Comment:                      types.StringValue(a.Comment),
	}
}
