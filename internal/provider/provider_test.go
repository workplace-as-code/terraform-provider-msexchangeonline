package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEnvOrConfigPrefersExplicitConfig(t *testing.T) {
	t.Setenv("EXCHANGE_TENANT_ID", "from-env")

	if got := envOrConfig("EXCHANGE_TENANT_ID", types.StringValue("from-hcl")); got != "from-hcl" {
		t.Errorf("config should win, got %q", got)
	}
	if got := envOrConfig("EXCHANGE_TENANT_ID", types.StringNull()); got != "from-env" {
		t.Errorf("env should be the fallback, got %q", got)
	}
	if got := envOrConfig("EXCHANGE_TENANT_ID", types.StringUnknown()); got != "from-env" {
		t.Errorf("unknown config should fall back to env, got %q", got)
	}
}

func TestIsGUID(t *testing.T) {
	cases := map[string]bool{
		"4f2b8c1e-9a3d-4e5f-8b7a-1c2d3e4f5a6b": true,
		"4F2B8C1E-9A3D-4E5F-8B7A-1C2D3E4F5A6B": true,
		"contoso.onmicrosoft.com":              false,
		"contoso":                              false,
		"4f2b8c1e9a3d4e5f8b7a1c2d3e4f5a6b":     false,
	}
	for in, want := range cases {
		if got := isGUID(in); got != want {
			t.Errorf("isGUID(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCertificateFileProblem(t *testing.T) {
	dir := t.TempDir()
	pfx := filepath.Join(dir, "app.pfx")
	if err := os.WriteFile(pfx, []byte("pfx"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		path string
		want string // substring of the problem, "" when the file is usable
	}{
		"empty path":   {"", ""},
		"usable file":  {pfx, ""},
		"missing file": {filepath.Join(dir, "missing.pfx"), "does not exist"},
		"folder":       {dir, "is a folder"},
	}
	for name, c := range cases {
		got := certificateFileProblem(c.path)
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: certificateFileProblem(%q) = %q, want %q", name, c.path, got, c.want)
		}
	}
}
