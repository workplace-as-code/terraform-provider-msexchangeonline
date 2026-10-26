package provider

import (
	"context"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/client"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/datasources"
	"github.com/workplace-as-code/terraform-provider-msexchangeonline/internal/resources"
)

var _ provider.ProviderWithListResources = &exchangeProvider{}

type exchangeProvider struct {
	version string
}

// exchangeProviderModel represents the HCL provider {} block. Every field is
// optional: it can also come from an EXCHANGE_* environment variable.
type exchangeProviderModel struct {
	TenantID              types.String `tfsdk:"tenant_id"`
	ClientID              types.String `tfsdk:"client_id"`
	AuthMethod            types.String `tfsdk:"auth_method"`
	CertificatePath       types.String `tfsdk:"certificate_path"`
	CertificatePassword   types.String `tfsdk:"certificate_password"`
	CertificateThumbprint types.String `tfsdk:"certificate_thumbprint"`
	CertificateBase64     types.String `tfsdk:"certificate_base64"`
	OIDCToken             types.String `tfsdk:"oidc_token"`
	OIDCTokenFilePath     types.String `tfsdk:"oidc_token_file_path"`
	Environment           types.String `tfsdk:"environment"`
}

// New returns a factory function that creates the provider.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &exchangeProvider{version: version}
	}
}

func (p *exchangeProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "msexchangeonline"
	resp.Version = p.version
}

func (p *exchangeProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages Exchange Online configuration as code.",
		Attributes: map[string]schema.Attribute{
			"tenant_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The tenant's primary `.onmicrosoft.com` domain (for example `contoso.onmicrosoft.com`), or its short name (`contoso`). A tenant GUID is not accepted: `Connect-ExchangeOnline` needs the domain. Required. Can also be set via `EXCHANGE_TENANT_ID` environment variable.",
			},
			"client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Entra ID app registration client ID. Required. Can also be set via `EXCHANGE_CLIENT_ID` environment variable.",
			},
			"auth_method": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf(authMethods...)},
				MarkdownDescription: "How the provider authenticates. Valid values: `client_certificate` (default: a certificate, see `certificate_path`, `certificate_base64` and `certificate_thumbprint`), `oidc` (an OIDC token from any platform, see `oidc_token` and `oidc_token_file_path`), `oidc_github` (the OIDC token of the GitHub Actions job, which needs `permissions: id-token: write`). Can also be set via `EXCHANGE_AUTH_METHOD` environment variable.",
			},
			"certificate_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to the `.pfx` certificate file for app-only (CBA) authentication. Use only one of `certificate_path`, `certificate_base64` and `certificate_thumbprint`. Can also be set via `EXCHANGE_CERTIFICATE_PATH` environment variable.",
			},
			"certificate_base64": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Content of the `.pfx` certificate file, private key included, encoded in base64 (for example `openssl base64 -A -in app.pfx`). Decoded in memory: no file is written. Use it to pass the certificate from a vault or a masked CI variable. Use only one of `certificate_path`, `certificate_base64` and `certificate_thumbprint`. Can also be set via `EXCHANGE_CERTIFICATE_BASE64` environment variable.",
			},
			"certificate_password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for the `.pfx` certificate, given as a file or in base64. Can also be set via `EXCHANGE_CERTIFICATE_PASSWORD` environment variable.",
			},
			"certificate_thumbprint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Thumbprint of a certificate already loaded in the local certificate store (Windows). Use only one of `certificate_path`, `certificate_base64` and `certificate_thumbprint`. Can also be set via `EXCHANGE_CERTIFICATE_THUMBPRINT` environment variable.",
			},
			"oidc_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "With `auth_method = \"oidc\"`: the OIDC token that Entra ID exchanges for an access token (for example a GitLab `id_tokens` value). It cannot be renewed during the run: prefer `oidc_token_file_path` when the platform writes the token to a file. Use only one of `oidc_token` and `oidc_token_file_path`. Can also be set via `EXCHANGE_OIDC_TOKEN` environment variable.",
			},
			"oidc_token_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "With `auth_method = \"oidc\"`: path to a file that contains the OIDC token, read again each time a new access token is needed (Kubernetes renews projected tokens). Use only one of `oidc_token` and `oidc_token_file_path`. Can also be set via `EXCHANGE_OIDC_TOKEN_FILE_PATH` environment variable.",
			},
			"environment": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("global", "gcc", "gcchigh", "dod", "china")},
				MarkdownDescription: "M365 cloud environment. Valid values: `global` (default), `gcc`, `gcchigh`, `dod`, `china`. Can also be set via `EXCHANGE_ENVIRONMENT` environment variable.",
			},
		},
	}
}

func (p *exchangeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config exchangeProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := resolveConfig(config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient, err := client.New(ctx, cfg)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Exchange client", err.Error())
		return
	}

	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
	resp.ListResourceData = apiClient
}

// Resources returns the list of all resources managed by the provider.
func (p *exchangeProvider) Resources(_ context.Context) []func() resource.Resource {
	return resources.Resources()
}

// ListResources lists the existing objects for terraform query (Terraform 1.14+).
func (p *exchangeProvider) ListResources(_ context.Context) []func() list.ListResource {
	return resources.ListResources()
}

// DataSources returns the list of all data sources.
func (p *exchangeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewAcceptedDomainDataSource,
		datasources.NewAcceptedDomainsDataSource,
		datasources.NewMailboxDataSource,
		datasources.NewOrganizationConfigDataSource,
	}
}

var guidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isGUID reports whether s looks like an Entra tenant ID (a GUID).
func isGUID(s string) bool {
	return guidRe.MatchString(s)
}

// envOrConfig returns the HCL config value if set, otherwise the environment
// variable. An explicit provider block always wins, as in the HashiCorp
// providers, so a stray variable in a CI runner cannot silently redirect
// Terraform to another tenant.
func envOrConfig(envKey string, configVal types.String) string {
	if !configVal.IsNull() && !configVal.IsUnknown() && configVal.ValueString() != "" {
		return configVal.ValueString()
	}
	return os.Getenv(envKey)
}
