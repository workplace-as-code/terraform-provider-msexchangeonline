package client

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDkimKeySizeUnderStrictMode guards dkim_signing_config.ps1: reading a
// KeySize property, which Get-DkimSigningConfig does not have, failed every
// read under StrictMode.
func TestDkimKeySizeUnderStrictMode(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive",
		"-File", "testdata/dkim_keysize.ps1", "-Script", "../../bridge/scripts/dkim_signing_config.ps1").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, want := range []string{"no KeySize property: 2048", "after rotation to selector2: 2048", "nothing reported: 0"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
