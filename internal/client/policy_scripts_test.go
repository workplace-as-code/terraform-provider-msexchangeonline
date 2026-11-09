package client

import (
	"strings"
	"testing"
)

// TestHostedContentFilterQuarantineTag guards hosted_content_filter_policy.ps1:
// Exchange Online has no QuarantineTag parameter, the quarantine policy for spam
// is SpamQuarantineTag.
func TestHostedContentFilterQuarantineTag(t *testing.T) {
	out := runStub(t, "hosted_content_filter_policy.ps1", "hosted_content_filter_policy.ps1")
	expectLines(t, out, "-SpamQuarantineTag: AdminOnlyAccessPolicy", `"QuarantineTag":"AdminOnlyAccessPolicy"`)
	if strings.Contains(out, "-QuarantineTag:") {
		t.Errorf("-QuarantineTag does not exist in Exchange Online:\n%s", out)
	}
}
