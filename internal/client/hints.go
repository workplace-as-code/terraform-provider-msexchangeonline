package client

import "strings"

// hints maps fragments of a known Exchange error message (all must appear) to
// advice that tells the user how to fix it.
var hints = []struct {
	fragments []string
	advice    string
}{
	{
		[]string{"Enable-OrganizationCustomization"},
		"This tenant has never been customized. An Exchange administrator must run Enable-OrganizationCustomization once " +
			"(it can take up to an hour to apply) before custom policies can be created.",
	},
	{
		[]string{"-SafeLinks", "is not recognized"},
		"Safe Links cmdlets only exist in tenants licensed for Microsoft Defender for Office 365 (Plan 1 or 2).",
	},
	{
		[]string{"-SafeAttachment", "is not recognized"},
		"Safe Attachments cmdlets only exist in tenants licensed for Microsoft Defender for Office 365 (Plan 1 or 2).",
	},
}

// withHint trims the noise PowerShell adds to some messages and appends the
// advice for known errors.
func withHint(msg string) string {
	msg = strings.TrimLeft(msg, "| ")
	for _, h := range hints {
		if containsAll(msg, h.fragments) {
			return msg + "\n\n" + h.advice
		}
	}
	return msg
}

func containsAll(s string, fragments []string) bool {
	for _, f := range fragments {
		if !strings.Contains(s, f) {
			return false
		}
	}
	return true
}
