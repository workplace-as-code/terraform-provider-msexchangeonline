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

var _ resource.Resource = &outboundConnectorResource{}
var _ resource.ResourceWithImportState = &outboundConnectorResource{}

// outboundConnectorResource manages Exchange Online outbound mail flow connectors.
type outboundConnectorResource struct {
	client *client.Client
}

type outboundConnectorModel struct {
	ID                            types.String `tfsdk:"id"`
	Name                          types.String `tfsdk:"name"`
	Enabled                       types.Bool   `tfsdk:"enabled"`
	ConnectorType                 types.String `tfsdk:"connector_type"`
	ConnectorSource               types.String `tfsdk:"connector_source"`
	RecipientDomains              types.List   `tfsdk:"recipient_domains"`
	SmartHosts                    types.List   `tfsdk:"smart_hosts"`
	TlsSettings                   types.String `tfsdk:"tls_settings"`
	RouteAllMessagesViaOnPremises types.Bool   `tfsdk:"route_all_messages_via_on_premises"`
	UseMxRecord                   types.Bool   `tfsdk:"use_mx_record"`
	Comment                       types.String `tfsdk:"comment"`
}

type outboundConnectorAPIResponse struct {
	Identity                      string          `json:"Identity"`
	Name                          string          `json:"Name"`
	Enabled                       bool            `json:"Enabled"`
	ConnectorType                 string          `json:"ConnectorType"`
	ConnectorSource               string          `json:"ConnectorSource"`
	RecipientDomains              FlexStringSlice `json:"RecipientDomains"`
	SmartHosts                    FlexStringSlice `json:"SmartHosts"`
	TlsSettings                   string          `json:"TlsSettings"`
	RouteAllMessagesViaOnPremises bool            `json:"RouteAllMessagesViaOnPremises"`
	UseMxRecord                   bool            `json:"UseMxRecord"`
	Comment                       string          `json:"Comment"`
}

func NewOutboundConnectorResource() resource.Resource {
	return &outboundConnectorResource{}
}

func (r *outboundConnectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outbound_connector"
}

func (r *outboundConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Exchange Online outbound connector. Outbound connectors control how Exchange Online sends email to external systems such as on-premises Exchange servers or partner organizations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the outbound connector.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the outbound connector. Immutable after creation.",
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
			"recipient_domains":                  optionalList("Domain names to which this connector routes messages (e.g. `contoso.com`). Required for `Partner` connectors."),
			"smart_hosts":                        optionalList("Smart host FQDNs or IP addresses used to route outbound mail (e.g. `mail.contoso.local`). Typically used for `OnPremises` connectors."),
			"tls_settings":                       optionalString("TLS requirement for outbound connections. Valid values: `None`, `EncryptionOnly`, `CertificateVerification`, `DomainValidation`."),
			"route_all_messages_via_on_premises": optionalBool("Route all outbound internet mail through on-premises hosts."),
			"use_mx_record":                      optionalBool("Use MX records instead of smart hosts to route mail."),
			"comment":                            optionalString("Optional description of the connector."),
		},
	}
}

func (r *outboundConnectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *outboundConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan outboundConnectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg outboundConnectorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "outbound_connector.ps1", r.buildParams(ctx, "Create", cfg, ""))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create outbound connector", err.Error())
		return
	}

	var apiResp outboundConnectorAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, outboundConnectorFromAPI(apiResp))...)
}

func (r *outboundConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state outboundConnectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "outbound_connector.ps1", map[string]string{
		"Action":   "Get",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read outbound connector", err.Error())
		return
	}

	var apiResp outboundConnectorAPIResponse
	if client.IsNotFound(output) {
		// The bridge returned nothing: the object was deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, outboundConnectorFromAPI(apiResp))...)
}

func (r *outboundConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state outboundConnectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Only the attributes set in the configuration are sent to Exchange.
	var cfg outboundConnectorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.client.RunScript(ctx, "outbound_connector.ps1", r.buildParams(ctx, "Update", cfg, state.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update outbound connector", err.Error())
		return
	}

	var apiResp outboundConnectorAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, outboundConnectorFromAPI(apiResp))...)
}

func (r *outboundConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state outboundConnectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunScript(ctx, "outbound_connector.ps1", map[string]string{
		"Action":   "Delete",
		"Identity": state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete outbound connector", err.Error())
	}
}

func (r *outboundConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *outboundConnectorResource) buildParams(ctx context.Context, action string, cfg outboundConnectorModel, identity string) map[string]string {
	p := map[string]string{"Action": action, "Name": cfg.Name.ValueString()}
	putBool(p, "Enabled", cfg.Enabled)
	putString(p, "ConnectorType", cfg.ConnectorType)
	putString(p, "TlsSettings", cfg.TlsSettings)
	putBool(p, "RouteAllMessagesViaOnPremises", cfg.RouteAllMessagesViaOnPremises)
	putBool(p, "UseMxRecord", cfg.UseMxRecord)
	putString(p, "Comment", cfg.Comment)
	if known(cfg.RecipientDomains) {
		p["RecipientDomains"] = listToCSV(ctx, cfg.RecipientDomains)
	}
	if known(cfg.SmartHosts) {
		p["SmartHosts"] = listToCSV(ctx, cfg.SmartHosts)
	}
	if identity != "" {
		p["Identity"] = identity
	}
	return p
}

func outboundConnectorFromAPI(a outboundConnectorAPIResponse) outboundConnectorModel {
	return outboundConnectorModel{
		ID:                            types.StringValue(a.Identity),
		Name:                          types.StringValue(a.Name),
		Enabled:                       types.BoolValue(a.Enabled),
		ConnectorType:                 types.StringValue(a.ConnectorType),
		ConnectorSource:               types.StringValue(a.ConnectorSource),
		RecipientDomains:              stringsToList(a.RecipientDomains),
		SmartHosts:                    stringsToList(a.SmartHosts),
		TlsSettings:                   types.StringValue(a.TlsSettings),
		RouteAllMessagesViaOnPremises: types.BoolValue(a.RouteAllMessagesViaOnPremises),
		UseMxRecord:                   types.BoolValue(a.UseMxRecord),
		Comment:                       types.StringValue(a.Comment),
	}
}
