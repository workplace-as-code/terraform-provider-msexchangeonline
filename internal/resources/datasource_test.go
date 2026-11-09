package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDataSourcesMirrorTheirResource checks every data source built from a
// resource: its key is the only argument, and its value type is the
// resource's, since Read copies the resource state as is.
func TestDataSourcesMirrorTheirResource(t *testing.T) {
	ctx := context.Background()
	if len(DataSources()) != len(allDataSourceResources()) {
		t.Fatalf("%d data sources for %d resources", len(DataSources()), len(allDataSourceResources()))
	}
	for _, newDataSource := range DataSources() {
		d := newDataSource().(*resourceDataSource)
		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "msexchangeonline"}, &meta)
		t.Run(meta.TypeName, func(t *testing.T) {
			var resp datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			for name, a := range resp.Schema.Attributes {
				if a.IsRequired() != (name == d.key) || (name != d.key && !a.IsComputed()) {
					t.Errorf("%s: required=%v computed=%v", name, a.IsRequired(), a.IsComputed())
				}
			}
			if _, ok := resp.Schema.Attributes[d.key].(dsschema.StringAttribute); d.key != "" && !ok {
				t.Errorf("no %s attribute", d.key)
			}
			want := d.resourceSchema(ctx).Type().TerraformType(ctx)
			if got := resp.Schema.Type().TerraformType(ctx); !got.Equal(want) {
				t.Errorf("type differs from the resource:\n%s\n%s", got, want)
			}
		})
	}
}

// TestReadOnlySpecsAreDataSourcesOnly: the hybrid objects that the Hybrid
// Configuration Wizard manages are data sources, never resources.
func TestReadOnlySpecsAreDataSourcesOnly(t *testing.T) {
	ctx := context.Background()
	dataSources := map[string]bool{}
	for _, newDataSource := range DataSources() {
		var meta datasource.MetadataResponse
		newDataSource().Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "msexchangeonline"}, &meta)
		dataSources[meta.TypeName] = true
	}
	resources := map[string]bool{}
	for _, newResource := range Resources() {
		var meta resource.MetadataResponse
		newResource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "msexchangeonline"}, &meta)
		resources[meta.TypeName] = true
	}
	for _, name := range []string{"msexchangeonline_onpremises_organization", "msexchangeonline_intra_organization_connector"} {
		if !dataSources[name] {
			t.Errorf("%s must be a data source", name)
		}
		if resources[name] {
			t.Errorf("%s must not be a resource", name)
		}
	}
}

// TestDataSourceWithoutKey: a tenant-wide setting has one object, so its data
// source takes no argument.
func TestDataSourceWithoutKey(t *testing.T) {
	d := &resourceDataSource{resource: NewQuarantineGlobalSettingsResource(), key: ""}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	for name, a := range resp.Schema.Attributes {
		if a.IsRequired() || !a.IsComputed() {
			t.Errorf("%s: required=%v computed=%v", name, a.IsRequired(), a.IsComputed())
		}
	}
	if got := dataSourceImportID(d.key, types.StringNull()); got != "default" {
		t.Errorf("import ID = %q, want default", got)
	}
	if got := dataSourceImportID("name", types.StringValue("TF-CI")); got != "TF-CI" {
		t.Errorf("import ID = %q, want TF-CI", got)
	}
}
