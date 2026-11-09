package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
)

const (
	quarantineGlobalSettingsID     = "default"
	quarantineGlobalSettingsScript = "quarantine_global_settings.ps1"
	quarantineMaxLanguages         = 3
	quarantineMaxDisclaimerLength  = 200
)

// quarantineLanguages are the values Set-QuarantinePolicy documents for
// MultiLanguageSetting.
var quarantineLanguages = []string{
	"Default", "Amharic", "Arabic", "Basque", "BengaliIndia", "Bulgarian", "Catalan", "ChineseSimplified",
	"ChineseTraditional", "Croatian", "Cyrillic", "Czech", "Danish", "Dutch", "English", "Estonian", "Filipino",
	"Finnish", "French", "Galician", "German", "Greek", "Gujarati", "Hebrew", "Hindi", "Hungarian", "Icelandic",
	"Indonesian", "Italian", "Japanese", "Kannada", "Kazakh", "Korean", "Latvian", "Lithuanian", "Malay", "Malayalam",
	"Marathi", "Norwegian", "NorwegianNynorsk", "Odia", "Persian", "Polish", "Portuguese", "PortuguesePortugal",
	"Romanian", "Russian", "Serbian", "SerbianCyrillic", "Slovak", "Slovenian", "Spanish", "Swahili", "Swedish",
	"Tamil", "Telugu", "Thai", "Turkish", "Ukrainian", "Urdu", "Vietnamese",
}

var quarantineLanguageType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"language": types.StringType, "sender_name": types.StringType, "subject": types.StringType, "disclaimer": types.StringType,
}}

var (
	_ resource.Resource                   = &quarantineGlobalSettingsResource{}
	_ resource.ResourceWithImportState    = &quarantineGlobalSettingsResource{}
	_ resource.ResourceWithValidateConfig = &quarantineGlobalSettingsResource{}
)

// quarantineGlobalSettingsResource manages the global settings of quarantine
// notifications: one object per tenant, which destroy leaves in place.
type quarantineGlobalSettingsResource struct {
	client *client.Client
}

type quarantineGlobalSettingsModel struct {
	ID                          types.String `tfsdk:"id"`
	NotificationFrequency       types.String `tfsdk:"notification_frequency"`
	SenderAddress               types.String `tfsdk:"sender_address"`
	OrganizationBrandingEnabled types.Bool   `tfsdk:"organization_branding_enabled"`
	Languages                   types.Set    `tfsdk:"languages"`
}

type quarantineLanguageAPI struct {
	Language   string `json:"Language"`
	SenderName string `json:"SenderName"`
	Subject    string `json:"Subject"`
	Disclaimer string `json:"Disclaimer"`
}

type quarantineGlobalSettingsAPI struct {
	Frequency     string                  `json:"EndUserSpamNotificationFrequency"`
	SenderAddress string                  `json:"EndUserSpamNotificationCustomFromAddress"`
	Branding      bool                    `json:"OrganizationBrandingEnabled"`
	Languages     []quarantineLanguageAPI `json:"Languages"`
}

func NewQuarantineGlobalSettingsResource() resource.Resource {
	return &quarantineGlobalSettingsResource{}
}

func (r *quarantineGlobalSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quarantine_global_settings"
}

func (r *quarantineGlobalSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the global settings of quarantine notifications (`Set-QuarantinePolicy`): frequency, sender, " +
			"languages and organization branding. This is a singleton: there is exactly one per tenant, and destroying the resource only removes it from Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Always `default`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"notification_frequency": optionalEnum("How often users receive quarantine notifications. Valid values: `04:00:00` (4 hours), `1.00:00:00` (1 day), `7.00:00:00` (7 days).",
				"04:00:00", "1.00:00:00", "7.00:00:00"),
			"sender_address":                optionalString("Address of an existing internal sender used for quarantine notifications. Empty restores the Microsoft sender."),
			"organization_branding_enabled": optionalBool("Replace the Microsoft logo with the logo of the organization theme in the Microsoft 365 admin center."),
			"languages": schema.SetNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Customized notifications, at most three, one element per language. An empty set removes all of them.",
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"language": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Language of the notification, for example `English`, `French` or `Default`, as in `Set-QuarantinePolicy -MultiLanguageSetting`.",
						Validators:          []validator.String{stringvalidator.OneOf(quarantineLanguages...)},
					},
					"sender_name": schema.StringAttribute{Optional: true, MarkdownDescription: "Display name of the sender."},
					"subject":     schema.StringAttribute{Optional: true, MarkdownDescription: "Subject of the notification."},
					"disclaimer":  schema.StringAttribute{Optional: true, MarkdownDescription: "Text at the bottom of the notification, at most 200 characters."},
				}},
			},
		},
	}
}

