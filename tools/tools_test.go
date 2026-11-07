// Package tools holds helper scripts for administrators. The tests keep them
// consistent with the bridge scripts embedded in the provider.
package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const roleGroupScript = "new_role_group.ps1"

var (
	cmdletRe   = regexp.MustCompile(`\b(?:Get|Set|New|Remove|Enable|Disable|Add)-[A-Z][A-Za-z]+`)
	functionRe = regexp.MustCompile(`(?m)^\s*function\s+([A-Za-z]+-[A-Za-z]+)`)
	listRe     = regexp.MustCompile(`(?s)\$ProviderCmdlets = @\((.*?)\n\)`)
	rmListRe   = regexp.MustCompile(`(?s)\$RoleManagementCmdlets = @\((.*?)\n\)`)
	rolesRe    = regexp.MustCompile(`(?s)\$ProviderRoles = @\((.*?)\n\)`)
	// customRoleRe matches one entry of $CustomRoles: name, parent, cmdlets.
	customRoleRe = regexp.MustCompile(`(?s)@\{ Name = "([^"]+)"; Parent = "([^"]+)"; Cmdlets = @\((.*?)\) \}`)
	// guideRoleRe matches the role list of the authentication guide: one
	// "- **Role**" item per role, alone on its line.
	guideRoleRe = regexp.MustCompile(`(?m)^- \*\*([^*]+)\*\*$`)
	quotedRe    = regexp.MustCompile(`"([^"]+)"`)
	// rbacNounRe matches the cmdlets of the Role Management role.
	rbacNounRe = regexp.MustCompile(`^[A-Za-z]+-(ManagementScope|ManagementRole|ManagementRoleEntry|RoleGroup|` +
		`ManagementRoleAssignment|RoleAssignmentPolicy|ServicePrincipal)$`)
	// Sync-BridgeRule (common.ps1) builds the rule cmdlet names from -Noun.
	ruleNounRe = regexp.MustCompile(`Sync-BridgeRule -Noun "([A-Za-z]+)"`)
	// Invoke-BridgePresetRule (common.ps1) does the same for the preset rules.
	presetNounRe = regexp.MustCompile(`Invoke-BridgePresetRule -Noun "([A-Za-z]+)"`)
	// restRe matches the REST cmdlets of the module, which use the permissions
	// of the server cmdlet without EXO: Get-EXORecipient needs Get-Recipient.
	restRe = regexp.MustCompile(`^Get-EXO([A-Z][A-Za-z]+)$`)
)

// ruleVerbs are the verbs Sync-BridgeRule and exo_rule.ps1 use on a rule noun.
var ruleVerbs = []string{"Get", "New", "Set", "Remove", "Enable", "Disable"}

// presetVerbs are the verbs Invoke-BridgePresetRule uses: the preset rules are
// adopted, never created or removed.
var presetVerbs = []string{"Get", "Set", "Enable", "Disable"}

var (
	specNounRe      = regexp.MustCompile(`noun:\s*"([A-Za-z]+)"`)
	specSingletonRe = regexp.MustCompile(`singleton:\s*true`)
	specAdoptedRe   = regexp.MustCompile(`adopted:\s*true`)
	specRuleRe      = regexp.MustCompile(`script:\s*"exo_rule\.ps1"`)
)

// specCmdlets returns the cmdlets the generic resources (internal/resources/specs.go)
// call through exo_object.ps1 and exo_rule.ps1, which build the names from the noun.
func specCmdlets(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../internal/resources/specs.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, block := range strings.Split(string(data), "\n\t{\n")[1:] {
		m := specNounRe.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		verbs := []string{"Get", "New", "Set", "Remove"}
		switch {
		case specSingletonRe.MatchString(block), specAdoptedRe.MatchString(block):
			verbs = []string{"Get", "Set"}
		case specRuleRe.MatchString(block):
			verbs = ruleVerbs
		}
		for _, v := range verbs {
			out = append(out, v+"-"+m[1])
		}
	}
	if len(out) == 0 {
		t.Fatal("no noun found in specs.go")
	}
	// The read-only objects (data sources only) are read with Get- only.
	readOnly, err := os.ReadFile("../internal/resources/readonly_specs.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range specNounRe.FindAllStringSubmatch(string(readOnly), -1) {
		out = append(out, "Get-"+m[1])
	}
	return out
}

// builtins are PowerShell or .NET helpers used by the bridge scripts, not
// Exchange cmdlets, so they need no management role. Set-CASMailbox,
// New-AcceptedDomain and Remove-AcceptedDomain only appear in comments.
var builtins = map[string]bool{
	"Get-Command": true, "Get-Variable": true, "Get-Date": true, "Get-Item": true, "Get-ChildItem": true,
	"Get-Content": true, "Get-Member": true, "Get-PSSession": true, "Get-ConnectionInformation": true,
	"New-Object": true, "New-PSSession": true, "New-PSSessionOption": true, "New-Guid": true,
	"New-SelfSignedCertificate": true, "Remove-PSSession": true, "Remove-Variable": true, "Remove-Item": true,
	"Set-StrictMode": true, "Set-Variable": true, "Set-Content": true, "Set-CASMailbox": true,
	"New-AcceptedDomain": true, "Remove-AcceptedDomain": true, "Add-Member": true,
}

