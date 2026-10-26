package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// authEnv lists every variable resolveConfig reads. Each case starts with all
// of them empty, so the shell running the tests (a sourced .env) cannot change
// the result.
var authEnv = []string{
	"EXCHANGE_TENANT_ID", "EXCHANGE_CLIENT_ID", "EXCHANGE_AUTH_METHOD", "EXCHANGE_ENVIRONMENT",
	"EXCHANGE_CERTIFICATE_PATH", "EXCHANGE_CERTIFICATE_PASSWORD", "EXCHANGE_CERTIFICATE_THUMBPRINT", "EXCHANGE_CERTIFICATE_BASE64",
	"EXCHANGE_OIDC_TOKEN", "EXCHANGE_OIDC_TOKEN_FILE_PATH",
	"ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN",
}

// secrets are values that must never appear in a diagnostic.
const (
	secretToken  = "jwt-secret-value"
	secretBase64 = "c2VjcmV0LXBmeA=="
)

func TestResolveConfig(t *testing.T) {
	dir := t.TempDir()
	pfx := filepath.Join(dir, "app.pfx")
	tokenFile := filepath.Join(dir, "token.jwt")
	for _, f := range []string{pfx, tokenFile} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	base := map[string]string{"tenant_id": "contoso.onmicrosoft.com", "client_id": "00000000-0000-0000-0000-000000000001"}
	github := map[string]string{"ACTIONS_ID_TOKEN_REQUEST_URL": "https://token.example/x?api-version=2.0", "ACTIONS_ID_TOKEN_REQUEST_TOKEN": "req"}

	cases := map[string]struct {
		attrs     map[string]string
		env       map[string]string
		wantError string // substring of a diagnostic, "" when the configuration is valid
		wantOIDC  bool
		check     func(t *testing.T, base64 string)
	}{
		"default is client_certificate": {
			attrs: map[string]string{"certificate_path": pfx},
		},
		"leftover oidc env is ignored by default": {
			attrs: map[string]string{"certificate_path": pfx},
			env:   map[string]string{"EXCHANGE_OIDC_TOKEN": secretToken},
		},
		"oidc_github": {
			attrs:    map[string]string{"auth_method": "oidc_github"},
			env:      github,
			wantOIDC: true,
		},
		"oidc_github from env": {
			env:      merge(github, map[string]string{"EXCHANGE_AUTH_METHOD": "oidc_github"}),
			wantOIDC: true,
		},
		"oidc_github without GitHub variables": {
			attrs:     map[string]string{"auth_method": "oidc_github"},
			wantError: "id-token: write",
		},
		"oidc with oidc_token": {
			attrs:    map[string]string{"auth_method": "oidc", "oidc_token": secretToken},
			wantOIDC: true,
		},
		"oidc with oidc_token_file_path": {
			attrs:    map[string]string{"auth_method": "oidc", "oidc_token_file_path": tokenFile},
			wantOIDC: true,
		},
		"oidc with both sources": {
			attrs:     map[string]string{"auth_method": "oidc", "oidc_token": secretToken, "oidc_token_file_path": tokenFile},
			wantError: "only one",
		},
		"oidc without source": {
			attrs:     map[string]string{"auth_method": "oidc"},
			wantError: "oidc_token",
		},
		"oidc_token_file_path missing file": {
			attrs:     map[string]string{"auth_method": "oidc", "oidc_token_file_path": filepath.Join(dir, "missing.jwt")},
			wantError: filepath.Join(dir, "missing.jwt"),
		},
		"oidc with a certificate": {
			attrs:     map[string]string{"auth_method": "oidc", "oidc_token": secretToken},
			env:       map[string]string{"EXCHANGE_CERTIFICATE_PATH": pfx},
			wantError: "choose one",
		},
		"client_certificate with oidc_token": {
			attrs:     map[string]string{"auth_method": "client_certificate", "certificate_path": pfx, "oidc_token": secretToken},
			wantError: "choose one",
		},
		"oidc on gcchigh": {
			attrs:     map[string]string{"auth_method": "oidc_github", "environment": "gcchigh"},
			env:       github,
			wantError: "only supported in the global and gcc environments",
		},
		"oidc on gcc": {
			attrs:    map[string]string{"auth_method": "oidc_github", "environment": "gcc"},
			env:      github,
			wantOIDC: true,
		},
		"unknown auth_method from env": {
			attrs:     map[string]string{"certificate_path": pfx},
			env:       map[string]string{"EXCHANGE_AUTH_METHOD": "foo"},
			wantError: "client_certificate, oidc, oidc_github",
		},
		"two certificate sources": {
			attrs:     map[string]string{"certificate_path": pfx, "certificate_base64": secretBase64},
			wantError: "only one",
		},
		"no certificate": {
			wantError: "certificate_path, certificate_base64 or certificate_thumbprint",
		},
		"certificate base64": {
			attrs: map[string]string{"certificate_base64": secretBase64},
			check: func(t *testing.T, b64 string) {
				if b64 != secretBase64 {
					t.Errorf("CertificateBase64 = %q", b64)
				}
			},
		},
		"certificate base64 with newlines": {
			env: map[string]string{"EXCHANGE_CERTIFICATE_BASE64": "c2VjcmV0\nLXBmeA==\n"},
			check: func(t *testing.T, b64 string) {
				if b64 != secretBase64 {
					t.Errorf("CertificateBase64 = %q, want the blanks removed", b64)
				}
			},
		},
		"invalid base64": {
			attrs:     map[string]string{"certificate_base64": "not base64 " + secretBase64},
			wantError: "not valid base64",
		},
		"short tenant name with oidc": {
			attrs:    map[string]string{"tenant_id": "contoso", "auth_method": "oidc", "oidc_token": secretToken},
			wantOIDC: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			for _, k := range authEnv {
				t.Setenv(k, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			cfg, diags := resolveConfig(model(merge(base, c.attrs)))

			var messages []string
			for _, d := range diags {
				messages = append(messages, d.Summary()+": "+d.Detail())
			}
			all := strings.Join(messages, "\n")
			if strings.Contains(all, secretToken) || strings.Contains(all, secretBase64) {
				t.Errorf("a diagnostic leaks a secret:\n%s", all)
			}

			switch {
			case c.wantError != "" && !strings.Contains(all, c.wantError):
				t.Fatalf("want an error containing %q, got:\n%s", c.wantError, all)
			case c.wantError == "" && diags.HasError():
				t.Fatalf("unexpected error:\n%s", all)
			case c.wantError != "":
				return
			}
			if got := cfg.TokenSource != nil; got != c.wantOIDC {
				t.Errorf("TokenSource set = %v, want %v", got, c.wantOIDC)
			}
			if c.check != nil {
				c.check(t, cfg.CertificateBase64)
			}
		})
	}
}

func TestOrganization(t *testing.T) {
	for in, want := range map[string]string{
		"contoso":                 "contoso.onmicrosoft.com",
		"contoso.onmicrosoft.com": "contoso.onmicrosoft.com",
	} {
		if got := organization(in); got != want {
			t.Errorf("organization(%q) = %q, want %q", in, got, want)
		}
	}
}

// model builds the provider block from attribute values; absent attributes are null.
func model(attrs map[string]string) exchangeProviderModel {
	v := func(name string) types.String {
		if s, ok := attrs[name]; ok {
			return types.StringValue(s)
		}
		return types.StringNull()
	}
	return exchangeProviderModel{
		TenantID:              v("tenant_id"),
		ClientID:              v("client_id"),
		AuthMethod:            v("auth_method"),
		CertificatePath:       v("certificate_path"),
		CertificatePassword:   v("certificate_password"),
		CertificateThumbprint: v("certificate_thumbprint"),
		CertificateBase64:     v("certificate_base64"),
		OIDCToken:             v("oidc_token"),
		OIDCTokenFilePath:     v("oidc_token_file_path"),
		Environment:           v("environment"),
	}
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
