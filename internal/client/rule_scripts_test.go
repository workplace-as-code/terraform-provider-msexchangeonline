package client

import (
	"os/exec"
	"strings"
	"testing"
)

// TestJournalRuleScript guards journal_rule.ps1: the state is read from
// Enabled (journal rules have no State), and an empty recipient clears it,
// since New-JournalRule and Set-JournalRule reject "".
func TestJournalRuleScript(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive",
		"-File", "testdata/journal_rule.ps1", "-Script", "../../bridge/scripts/journal_rule.ps1").CombinedOutput()
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

// TestPresetRuleScript guards the preset security policy rule scripts
// (Invoke-BridgePresetRule): the rule is adopted, never created, and
// destroying the resource turns the preset off.
func TestPresetRuleScript(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive",
		"-File", "testdata/preset_rule.ps1", "-Script", "../../bridge/scripts/eop_protection_policy_rule.ps1").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, want := range []string{
		"missing: 1",
		"Defender portal",
		// A one-element list comes back as a string; toStrings accepts both.
		`create: {"Name":"Standard Preset Security Policy","SentToMemberOf":"Pilot","Enabled":true}`,
		"create calls: Set SentToMemberOf; Enable",
		"delete: {}",
		"delete calls: Disable | state Disabled",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
