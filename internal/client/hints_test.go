package client

import (
	"strings"
	"testing"
)

func TestWithHint(t *testing.T) {
	got := withHint("||The command you tried to run isn't currently allowed in your organization. To run this command, you first need to run the command: Enable-OrganizationCustomization.")
	if strings.HasPrefix(got, "|") {
		t.Errorf("leading pipes not trimmed: %q", got)
	}
	if !strings.Contains(got, "must run Enable-OrganizationCustomization once") {
		t.Errorf("advice missing: %q", got)
	}
	if got := withHint("The term 'Get-SafeLinksPolicy' is not recognized as a name of a cmdlet"); !strings.Contains(got, "Defender for Office 365") {
		t.Errorf("Defender advice missing: %q", got)
	}
	if got := withHint("Some other error"); got != "Some other error" {
		t.Errorf("unknown error changed: %q", got)
	}
}
