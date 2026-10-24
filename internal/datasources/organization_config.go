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

var _ datasource.DataSource = &organizationConfigDataSource{}

type organizationConfigDataSource struct {
	client *client.Client
}

// organizationConfigDataModel is the read-only state returned to Terraform
type organizationConfigDataModel struct {
	ID                           types.String `tfsdk:"id"`
	DisplayName                  types.String `tfsdk:"display_name"`
	SCLJunkThreshold             types.Int64  `tfsdk:"scl_junk_threshold"`
	MaxSendSize                  types.Int64  `tfsdk:"max_send_size"`
	MaxReceiveSize               types.Int64  `tfsdk:"max_receive_size"`
	AuditDisabled                types.Bool   `tfsdk:"audit_disabled"`
	OutboundSpamFilteringEnabled types.Bool   `tfsdk:"outbound_spam_filtering_enabled"`
	ReadTrackingEnabled          types.Bool   `tfsdk:"read_tracking_enabled"`
	FocusedInboxOn               types.Bool   `tfsdk:"focused_inbox_on"`
}

type organizationConfigAPIResponse struct {
	Identity                     string `json:"Identity"`
	DisplayName                  string `json:"DisplayName"`
	SCLJunkThreshold             int64  `json:"SCLJunkThreshold"`
	MaxSendSize                  int64  `json:"MaxSendSize"`
	MaxReceiveSize               int64  `json:"MaxReceiveSize"`
	AuditDisabled                bool   `json:"AuditDisabled"`
	OutboundSpamFilteringEnabled bool   `json:"OutboundSpamFilteringEnabled"`
	ReadTrackingEnabled          bool   `json:"ReadTrackingEnabled"`
	FocusedInboxOn               bool   `json:"FocusedInboxOn"`
}

func NewOrganizationConfigDataSource() datasource.DataSource {
	return &organizationConfigDataSource{}
}

func (d *organizationConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_config"
}

func (d *organizationConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the Exchange Online organization configuration. Use this data source to reference tenant-level settings in other resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Exchange identity of the organization configuration object.",
			},
			"display_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The display name of the organization.",
			},
			"scl_junk_threshold": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The spam confidence level (SCL) threshold above which messages are moved to the Junk Email folder.",
			},
			"max_send_size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum message send size in bytes. `-1` means unlimited.",
			},
			"max_receive_size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum message receive size in bytes. `-1` means unlimited.",
			},
			"audit_disabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether mailbox audit logging is disabled at the organization level.",
			},
			"outbound_spam_filtering_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether outbound spam filtering is enabled.",
			},
			"read_tracking_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether read receipt tracking is enabled for the organization.",
			},
			"focused_inbox_on": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether Focused Inbox is enabled for the organization.",
			},
		},
	}
}

func (d *organizationConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *organizationConfigDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	output, err := d.client.RunScript(ctx, "organization_config.ps1", map[string]string{
		"Action": "Get",
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read organization config", err.Error())
		return
	}

	var apiResp organizationConfigAPIResponse
	if err := json.Unmarshal(output, &apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationConfigDataModel{
		ID:                           types.StringValue(apiResp.Identity),
		DisplayName:                  types.StringValue(apiResp.DisplayName),
		SCLJunkThreshold:             types.Int64Value(apiResp.SCLJunkThreshold),
		MaxSendSize:                  types.Int64Value(apiResp.MaxSendSize),
		MaxReceiveSize:               types.Int64Value(apiResp.MaxReceiveSize),
		AuditDisabled:                types.BoolValue(apiResp.AuditDisabled),
		OutboundSpamFilteringEnabled: types.BoolValue(apiResp.OutboundSpamFilteringEnabled),
		ReadTrackingEnabled:          types.BoolValue(apiResp.ReadTrackingEnabled),
		FocusedInboxOn:               types.BoolValue(apiResp.FocusedInboxOn),
	})...)
}
