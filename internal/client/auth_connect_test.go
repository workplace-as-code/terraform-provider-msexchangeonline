package client

import (
	"encoding/json"
	"os/exec"
	"testing"
)

// TestAuthConnects checks how auth.ps1 calls Connect-ExchangeOnline with an
// OIDC access token and with a certificate passed in base64.
func TestAuthConnects(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	type result struct {
		Connected struct {
			AccessToken  string
			Organization string
			Thumbprint   string
		} `json:"connected"`
		ExpectedThumbprint string `json:"expectedThumbprint"`
	}
	connect := func(t *testing.T, name string) result {
		t.Helper()
		out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive",
			"-File", "testdata/auth_connect.ps1", "-Script", "../../bridge/scripts/auth.ps1", "-Case", name).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return r
	}

	t.Run("token", func(t *testing.T) {
		r := connect(t, "token")
		if r.Connected.AccessToken != "at-1" || r.Connected.Organization != "contoso.onmicrosoft.com" || r.Connected.Thumbprint != "" {
			t.Errorf("Connect-ExchangeOnline got %+v", r.Connected)
		}
	})
	for _, name := range []string{"base64", "base64-wrapped"} {
		t.Run(name, func(t *testing.T) {
			r := connect(t, name)
			if r.ExpectedThumbprint == "" || r.Connected.Thumbprint != r.ExpectedThumbprint || r.Connected.AccessToken != "" {
				t.Errorf("Connect-ExchangeOnline got %+v, want the certificate %s", r.Connected, r.ExpectedThumbprint)
			}
		})
	}
}
