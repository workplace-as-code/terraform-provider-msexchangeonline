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
	cmdletRe   = regexp.MustCompile(`\b(?:Get|Set|New|Remove|Enable|Disable)-[A-Z][A-Za-z]+`)
	functionRe = regexp.MustCompile(`(?m)^\s*function\s+([A-Za-z]+-[A-Za-z]+)`)
	listRe     = regexp.MustCompile(`(?s)\$ProviderCmdlets = @\((.*?)\n\)`)
	quotedRe   = regexp.MustCompile(`"([^"]+)"`)
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
		case specSingletonRe.MatchString(block):
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
	"New-AcceptedDomain": true, "Remove-AcceptedDomain": true,
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
	var listed []string
	for _, m := range quotedRe.FindAllStringSubmatch(block[1], -1) {
		listed = append(listed, m[1])
	}
	sort.Strings(listed)

	want := bridgeCmdlets(t)
	if strings.Join(listed, ",") != strings.Join(want, ",") {
		t.Errorf("$ProviderCmdlets in %s is out of date.\nlisted: %v\nbridge: %v", roleGroupScript, listed, want)
	}
}

func TestSelectBridgeRoles(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	// "Broad" contains every needed cmdlet but weighs 500: the two narrow roles
	// must be preferred even though it takes two of them.
	script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + `
$byCmdlet = @{
  "Get-TransportRule" = @("Transport Rules", "Broad")
  "Set-TransportRule" = @("Transport Rules", "Broad")
  "Get-RemoteDomain"  = @("Remote and Accepted Domains", "Broad")
}
$a = Select-BridgeRoles -RolesByCmdlet $byCmdlet -RoleWeight @{ "Broad" = 500; "Transport Rules" = 10; "Remote and Accepted Domains" = 12 }
$byCmdlet2 = @{
  "Get-TransportRule" = @("Transport Rules", "Big Transport")
  "Set-TransportRule" = @("Transport Rules", "Big Transport")
  "Get-RemoteDomain"  = @("Remote and Accepted Domains")
}
$b = Select-BridgeRoles -RolesByCmdlet $byCmdlet2 -RoleWeight @{ "Big Transport" = 300; "Transport Rules" = 10; "Remote and Accepted Domains" = 12 }
# What Exchange Online returns for the two recipient cmdlets of the bridge:
# three roles grant all their parameters, only View-Only Recipients just reads.
# It is the largest of the three, so weighing roles by size alone picks Mail
# Recipient Creation.
$gets = 1..80 | ForEach-Object { "Get-Thing$_" }
$writes = 1..35 | ForEach-Object { "New-Thing$_" }
$byCmdlet3 = @{
  "Get-Mailbox"   = @("Mail Recipient Creation", "Mail Recipients", "View-Only Recipients")
  "Get-Recipient" = @("Mail Recipient Creation", "Mail Recipients", "View-Only Recipients")
}
$weights = @{
  "Mail Recipient Creation" = Get-RoleWeight -Cmdlets (@($gets[0..29]) + $writes)
  "Mail Recipients"         = Get-RoleWeight -Cmdlets ($gets + $writes + $writes)
  "View-Only Recipients"    = Get-RoleWeight -Cmdlets $gets
}
$c = Select-BridgeRoles -RolesByCmdlet $byCmdlet3 -RoleWeight $weights
# "Small" is taken first (best ratio), then "Wide" for the rest: Wide also
# covers Get-A, so Small is dropped.
$byCmdlet4 = @{
  "Get-A" = @("Small", "Wide")
  "Get-B" = @("Wide")
  "Get-C" = @("Wide")
}
$d = Select-BridgeRoles -RolesByCmdlet $byCmdlet4 -RoleWeight @{ "Small" = 3; "Wide" = 10 }
# Exchange Online as seen on 2026-10-30: Compliance Admin grants the recipient
# reads and many writes the provider does not run; Retention Management, needed
# anyway for the retention policies, also grants the recipient reads.
$needed = @("Get-Mailbox", "Get-Recipient", "New-RetentionPolicy", "Get-JournalRule", "New-JournalRule")
$byCmdlet5 = @{
  "Get-Mailbox"         = @("Compliance Admin", "Retention Management", "View-Only Recipients")
  "Get-Recipient"       = @("Compliance Admin", "Retention Management", "View-Only Recipients", "Journaling")
  "New-RetentionPolicy" = @("Retention Management", "Compliance Admin")
  "Get-JournalRule"     = @("Journaling", "Compliance Admin")
  "New-JournalRule"     = @("Journaling", "Compliance Admin")
}
$weights5 = @{
  "Compliance Admin"     = Get-RoleWeight -Needed $needed -Cmdlets ($needed + @($gets[0..39]) + $writes)
  "Retention Management" = Get-RoleWeight -Needed $needed -Cmdlets @("Get-Mailbox", "Get-Recipient", "New-RetentionPolicy", "Get-Thing1")
  "View-Only Recipients" = Get-RoleWeight -Needed $needed -Cmdlets (@("Get-Mailbox", "Get-Recipient") + $gets)
  "Journaling"           = Get-RoleWeight -Needed $needed -Cmdlets @("Get-Recipient", "Get-JournalRule", "New-JournalRule")
}
$e = Select-BridgeRoles -RolesByCmdlet $byCmdlet5 -RoleWeight $weights5
$err = ""
try { Select-BridgeRoles -RolesByCmdlet @{ "Get-Nothing" = @() } | Out-Null } catch { $err = $_.Exception.Message }
$w = Get-RoleWeight -Cmdlets @("Get-A", "Get-B", "Set-A", "Set-B") -Needed @("Set-B")
@{ a = @($a); b = @($b); c = @($c); d = @($d); e = @($e); err = $err; w = $w } | ConvertTo-Json -Compress
`
	cmd := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pwsh: %v\n%s", err, out)
	}
	var res struct {
		A   []string `json:"a"`
		B   []string `json:"b"`
		C   []string `json:"c"`
		D   []string `json:"d"`
		E   []string `json:"e"`
		Err string   `json:"err"`
		W   int      `json:"w"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if strings.Join(res.A, ",") != "Remote and Accepted Domains,Transport Rules" {
		t.Errorf("narrow roles must beat a broad one: got %v", res.A)
	}
	if strings.Join(res.B, ",") != "Remote and Accepted Domains,Transport Rules" {
		t.Errorf("got %v", res.B)
	}
	if strings.Join(res.C, ",") != "View-Only Recipients" {
		t.Errorf("a read-only role must beat roles that also write: got %v", res.C)
	}
	if strings.Join(res.D, ",") != "Wide" {
		t.Errorf("a role covered by the others must be dropped: got %v", res.D)
	}
	if strings.Join(res.E, ",") != "Journaling,Retention Management" {
		t.Errorf("roles needed anyway must cover the recipient reads: got %v", res.E)
	}
	if res.W != 104 {
		t.Errorf("weight of two Get-, one needed and one unneeded Set- cmdlet: got %d, want 104", res.W)
	}
	if !strings.Contains(res.Err, "Get-Nothing") {
		t.Errorf("uncovered cmdlet not reported: %q", res.Err)
	}
}

func TestSelectFullRoles(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	// Mail Tips grants Set-OrganizationConfig for 12 of its 126 parameters: it
	// must not count as a role that can run the cmdlet.
	script := `
$env:MSEXCHANGEONLINE_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + `
$full = Select-FullRoles -ParameterCount @{ "Organization Configuration" = 126; "Mail Tips" = 12; "Wider" = 130 }
$none = Select-FullRoles -ParameterCount @{}
# Get-Mailbox in Exchange Online: Compliance Admin grants more parameters than
# the read-only View-Only Recipients, which must stay a candidate.
$read = Select-FullRoles -Read -ParameterCount @{ "Compliance Admin" = 34; "View-Only Recipients" = 28; "Retention Management" = 21 }
@{ full = @($full); none = @($none); read = @($read) } | ConvertTo-Json -Compress
`
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("pwsh: %v\n%s", err, out)
	}
	var res struct {
		Full []string `json:"full"`
		None []string `json:"none"`
		Read []string `json:"read"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if strings.Join(res.Full, ",") != "Organization Configuration,Wider" || len(res.None) != 0 {
		t.Errorf("got %+v, want the two roles with (almost) every parameter and nothing for no role", res)
	}
	if strings.Join(res.Read, ",") != "Compliance Admin,Retention Management,View-Only Recipients" {
		t.Errorf("a read keeps every role: got %v", res.Read)
	}
}
