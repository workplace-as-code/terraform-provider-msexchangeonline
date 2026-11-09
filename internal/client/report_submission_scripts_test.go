package client

import (
	"strings"
	"testing"
)

// TestReportSubmissionScript guards report_submission_policy.ps1: the policy is
// created when it is only virtual, one mailbox keeps the policy and the rule in
// step, and every language list is written whole.
func TestReportSubmissionScript(t *testing.T) {
	out := runStub(t, "report_submission_policy.ps1", "report_submission_policy.ps1")
	const flagsTrue = "ReportJunkToCustomizedAddress=True ReportNotJunkAddresses=a@contoso.com " +
		"ReportNotJunkToCustomizedAddress=True ReportPhishAddresses=a@contoso.com ReportPhishToCustomizedAddress=True ThirdPartyReportAddresses=<null>"
	expectLines(t, out,
		"virtual calls: New policy; Set EnableUserEmailNotification=True",
		"real calls: Set EnableUserEmailNotification=True",
		"microsoft calls: Set ReportJunkAddresses=a@contoso.com "+flagsTrue+"; New rule DefaultReportSubmissionRule DefaultReportSubmissionPolicy a@contoso.com",
		"EnableThirdPartyAddress=True ReportJunkAddresses=<null> ReportJunkToCustomizedAddress=False",
		"ReportPhishToCustomizedAddress=False ThirdPartyReportAddresses=a@contoso.com; New rule",
		"existing calls: Set ReportJunkAddresses=b@contoso.com",
		"; Set rule DefaultReportSubmissionRule b@contoso.com",
		"none calls: Set ReportJunkAddresses=<null> ReportJunkToCustomizedAddress=False",
		"ThirdPartyReportAddresses=<null>; Remove rule DefaultReportSubmissionRule",
		"MultiLanguageSetting=en,fr",
		"MultiLanguagePreSubmitMessageForJunk=,d'assistance",
		"MultiLanguagePostSubmitMessageButtonLinkForPhishing=,",
		"gone mailbox: []",
		"differs mailbox: [b@contoso.com]",
		"alias mailbox: [alias@contoso.com]",
		"rulehand mailbox: [c@contoso.com]",
		"import mailbox: [a@contoso.com]",
		`"Language":"en"`, `"MultiLanguagePreSubmitMessageForJunk":"Détails"`,
	)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "languages calls:") && strings.Count(line, "MultiLanguage") != 21 {
			t.Errorf("all 21 lists must be sent, got %d:\n%s", strings.Count(line, "MultiLanguage"), line)
		}
	}
	if strings.Contains(out, "real calls: New") {
		t.Errorf("a real policy must not be created again:\n%s", out)
	}
}
