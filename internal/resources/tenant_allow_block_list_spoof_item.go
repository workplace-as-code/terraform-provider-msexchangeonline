package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

const spoofItemScript = "tenant_allow_block_list_spoof_item.ps1"

var (
	_ resource.Resource                = &spoofItemResource{}
	_ resource.ResourceWithImportState = &spoofItemResource{}
)

// spoofItemResource manages one spoofed sender pair of the Tenant Allow/Block
// List. An entry is identified by its pair, not by the GUID Exchange gives it.
type spoofItemResource struct {
	client *client.Client
}

type spoofItemModel struct {
	ID                    types.String `tfsdk:"id"`
	SpoofedUser           types.String `tfsdk:"spoofed_user"`
	SendingInfrastructure types.String `tfsdk:"sending_infrastructure"`
	SpoofType             types.String `tfsdk:"spoof_type"`
	Action                types.String `tfsdk:"action"`
}

type spoofItemAPI struct {
	SpoofType             string `json:"SpoofType"`
	SpoofedUser           string `json:"SpoofedUser"`
	SendingInfrastructure string `json:"SendingInfrastructure"`
	Action                string `json:"Action"`
}

func NewSpoofItemResource() resource.Resource {
	return &spoofItemResource{}
}

func (r *spoofItemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_allow_block_list_spoof_item"
}

func (r *spoofItemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a spoofed sender pair of the Tenant Allow/Block List (`New-TenantAllowBlockListSpoofItems`): " +
			"a spoofed user and the infrastructure sending as that user, allowed or blocked.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`<spoof_type>:<spoofed_user>:<sending_infrastructure>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"spoofed_user": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Email address for a sender inside the organization, domain for a sender outside it. " +
					"Changing it creates a new entry.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"sending_infrastructure": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Source of the messages: a domain found in the reverse DNS of the sending server, a verified DKIM " +
					"domain, or an IP address written `<address>/24`. Changing it creates a new entry.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"spoof_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`Internal` or `External`. Changing it creates a new entry.",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive("Internal", "External")},
				PlanModifiers:       replace,
			},
			"action": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`Allow` or `Block`.",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive("Allow", "Block")},
				PlanModifiers:       []planmodifier.String{caseInsensitive()},
			},
		},
	}
}

func (r *spoofItemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// canonicalSpoofType is the value the script's parameter accepts.
func canonicalSpoofType(v string) string {
	if strings.EqualFold(v, "Internal") {
		return "Internal"
	}
	return "External"
}

func spoofItemParams(action string, m spoofItemModel) map[string]string {
	return map[string]string{
		"Action":                action,
		"SpoofType":             canonicalSpoofType(m.SpoofType.ValueString()),
		"SpoofedUser":           m.SpoofedUser.ValueString(),
		"SendingInfrastructure": m.SendingInfrastructure.ValueString(),
	}
}

func (r *spoofItemResource) write(ctx context.Context, action string, plan spoofItemModel, state stateSetter, diags diagAppender) {
	p := spoofItemParams(action, plan)
	p["ListAction"] = canonicalAction(plan.Action.ValueString())
	output, err := r.client.RunScript(ctx, spoofItemScript, p)
	if err != nil {
		diags.AddError("Failed to write the spoofed sender pair", err.Error())
		return
	}
	m, err := spoofItemFromAPI(plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

// canonicalAction is the value the script's parameter accepts.
func canonicalAction(v string) string {
	if strings.EqualFold(v, "Block") {
		return "Block"
	}
	return "Allow"
}

func (r *spoofItemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan spoofItemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !resp.Diagnostics.HasError() {
		r.write(ctx, "Create", plan, &resp.State, &resp.Diagnostics)
	}
}

func (r *spoofItemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan spoofItemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !resp.Diagnostics.HasError() {
		r.write(ctx, "Update", plan, &resp.State, &resp.Diagnostics)
	}
}

func (r *spoofItemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior spoofItemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	output, err := r.client.RunScript(ctx, spoofItemScript, spoofItemParams("Get", prior))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the spoofed sender pair", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := spoofItemFromAPI(prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *spoofItemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state spoofItemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.RunScript(ctx, spoofItemScript, spoofItemParams("Delete", state)); err != nil {
		resp.Diagnostics.AddError("Failed to delete the spoofed sender pair", err.Error())
	}
}

// parseSpoofItemID splits on the first two colons only: the last part is
// free-form.
func parseSpoofItemID(id string) (spoofType, spoofedUser, infrastructure string, ok bool) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	if !strings.EqualFold(parts[0], "Internal") && !strings.EqualFold(parts[0], "External") {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// ImportState accepts "<spoof_type>:<spoofed_user>:<sending_infrastructure>".
func (r *spoofItemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	spoofType, user, infrastructure, ok := parseSpoofItemID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("Expected <spoof_type>:<spoofed_user>:<sending_infrastructure>, got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("spoof_type"), spoofType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("spoofed_user"), user)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sending_infrastructure"), infrastructure)...)
}

// spoofItemFromAPI builds the state from the bridge response; prior keeps the
// configured case.
func spoofItemFromAPI(prior spoofItemModel, output []byte) (spoofItemModel, error) {
	var a spoofItemAPI
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	m := prior
	m.SpoofType = foldString(prior.SpoofType, a.SpoofType)
	m.SpoofedUser = foldString(prior.SpoofedUser, a.SpoofedUser)
	m.SendingInfrastructure = foldString(prior.SendingInfrastructure, a.SendingInfrastructure)
	m.Action = foldString(prior.Action, a.Action)
	m.ID = types.StringValue(m.SpoofType.ValueString() + ":" + m.SpoofedUser.ValueString() + ":" + m.SendingInfrastructure.ValueString())
	return m, nil
}
