package provider

import (
	"context"
	"fmt"
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
	CertificatePath       types.String `tfsdk:"certificate_path"`
	CertificatePassword   types.String `tfsdk:"certificate_password"`
	CertificateThumbprint types.String `tfsdk:"certificate_thumbprint"`
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
			"certificate_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to the `.pfx` certificate file for app-only (CBA) authentication. Use this or `certificate_thumbprint`. Can also be set via `EXCHANGE_CERTIFICATE_PATH` environment variable.",
			},
			"certificate_password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for the `.pfx` certificate file. Can also be set via `EXCHANGE_CERTIFICATE_PASSWORD` environment variable.",
			},
			"certificate_thumbprint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Thumbprint of a certificate already loaded in the local certificate store (Windows). Use this or `certificate_path`. Can also be set via `EXCHANGE_CERTIFICATE_THUMBPRINT` environment variable.",
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

	cfg := client.Config{
		TenantID:              envOrConfig("EXCHANGE_TENANT_ID", config.TenantID),
		ClientID:              envOrConfig("EXCHANGE_CLIENT_ID", config.ClientID),
		CertificatePath:       envOrConfig("EXCHANGE_CERTIFICATE_PATH", config.CertificatePath),
		CertificatePassword:   envOrConfig("EXCHANGE_CERTIFICATE_PASSWORD", config.CertificatePassword),
		CertificateThumbprint: envOrConfig("EXCHANGE_CERTIFICATE_THUMBPRINT", config.CertificateThumbprint),
		Environment:           envOrConfig("EXCHANGE_ENVIRONMENT", config.Environment),
	}
	if cfg.Environment == "" {
		cfg.Environment = "global"
	}

	if cfg.TenantID == "" {
		resp.Diagnostics.AddError("Missing tenant_id", "tenant_id is required. Set it in the provider block or via EXCHANGE_TENANT_ID.")
	} else if isGUID(cfg.TenantID) {
		resp.Diagnostics.AddError("Invalid tenant_id",
			fmt.Sprintf("tenant_id %q is a tenant GUID. Exchange Online needs the tenant's primary .onmicrosoft.com domain, for example contoso.onmicrosoft.com: Connect-ExchangeOnline -Organization does not accept a GUID.", cfg.TenantID))
	}
	if cfg.ClientID == "" {
		resp.Diagnostics.AddError("Missing client_id", "client_id is required. Set it in the provider block or via EXCHANGE_CLIENT_ID.")
	}
	if cfg.CertificatePath == "" && cfg.CertificateThumbprint == "" {
		resp.Diagnostics.AddError("Missing certificate", "Either certificate_path or certificate_thumbprint is required.")
	} else if problem := certificateFileProblem(cfg.CertificatePath); problem != "" {
		// Checked here so that a wrong path fails at plan time with a clear message,
		// instead of an OpenSSL error from PowerShell at apply time.
		resp.Diagnostics.AddError("Invalid certificate_path", problem)
	}

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

// certificateFileProblem describes why the .pfx file at path cannot be used, or
// returns "" when it is a readable file. An empty path is not checked: the
// certificate then comes from certificate_thumbprint.
func certificateFileProblem(path string) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return fmt.Sprintf("certificate_path %q does not exist. Set the path of the app registration's .pfx file, in the provider block or via EXCHANGE_CERTIFICATE_PATH. A relative path is resolved from the folder where Terraform runs.", path)
	case err != nil:
		return fmt.Sprintf("certificate_path %q cannot be read: %v", path, err)
	case info.IsDir():
		return fmt.Sprintf("certificate_path %q is a folder: set the path of the .pfx file itself.", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("certificate_path %q cannot be read: %v", path, err)
	}
	_ = f.Close()
	return ""
}
