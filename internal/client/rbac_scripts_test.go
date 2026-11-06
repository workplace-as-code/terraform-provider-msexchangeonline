package client

import (
	"os/exec"
	"strings"
	"testing"
)

// runStub runs a testdata script that fakes Exchange cmdlets around a bridge script.
func runStub(t *testing.T, stub, script string) string {
	t.Helper()
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive",
		"-File", "testdata/"+stub, "-Script", "../../bridge/scripts/"+script).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return string(out)
}

func expectLines(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

// TestRBACListSkipsBuiltIn guards list.ps1: terraform query lists custom role
// groups and roles only, so the generated configuration never manages the
// built-in ones.
func TestRBACListSkipsBuiltIn(t *testing.T) {
	out := runStub(t, "list_rbac.ps1", "list.ps1")
	expectLines(t, out, `role groups: ["TF-CI-RoleGroup"]`, `roles: ["TF-CI-Role"]`)
}
