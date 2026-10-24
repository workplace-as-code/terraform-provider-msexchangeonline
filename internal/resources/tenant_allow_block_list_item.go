package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	_ resource.Resource                   = &tenantAllowBlockListItemResource{}
	_ resource.ResourceWithImportState    = &tenantAllowBlockListItemResource{}
	_ resource.ResourceWithValidateConfig = &tenantAllowBlockListItemResource{}
)

// tenantAllowBlockListItemResource manages one entry of the Tenant
// Allow/Block List (New-TenantAllowBlockListItems). An entry is identified by
// its list type and value.
type tenantAllowBlockListItemResource struct {
	client *client.Client
}

type tenantAllowBlockListItemModel struct {
	ID             types.String `tfsdk:"id"`
	ListType       types.String `tfsdk:"list_type"`
	Entry          types.String `tfsdk:"entry"`
	Action         types.String `tfsdk:"action"`
	Notes          types.String `tfsdk:"notes"`
	ExpirationDate types.String `tfsdk:"expiration_date"`
}

type tenantAllowBlockListItemAPIResponse struct {
	Value          string `json:"Value"`
	Action         string `json:"Action"`
	Notes          string `json:"Notes"`
	ExpirationDate string `json:"ExpirationDate"`
}

func NewTenantAllowBlockListItemResource() resource.Resource {
	return &tenantAllowBlockListItemResource{}
}

func (r *tenantAllowBlockListItemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_allow_block_list_item"
}

func (r *tenantAllowBlockListItemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an entry of the Tenant Allow/Block List (`New-TenantAllowBlockListItems`): a sender, URL, file hash or IP address " +
			"that Exchange Online Protection always blocks or allows. Exchange Online only.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`<list_type>:<entry>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"list_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "List the entry belongs to. Valid values: `Sender` (email address or domain), `Url`, `FileHash` (SHA256), `IP` (IPv6 address or range).",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive("Sender", "Url", "FileHash", "IP")},
				PlanModifiers:       replace,
			},
			"entry": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The sender, URL, file hash or IP address. Changing it creates a new entry.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       replace,
			},
			"action": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`Block` or `Allow`. Changing it creates a new entry.",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive("Block", "Allow")},
				PlanModifiers:       replace,
			},
			"notes": optionalString("Free text describing the entry."),
			"expiration_date": optionalString("Date the entry expires, in RFC 3339 format and UTC, for example `2027-01-31T00:00:00Z`. " +
				"When omitted at creation, a `Block` entry never expires (empty value) and an `Allow` entry is removed 45 days after it was last used."),
		},
	}
}

func (r *tenantAllowBlockListItemResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg tenantAllowBlockListItemModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.ExpirationDate.IsNull() || cfg.ExpirationDate.IsUnknown() {
		return
	}
	t, err := time.Parse(time.RFC3339, cfg.ExpirationDate.ValueString())
	if err != nil || t.Location() != time.UTC {
		resp.Diagnostics.AddAttributeError(path.Root("expiration_date"), "Invalid expiration date",
			fmt.Sprintf("%q is not an RFC 3339 date in UTC, for example 2027-01-31T00:00:00Z.", cfg.ExpirationDate.ValueString()))
	}
}

func (r *tenantAllowBlockListItemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// params sends the attributes set in the configuration.
func (r *tenantAllowBlockListItemResource) params(action string, cfg tenantAllowBlockListItemModel) map[string]string {
	p := map[string]string{
		"Action":   action,
		"ListType": cfg.ListType.ValueString(),
		"Entry":    cfg.Entry.ValueString(),
	}
	putString(p, "ListAction", cfg.Action)
	putString(p, "Notes", cfg.Notes)
	putString(p, "ExpirationDate", cfg.ExpirationDate)
	return p
}

func (r *tenantAllowBlockListItemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg tenantAllowBlockListItemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Create", plan, cfg, &resp.State, &resp.Diagnostics)
}

func (r *tenantAllowBlockListItemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, cfg tenantAllowBlockListItemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, "Update", plan, cfg, &resp.State, &resp.Diagnostics)
}

func (r *tenantAllowBlockListItemResource) write(ctx context.Context, action string, plan, cfg tenantAllowBlockListItemModel, state stateSetter, diags diagAppender) {
	output, err := r.client.RunScript(ctx, "tenant_allow_block_list_item.ps1", r.params(action, cfg))
	if err != nil {
		diags.AddError("Failed to write Tenant Allow/Block List entry", err.Error())
		return
	}
	m, err := tenantAllowBlockListItemFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *tenantAllowBlockListItemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior tenantAllowBlockListItemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, "tenant_allow_block_list_item.ps1", map[string]string{
		"Action": "Get", "ListType": prior.ListType.ValueString(), "Entry": prior.Entry.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Tenant Allow/Block List entry", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := tenantAllowBlockListItemFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *tenantAllowBlockListItemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tenantAllowBlockListItemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, "tenant_allow_block_list_item.ps1", map[string]string{
		"Action": "Delete", "ListType": state.ListType.ValueString(), "Entry": state.Entry.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to delete Tenant Allow/Block List entry", err.Error())
	}
}

// ImportState accepts "<list_type>:<entry>", for example "Sender:spam@contoso.com".
func (r *tenantAllowBlockListItemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	listType, entry, ok := strings.Cut(req.ID, ":")
	if !ok || listType == "" || entry == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("Expected <list_type>:<entry>, got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("list_type"), listType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("entry"), entry)...)
}

// tenantAllowBlockListItemFromAPI builds the state from the bridge response.
// prior keeps the configured case.
func tenantAllowBlockListItemFromAPI(prior tenantAllowBlockListItemModel, output []byte) (tenantAllowBlockListItemModel, error) {
	var a tenantAllowBlockListItemAPIResponse
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.Entry = foldString(prior.Entry, a.Value)
	m.Action = foldString(prior.Action, a.Action)
	m.Notes = types.StringValue(a.Notes)
	m.ID = types.StringValue(m.ListType.ValueString() + ":" + m.Entry.ValueString())
	m.ExpirationDate = types.StringValue(a.ExpirationDate)
	return m, nil
}
