package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                   = &crossTenantPartnerResource{}
	_ resource.ResourceWithImportState    = &crossTenantPartnerResource{}
	_ resource.ResourceWithValidateConfig = &crossTenantPartnerResource{}
)

// exchangeCapabilities are the Microsoft 365 cross-tenant capabilities the
// resource manages. Graph has others (profile card, Places, migration): the
// script never reads or changes them.
var exchangeCapabilities = []string{
	"crossTenantCalendarAvailabilityBasic", "crossTenantCalendarAvailabilityLimitedDetails",
	"crossTenantMailTipsAll", "crossTenantMailTipsLimited",
	"crossTenantCalendarSharingFreeBusySimple", "crossTenantCalendarSharingFreeBusyDetail",
	"crossTenantCalendarSharingFreeBusyReviewer",
}

var guidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`)

// crossTenantPartnerResource manages the Exchange capabilities of the
// Microsoft 365 cross-tenant access policy of one partner, through Microsoft
// Graph (cross_tenant_partner.ps1).
type crossTenantPartnerResource struct {
	client *client.Client
}

type crossTenantPartnerModel struct {
	ID              types.String `tfsdk:"id"`
	PartnerTenantID types.String `tfsdk:"partner_tenant_id"`
	Capabilities    types.Set    `tfsdk:"capabilities"`
}

// capabilityAttrTypes is the type of one element of capabilities. A set of
// nested objects is a types.Set whose elements are types.Object values of
// this type.
var capabilityAttrTypes = map[string]attr.Type{
	"name":            types.StringType,
	"allowed":         types.BoolType,
	"included_groups": types.SetType{ElemType: types.StringType},
	"excluded_groups": types.SetType{ElemType: types.StringType},
	"included_users":  types.SetType{ElemType: types.StringType},
	"excluded_users":  types.SetType{ElemType: types.StringType},
}

// scopeAttrs maps the four scope attributes to the fields of the script JSON.
var scopeAttrs = []struct{ attr, json string }{
	{"included_groups", "includedGroups"}, {"excluded_groups", "excludedGroups"},
	{"included_users", "includedUsers"}, {"excluded_users", "excludedUsers"},
}

// capabilityJSON is one capability as the script reads and writes it.
type capabilityJSON struct {
	Name           string   `json:"name"`
	Allowed        bool     `json:"allowed"`
	IncludedGroups []string `json:"includedGroups"`
	ExcludedGroups []string `json:"excludedGroups"`
	IncludedUsers  []string `json:"includedUsers"`
	ExcludedUsers  []string `json:"excludedUsers"`
}

func (c capabilityJSON) scope(json string) []string {
	switch json {
	case "includedGroups":
		return c.IncludedGroups
	case "excludedGroups":
		return c.ExcludedGroups
	case "includedUsers":
		return c.IncludedUsers
	}
	return c.ExcludedUsers
}

func NewCrossTenantPartnerResource() resource.Resource {
	return &crossTenantPartnerResource{}
}

func (r *crossTenantPartnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cross_tenant_partner"
}

func (r *crossTenantPartnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	quoted := make([]string, len(exchangeCapabilities))
	for i, c := range exchangeCapabilities {
		quoted[i] = "`" + c + "`"
	}
	scope := func(desc string) schema.SetAttribute {
		return schema.SetAttribute{ElementType: types.StringType, Optional: true, MarkdownDescription: desc}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Exchange capabilities (calendar availability, MailTips and calendar sharing) of the " +
			"Microsoft 365 cross-tenant access policy of one partner tenant, through Microsoft Graph. The resource owns all " +
			"the Exchange capabilities of the partner: one set outside Terraform shows in the plan and is removed. Other " +
			"capabilities (profile card, Places) are never read nor changed. They take effect once the Microsoft Entra " +
			"partner configuration and its Microsoft 365 collaboration trust exist.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The partner tenant ID, in lowercase.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"partner_tenant_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Tenant ID of the partner organization. Changing it creates a new resource.",
				PlanModifiers:       []planmodifier.String{caseInsensitive(), stringplanmodifier.RequiresReplace()},
			},
			"capabilities": schema.SetNestedAttribute{
				Required:            true,
				MarkdownDescription: "The Exchange capabilities opened to the partner, one element per capability.",
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The capability. Valid values: " + strings.Join(quoted, ", ") + ".",
						Validators:          []validator.String{stringvalidator.OneOf(exchangeCapabilities...)},
					},
					"allowed": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether the partner's users can use the capability.",
					},
					"included_groups": scope("Object IDs of the groups whose members the capability applies to. Without " +
						"any included group or user, it applies to all users."),
					"excluded_groups": scope("Object IDs of the groups whose members the capability never applies to."),
					"included_users":  scope("Object IDs of the users the capability applies to."),
					"excluded_users":  scope("Object IDs of the users the capability never applies to."),
				}},
			},
		},
	}
}

func (r *crossTenantPartnerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg crossTenantPartnerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if v := cfg.PartnerTenantID; !v.IsUnknown() && !v.IsNull() && !guidRe.MatchString(v.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("partner_tenant_id"), "Invalid partner_tenant_id",
			fmt.Sprintf("%q is not a tenant ID (GUID).", v.ValueString()))
	}
	if cfg.Capabilities.IsUnknown() || cfg.Capabilities.IsNull() {
		return
	}
	seen := map[string]bool{}
	for _, e := range cfg.Capabilities.Elements() {
		obj, ok := e.(types.Object)
		if !ok || obj.IsUnknown() {
			continue
		}
		attrs := obj.Attributes()
		if name, ok := attrs["name"].(types.String); ok && !name.IsUnknown() && !name.IsNull() {
			if seen[name.ValueString()] {
				resp.Diagnostics.AddAttributeError(path.Root("capabilities"), "Duplicate capability",
					fmt.Sprintf("%s is listed twice: list each capability once.", name.ValueString()))
			}
			seen[name.ValueString()] = true
		}
		for _, s := range scopeAttrs {
			set, ok := attrs[s.attr].(types.Set)
			if !ok || set.IsUnknown() || set.IsNull() {
				continue
			}
			for _, id := range set.Elements() {
				if v, ok := id.(types.String); ok && !v.IsUnknown() && !guidRe.MatchString(v.ValueString()) {
					resp.Diagnostics.AddAttributeError(path.Root("capabilities"), "Invalid object ID",
						fmt.Sprintf("%s: %q is not a Microsoft Entra object ID (GUID).", s.attr, v.ValueString()))
				}
			}
		}
	}
}

func (r *crossTenantPartnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// params builds the script parameters: the configured capabilities and the
// names in the state (to tell a missing Entra role from an empty partner).
func (r *crossTenantPartnerResource) params(ctx context.Context, action string, partner string, desired, known types.Set) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]string{"Action": action, "PartnerTenantId": partner, "Known": encodeJSON(capabilityNames(known))}
	if !desired.IsNull() && !desired.IsUnknown() {
		var list []capabilityJSON
		for _, e := range desired.Elements() {
			attrs := e.(types.Object).Attributes()
			c := capabilityJSON{
				Name:    attrs["name"].(types.String).ValueString(),
				Allowed: attrs["allowed"].(types.Bool).ValueBool(),
			}
			for _, s := range scopeAttrs {
				ids := []string{}
				if set, ok := attrs[s.attr].(types.Set); ok && !set.IsNull() && !set.IsUnknown() {
					diags.Append(set.ElementsAs(ctx, &ids, false)...)
				}
				switch s.json {
				case "includedGroups":
					c.IncludedGroups = ids
				case "excludedGroups":
					c.ExcludedGroups = ids
				case "includedUsers":
					c.IncludedUsers = ids
				default:
					c.ExcludedUsers = ids
				}
			}
			list = append(list, c)
		}
		p["Capabilities"] = encodeJSON(list)
	}
	return p, diags
}

// capabilityNames returns the names of a capabilities set.
func capabilityNames(set types.Set) []string {
	names := []string{}
	if set.IsNull() || set.IsUnknown() {
		return names
	}
	for _, e := range set.Elements() {
		if obj, ok := e.(types.Object); ok {
			if n, ok := obj.Attributes()["name"].(types.String); ok {
				names = append(names, n.ValueString())
			}
		}
	}
	return names
}

func (r *crossTenantPartnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan crossTenantPartnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, diags := r.params(ctx, "Create", plan.PartnerTenantID.ValueString(), plan.Capabilities, types.SetNull(types.ObjectType{AttrTypes: capabilityAttrTypes}))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, p, plan, &resp.State, &resp.Diagnostics)
}

func (r *crossTenantPartnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state crossTenantPartnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, diags := r.params(ctx, "Update", state.ID.ValueString(), plan.Capabilities, state.Capabilities)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, p, plan, &resp.State, &resp.Diagnostics)
}

func (r *crossTenantPartnerResource) write(ctx context.Context, p map[string]string, plan crossTenantPartnerModel, state stateSetter, diags diagAppender) {
	output, err := r.client.RunScript(ctx, "cross_tenant_partner.ps1", p)
	if err != nil {
		diags.AddError("Failed to write the cross-tenant access policy of the partner", err.Error())
		return
	}
	m, err := crossTenantPartnerFromAPI(ctx, plan, output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *crossTenantPartnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior crossTenantPartnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, diags := r.params(ctx, "Get", prior.ID.ValueString(), types.SetNull(types.ObjectType{AttrTypes: capabilityAttrTypes}), prior.Capabilities)
	resp.Diagnostics.Append(diags...)
	output, err := r.client.RunScript(ctx, "cross_tenant_partner.ps1", p)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the cross-tenant access policy of the partner", err.Error())
		return
	}
	if client.IsNotFound(output) {
		resp.State.RemoveResource(ctx)
		return
	}
	m, err := crossTenantPartnerFromAPI(ctx, prior, output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *crossTenantPartnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state crossTenantPartnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, diags := r.params(ctx, "Delete", state.ID.ValueString(), types.SetNull(types.ObjectType{AttrTypes: capabilityAttrTypes}), state.Capabilities)
	resp.Diagnostics.Append(diags...)
	if _, err := r.client.RunScript(ctx, "cross_tenant_partner.ps1", p); err != nil {
		resp.Diagnostics.AddError("Failed to delete the cross-tenant access policy of the partner", err.Error())
	}
}

// ImportState accepts the partner tenant ID.
func (r *crossTenantPartnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.ToLower(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("partner_tenant_id"), id)...)
}

// crossTenantPartnerFromAPI builds the state from the script output. prior is
// the plan or previous state: a scope set it holds as null stays null when
// Graph has no value for it, so a scope left out of the configuration never
// shows as a change.
func crossTenantPartnerFromAPI(ctx context.Context, prior crossTenantPartnerModel, output []byte) (crossTenantPartnerModel, error) {
	var a struct {
		PartnerTenantID string           `json:"PartnerTenantId"`
		Capabilities    []capabilityJSON `json:"Capabilities"`
	}
	if err := json.Unmarshal(output, &a); err != nil {
		return prior, err
	}
	priorByName := map[string]map[string]attr.Value{}
	if !prior.Capabilities.IsNull() && !prior.Capabilities.IsUnknown() {
		for _, e := range prior.Capabilities.Elements() {
			if obj, ok := e.(types.Object); ok {
				if n, ok := obj.Attributes()["name"].(types.String); ok {
					priorByName[n.ValueString()] = obj.Attributes()
				}
			}
		}
	}
	elems := make([]attr.Value, 0, len(a.Capabilities))
	for _, c := range a.Capabilities {
		values := map[string]attr.Value{"name": types.StringValue(c.Name), "allowed": types.BoolValue(c.Allowed)}
		for _, s := range scopeAttrs {
			ids := c.scope(s.json)
			if p, ok := priorByName[c.Name][s.attr].(types.Set); len(ids) == 0 && (!ok || p.IsNull()) {
				values[s.attr] = types.SetNull(types.StringType)
				continue
			}
			ids = foldIDs(priorByName[c.Name][s.attr], ids)
			v, diags := types.SetValueFrom(ctx, types.StringType, ids)
			if diags.HasError() {
				return prior, fmt.Errorf("%s: %v", s.attr, diags)
			}
			values[s.attr] = v
		}
		obj, diags := types.ObjectValue(capabilityAttrTypes, values)
		if diags.HasError() {
			return prior, fmt.Errorf("capability %s: %v", c.Name, diags)
		}
		elems = append(elems, obj)
	}
	set, diags := types.SetValue(types.ObjectType{AttrTypes: capabilityAttrTypes}, elems)
	if diags.HasError() {
		return prior, fmt.Errorf("capabilities: %v", diags)
	}
	id := strings.ToLower(a.PartnerTenantID)
	m := crossTenantPartnerModel{ID: types.StringValue(id), PartnerTenantID: foldString(prior.PartnerTenantID, id), Capabilities: set}
	return m, nil
}

// foldIDs returns ids with each one written as in prior when they differ only
// by case: Graph returns object IDs in lowercase, and an ID configured in
// uppercase must not show as a change.
func foldIDs(prior attr.Value, ids []string) []string {
	set, ok := prior.(types.Set)
	if !ok || set.IsNull() || set.IsUnknown() {
		return ids
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id
		for _, e := range set.Elements() {
			if v, ok := e.(types.String); ok && strings.EqualFold(v.ValueString(), id) {
				out[i] = v.ValueString()
				break
			}
		}
	}
	return out
}
