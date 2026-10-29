package client

import (
	"os/exec"
	"strings"
	"testing"
)

// TestExoRuleJournalRule guards the journal rule specifics of exo_rule.ps1:
// the state is read from Enabled (journal rules have no State), and an empty
// recipient clears it, since New-JournalRule and Set-JournalRule reject "".
func TestExoRuleJournalRule(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive",
		"-File", "testdata/exo_rule_journal.ps1", "-Script", "../../bridge/scripts/exo_rule.ps1").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, want := range []string{
		`create: {"Name":"TF-Journal","JournalEmailAddress":"journal@archive.example","Recipient":null,"Scope":"Global","Enabled":false}`,
		"create calls: New Enabled,JournalEmailAddress,Name,Scope",
		`"Recipient":null,"Scope":"Global","Enabled":true}`,
		"update calls: Set Recipient=; Enable",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