func (r *quarantineGlobalSettingsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg quarantineGlobalSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Languages.IsNull() || cfg.Languages.IsUnknown() {
		return
	}
	languages := path.Root("languages")
	if n := len(cfg.Languages.Elements()); n > quarantineMaxLanguages {
		resp.Diagnostics.AddAttributeError(languages, "Too many languages", fmt.Sprintf("Exchange Online accepts at most %d languages, got %d.", quarantineMaxLanguages, n))
	}
	if d := duplicateLanguage(cfg.Languages); d != "" {
		resp.Diagnostics.AddAttributeError(languages, "Duplicate language", fmt.Sprintf("The language %q is set more than once.", d))
	}
	if name := emptyLanguageText(cfg.Languages); name != "" {
		resp.Diagnostics.AddAttributeError(languages, "Empty text", fmt.Sprintf("%s is empty: leave it out instead.", name))
	}
	allowed := map[string]bool{}
	for _, l := range quarantineLanguages {
		allowed[l] = true
	}
	for _, a := range knownLanguages(cfg.Languages) {
		if language := a["language"].(types.String); !language.IsUnknown() && !language.IsNull() && !allowed[language.ValueString()] {
			resp.Diagnostics.AddAttributeError(languages, "Unknown language", fmt.Sprintf("%q is not a value of Set-QuarantinePolicy -MultiLanguageSetting, with this exact case, such as English or French.", language.ValueString()))
		}
		if d := a["disclaimer"].(types.String); len([]rune(d.ValueString())) > quarantineMaxDisclaimerLength {
			resp.Diagnostics.AddAttributeError(languages, "Disclaimer too long", fmt.Sprintf("A disclaimer has at most %d characters.", quarantineMaxDisclaimerLength))
		}
	}
}

func (r *quarantineGlobalSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req, resp); c != nil {
		r.client = c
	}
}

// params sends the attributes set in the configuration: the others keep their
// Exchange value.
func quarantineGlobalSettingsParams(cfg quarantineGlobalSettingsModel) map[string]string {
	values := map[string]any{}
	if known(cfg.NotificationFrequency) {
		values["EndUserSpamNotificationFrequency"] = cfg.NotificationFrequency.ValueString()
	}
	if known(cfg.SenderAddress) {
		values["EndUserSpamNotificationCustomFromAddress"] = cfg.SenderAddress.ValueString()
	}
	if known(cfg.OrganizationBrandingEnabled) {
		values["OrganizationBrandingEnabled"] = cfg.OrganizationBrandingEnabled.ValueBool()
	}
	p := map[string]string{"Action": "Update", "Values": encodeJSON(values)}
	if known(cfg.Languages) {
		languages := make([]quarantineLanguageAPI, 0, len(cfg.Languages.Elements()))
		for _, e := range cfg.Languages.Elements() {
			a := e.(types.Object).Attributes()
			languages = append(languages, quarantineLanguageAPI{
				Language:   a["language"].(types.String).ValueString(),
				SenderName: a["sender_name"].(types.String).ValueString(),
				Subject:    a["subject"].(types.String).ValueString(),
				Disclaimer: a["disclaimer"].(types.String).ValueString(),
			})
		}
		p["Languages"] = encodeJSON(languages)
	}
	return p
}

func (r *quarantineGlobalSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg quarantineGlobalSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if !resp.Diagnostics.HasError() {
		r.write(ctx, cfg, &resp.State, &resp.Diagnostics)
	}
}

func (r *quarantineGlobalSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg quarantineGlobalSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if !resp.Diagnostics.HasError() {
		r.write(ctx, cfg, &resp.State, &resp.Diagnostics)
	}
}

func (r *quarantineGlobalSettingsResource) write(ctx context.Context, cfg quarantineGlobalSettingsModel, state stateSetter, diags diagAppender) {
	output, err := r.client.RunScript(ctx, quarantineGlobalSettingsScript, quarantineGlobalSettingsParams(cfg))
	if err != nil {
		diags.AddError("Failed to write the global quarantine settings", err.Error())
		return
	}
	m, err := quarantineGlobalSettingsFromAPI(output)
	if err != nil {
		diags.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *quarantineGlobalSettingsResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	output, err := r.client.RunScript(ctx, quarantineGlobalSettingsScript, map[string]string{"Action": "Get"})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the global quarantine settings", err.Error())
		return
	}
	m, err := quarantineGlobalSettingsFromAPI(output)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse PowerShell response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// Delete only removes the resource from the state: the settings stay.
func (r *quarantineGlobalSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	warnStateOnlyDestroy(&resp.Diagnostics, "quarantine_global_settings", "The global quarantine settings always exist.")
}

func (r *quarantineGlobalSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), quarantineGlobalSettingsID)...)
}

// quarantineGlobalSettingsFromAPI builds the state from the bridge response.
func quarantineGlobalSettingsFromAPI(output []byte) (quarantineGlobalSettingsModel, error) {
	var a quarantineGlobalSettingsAPI
	if err := json.Unmarshal(output, &a); err != nil {
		return quarantineGlobalSettingsModel{}, err
	}
	elems := make([]attr.Value, 0, len(a.Languages))
	for _, l := range a.Languages {
		elems = append(elems, types.ObjectValueMust(quarantineLanguageType.AttrTypes, map[string]attr.Value{
			"language":    types.StringValue(l.Language),
			"sender_name": optionalLanguageText(l.SenderName),
			"subject":     optionalLanguageText(l.Subject),
			"disclaimer":  optionalLanguageText(l.Disclaimer),
		}))
	}
	return quarantineGlobalSettingsModel{
		ID:                          types.StringValue(quarantineGlobalSettingsID),
		NotificationFrequency:       types.StringValue(a.Frequency),
		SenderAddress:               types.StringValue(a.SenderAddress),
		OrganizationBrandingEnabled: types.BoolValue(a.Branding),
		Languages:                   types.SetValueMust(quarantineLanguageType, elems),
	}, nil
}
