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

// TestManagementRoleScript guards management_role.ps1: the role entries match
// the configuration with the fewest calls, the case of a cmdlet never causes a
// change, and built-in roles are refused.
func TestManagementRoleScript(t *testing.T) {
	out := runStub(t, "management_role.ps1", "management_role.ps1")
	expectLines(t, out,
		"create calls: New TF-CI-Role Parent=Mail Recipients EnabledCmdlets=Get-Mailbox,Set-User; "+
			`SetEntry TF-CI-Role\Set-User Identity,Phone`,
		`"Cmdlets":["Get-Mailbox","Set-User"]`,
		`"CmdletParameters":{"Set-User":["Identity","Phone"]}`,
		`update calls: Add TF-CI-Role\Get-User; Remove TF-CI-Role\Set-User`,
		`"Cmdlets":["Get-Mailbox","Get-User"]`,
		`restrict calls: Add TF-CI-Role\Set-User; SetEntry TF-CI-Role\Set-User Identity,Phone`,
		`unrestrict calls: SetEntry TF-CI-Role\Set-User Department,Identity,MobilePhone,Phone`,
		"Mail Recipients is a built-in role: only custom roles can be managed",
		"New-Mailbox is not in the parent role Mail Recipients",
		"notinparent calls: \n",
	)
}
