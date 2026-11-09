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

// TestNotFoundIsAMissingObject guards Test-BridgeNotFound (common.ps1): a
// missing object makes the resource leave the state, an unknown or unreachable
// server is an error.
func TestNotFoundIsAMissingObject(t *testing.T) {
	out := runStub(t, "not_found.ps1", "common.ps1")
	expectLines(t, out, "object: True", "graph: True", "server: False", "network: False", "denied: False")
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
		"Set-User in the parent role has no parameter Bogus",
		"badparam calls: []",
	)
}

// TestRoleAssignmentScript guards management_role_assignment.ps1.
func TestRoleAssignmentScript(t *testing.T) {
	out := runStub(t, "management_role_assignment.ps1", "management_role_assignment.ps1")
	expectLines(t, out,
		"group calls: New CustomRecipientWriteScope=TF-CI-Scope,Role=TF-CI-Role,SecurityGroup=TF-CI-RoleGroup",
		`"RoleGroup":"TF-CI-RoleGroup"`,
		`"CustomRecipientWriteScope":"TF-CI-Scope"`,
		"app calls: New App=11111111-2222-3333-4444-555555555555,CustomResourceScope=TF-CI-Scope,Role=Application Mail.Read",
		`"App":"11111111-2222-3333-4444-555555555555"`,
		"unscope calls: Set Enabled=False,RecipientRelativeWriteScope=Organization",
		`"RecipientRelativeWriteScope":"Organization"`,
		`au: {"Name":"TF-CI-AU","Role":"Mail Recipients","RoleGroup":"TF-CI-RoleGroup","App":"","CustomRecipientWriteScope":"",`+
			`"ExclusiveRecipientWriteScope":"","CustomResourceScope":"","RecipientAdministrativeUnitScope":"au-1111",`+
			`"RecipientGroupScope":""`,
		`group: {"Name":"TF-CI-Group","Role":"Mail Recipients","RoleGroup":"TF-CI-RoleGroup","App":"","CustomRecipientWriteScope":"",`+
			`"ExclusiveRecipientWriteScope":"","CustomResourceScope":"","RecipientAdministrativeUnitScope":"",`+
			`"RecipientGroupScope":"Helpdesk Users"`,
	)
}

// TestRoleAssignmentListSkipsBuiltIn: terraform query lists the regular
// assignments of custom role groups and of applications only.
func TestRoleAssignmentListSkipsBuiltIn(t *testing.T) {
	out := runStub(t, "list_rbac.ps1", "list.ps1")
	expectLines(t, out, `assignments: ["TF-CI-Role-TF-CI-RoleGroup","Application Mail.Read-TF-CI-App"]`)
}

// TestServicePrincipalScript guards service_principal.ps1.
func TestServicePrincipalScript(t *testing.T) {
	out := runStub(t, "service_principal.ps1", "service_principal.ps1")
	expectLines(t, out,
		"missing: []",
		`create: {"AppId":"11111111-2222-3333-4444-555555555555","ObjectId":"obj-1","DisplayName":"TF-CI-App"}`,
		`"DisplayName":"TF-CI-App-2"`,
		"delete: {}",
		"calls: New AppId=11111111-2222-3333-4444-555555555555,DisplayName=TF-CI-App,ObjectId=obj-1; "+
			"Set sp-obj-1 DisplayName=TF-CI-App-2; Remove sp-obj-1",
	)
}

// TestRoleAssignmentPolicyRoles guards role_assignment_policy.ps1: only the
// roles that differ are added or removed, whatever their case.
func TestRoleAssignmentPolicyRoles(t *testing.T) {
	out := runStub(t, "role_assignment_policy.ps1", "role_assignment_policy.ps1")
	expectLines(t, out,
		`"Roles":["MyBaseOptions","MyVoiceMail"]`,
		"update calls: Add MyTextMessaging; Remove MyVoiceMail",
		`"Roles":["MyBaseOptions","MyTextMessaging"]`,
		"delete: {}",
		"delete calls: Remove MyBaseOptions; Remove MyTextMessaging; RemovePolicy TF-CI-Policy",
	)
}

// TestOmeConfigurationScript guards ome_configuration.ps1.
func TestOmeConfigurationScript(t *testing.T) {
	out := runStub(t, "ome_configuration.ps1", "ome_configuration.ps1")
	expectLines(t, out,
		`default: {"Name":"OME Configuration","DisclaimerText":"Confidential"`,
		"default delete: {}",
		`"Image":"iVBORw==","ExternalMailExpiryInDays":7`,
		"custom delete: {}",
		"calls: Set OME Configuration DisclaimerText=String; New TF-CI-OME ExternalMailExpiryInDays=Int32,Image=Byte[]; Remove TF-CI-OME",
	)
}

// TestManagementRoleInfoScript guards management_role_info.ps1: built-in roles
// are read like custom ones, and a missing role is empty output.
func TestManagementRoleInfoScript(t *testing.T) {
	out := runStub(t, "management_role_info.ps1", "management_role_info.ps1")
	expectLines(t, out,
		`builtin: {"Name":"Transport Rules","Description":"Built in","Parent":"","BuiltIn":true,"Cmdlets":["Get-Mailbox","Set-User"]}`,
		`custom: {"Name":"TF-CI-Role","Description":"","Parent":"Mail Recipients","BuiltIn":false,"Cmdlets":["Get-Mailbox","Set-User"]}`,
		"missing: []",
	)
}