func bridgeCmdlets(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../bridge/scripts/*.ps1")
	if err != nil || len(files) == 0 {
		t.Fatalf("no bridge scripts found: %v", err)
	}
	found := map[string]bool{}
	helpers := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range functionRe.FindAllStringSubmatch(string(data), -1) {
			helpers[m[1]] = true
		}
		for _, c := range cmdletRe.FindAllString(string(data), -1) {
			found[restRe.ReplaceAllString(c, "Get-$1")] = true
		}
		for _, m := range ruleNounRe.FindAllStringSubmatch(string(data), -1) {
			for _, verb := range ruleVerbs {
				found[verb+"-"+m[1]] = true
			}
		}
		for _, m := range presetNounRe.FindAllStringSubmatch(string(data), -1) {
			for _, verb := range presetVerbs {
				found[verb+"-"+m[1]] = true
			}
		}
	}
	for _, c := range specCmdlets(t) {
		found[c] = true
	}
	var out []string
	for c := range found {
		if !builtins[c] && !helpers[c] {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func TestRoleGroupScriptListsEveryBridgeCmdlet(t *testing.T) {
	data, err := os.ReadFile(roleGroupScript)
	if err != nil {
		t.Fatal(err)
	}
	block := listRe.FindStringSubmatch(string(data))
	if block == nil {
		t.Fatal("$ProviderCmdlets list not found")
	}
	rmBlock := rmListRe.FindStringSubmatch(string(data))
	if rmBlock == nil {
		t.Fatal("$RoleManagementCmdlets list not found")
	}
	// The Role Management cmdlets are only granted with -IncludeRoleManagement:
	// together, the two lists hold every cmdlet of the bridge, once.
	var listed []string
	seen := map[string]bool{}
	for _, b := range []string{block[1], rmBlock[1]} {
		for _, m := range quotedRe.FindAllStringSubmatch(b, -1) {
			if seen[m[1]] {
				t.Errorf("%s is listed twice", m[1])
			}
			seen[m[1]] = true
			listed = append(listed, m[1])
		}
	}
	sort.Strings(listed)
	for _, m := range quotedRe.FindAllStringSubmatch(rmBlock[1], -1) {
		if !rbacNounRe.MatchString(m[1]) {
			t.Errorf("%s is not a Role Management cmdlet", m[1])
		}
	}

	want := bridgeCmdlets(t)
	if strings.Join(listed, ",") != strings.Join(want, ",") {
		t.Errorf("$ProviderCmdlets in %s is out of date.\nlisted: %v\nbridge: %v", roleGroupScript, listed, want)
	}
}

func TestFindUncoveredCmdlets(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + `
$byCmdlet = @{
  "Get-TransportRule" = @("Transport Rules", "Organization Management")
  "Get-Mailbox"       = @("Mail Recipient Creation", "Retention Management")
  "New-Mailbox"       = @("Mail Recipient Creation")
}
$missing = Find-UncoveredCmdlets -RolesByCmdlet $byCmdlet -Roles @("Transport Rules", "Retention Management")
$none = Find-UncoveredCmdlets -RolesByCmdlet $byCmdlet -Roles @("Transport Rules", "Mail Recipient Creation")
@{ missing = @($missing); none = @($none) } | ConvertTo-Json -Compress
`
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("pwsh: %v\n%s", err, out)
	}
	var res struct {
		Missing []string `json:"missing"`
		None    []string `json:"none"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if strings.Join(res.Missing, ",") != "New-Mailbox" || len(res.None) != 0 {
		t.Errorf("got %+v, want only New-Mailbox uncovered, then nothing", res)
	}
}

func TestCustomRoleNames(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	run := func(args string) (string, error) {
		script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + ` ` + args + `
($CustomRoles | ForEach-Object { "$($_.Name)=$($_.Role)" }) -join "|"
`
		out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	out, err := run(`-RoleGroupName RG-TF`)
	if err != nil || !strings.Contains(out, "Retention=RG-TF Retention") {
		t.Errorf("default names: %v %s", err, out)
	}
	out, err = run(`-CustomRolePrefix CR-TF- -CustomRoleNames @{ Retention = "CR-RET" }`)
	if err != nil || !strings.Contains(out, "Retention=CR-RET") || !strings.Contains(out, "Sharing=CR-TF-Sharing") {
		t.Errorf("prefix and names: %v %s", err, out)
	}
	if out, err = run(`-CustomRoleNames @{ Retentions = "CR-RET" }`); err == nil {
		t.Errorf("an unknown custom role name must fail: %s", out)
	}
	if out, err = run(`-CustomRoleNames @{ Retention = "CR-X"; Sharing = "CR-X" }`); err == nil {
		t.Errorf("two custom roles with the same name must fail: %s", out)
	}
}

// TestGuideListsTheProviderRoles keeps the role list of the authentication
// guide in line with $ProviderRoles, so that the docs say exactly what the
// role group grants.
func TestGuideListsTheProviderRoles(t *testing.T) {
	script, err := os.ReadFile(roleGroupScript)
	if err != nil {
		t.Fatal(err)
	}
	block := rolesRe.FindStringSubmatch(string(script))
	if block == nil {
		t.Fatal("$ProviderRoles list not found")
	}
	var roles []string
	for _, m := range quotedRe.FindAllStringSubmatch(block[1], -1) {
		roles = append(roles, m[1])
	}
	guide, err := os.ReadFile(filepath.Join("..", "templates", "guides", "authentication.md"))
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, m := range guideRoleRe.FindAllStringSubmatch(string(guide), -1) {
		documented = append(documented, m[1])
	}
	if strings.Join(documented, ",") != strings.Join(roles, ",") {
		t.Errorf("the authentication guide lists %v, the script grants %v", documented, roles)
	}
}

// TestCustomRolesKeepTheBridgeCmdlets checks each custom role of $CustomRoles:
// for every noun of its list that the bridge runs, the role keeps exactly the
// cmdlets the bridge runs on that noun. A new cmdlet in a script cannot be
// forgotten, and the role never keeps more than needed. Nouns the bridge does
// not run yet are skipped: the roles of a release are granted before its
// resources are written, because roles take hours to reach the app.
func TestCustomRolesKeepTheBridgeCmdlets(t *testing.T) {
	script, err := os.ReadFile(roleGroupScript)
	if err != nil {
		t.Fatal(err)
	}
	roles := customRoleRe.FindAllStringSubmatch(string(script), -1)
	if len(roles) == 0 {
		t.Fatal("$CustomRoles entries not found")
	}
	bridgeByNoun := map[string][]string{}
	for _, c := range bridgeCmdlets(t) {
		noun := c[strings.Index(c, "-")+1:]
		bridgeByNoun[noun] = append(bridgeByNoun[noun], c)
	}
	for _, role := range roles {
		keptByNoun := map[string][]string{}
		for _, m := range quotedRe.FindAllStringSubmatch(role[3], -1) {
			noun := m[1][strings.Index(m[1], "-")+1:]
			keptByNoun[noun] = append(keptByNoun[noun], m[1])
		}
		for noun, kept := range keptByNoun {
			want, ok := bridgeByNoun[noun]
			if !ok {
				continue
			}
			sort.Strings(kept)
			if strings.Join(kept, ",") != strings.Join(want, ",") {
				t.Errorf("custom role %q keeps %v, the bridge runs %v", role[1], kept, want)
			}
		}
	}
}

// TestIncludeRoleManagement checks that Role Management and its cmdlets are
// only part of the role group with -IncludeRoleManagement.
func TestIncludeRoleManagement(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	run := func(args string) string {
		script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + ` ` + args + `
"$($ProviderRoles -contains 'Role Management') $($ProviderCmdlets -contains 'New-RoleGroup')"
`
		out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
		if err != nil {
			t.Fatalf("pwsh: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if out := run(""); out != "False False" {
		t.Errorf("without the switch: %s, want False False", out)
	}
	if out := run("-IncludeRoleManagement"); out != "True True" {
		t.Errorf("with -IncludeRoleManagement: %s, want True True", out)
	}
}

// TestDelegatingRoles: the roles the app may hand out get a delegating
// assignment, only with -IncludeRoleManagement (Exchange requires both to
// derive or assign a role).
func TestDelegatingRoles(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	run := func(args string) (string, error) {
		script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + ` ` + args + `
$DelegatingRoles -join "|"
`
		out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	if out, err := run(`-DelegatingRoles "Mail Recipients"`); err == nil {
		t.Errorf("-DelegatingRoles without -IncludeRoleManagement must fail: %s", out)
	}
	out, err := run(`-IncludeRoleManagement -DelegatingRoles "Mail Recipients","MyBaseOptions"`)
	if err != nil || out != "Mail Recipients|MyBaseOptions" {
		t.Errorf("with -IncludeRoleManagement: %v %s", err, out)
	}
}

// TestDelegationTarget: Exchange refuses an Application role on a role group,
// even as a delegation: the app's service principal gets it instead.
func TestDelegationTarget(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + `
@("Mail Recipients", "MyBaseOptions", "Application Mail.Send", "Exchange Application") | ForEach-Object { Get-DelegationTarget $_ }
`
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("pwsh: %v\n%s", err, out)
	}
	if got := strings.Join(strings.Fields(string(out)), ","); got != "SecurityGroup,SecurityGroup,App,App" {
		t.Errorf("got %s", got)
	}
}
