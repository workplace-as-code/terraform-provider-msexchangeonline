package client

import (
	"strings"
	"testing"
)

// TestQuarantineGlobalSettingsScript guards quarantine_global_settings.ps1: the
// object is created when it is only virtual, and the four language lists are
// always written whole and in the same order.
func TestQuarantineGlobalSettingsScript(t *testing.T) {
	out := runStub(t, "quarantine_global_settings.ps1", "quarantine_global_settings.ps1")
	expectLines(t, out,
		"virtual calls: New DefaultGlobalTag GlobalQuarantinePolicy; Set EndUserSpamNotificationFrequency=04:00:00",
		"real calls: Set EndUserSpamNotificationFrequency=04:00:00",
		"EsnCustomSubject=,Objet",
		"MultiLanguageCustomDisclaimer=Help,d'assistance",
		"MultiLanguageSenderName=Sender,Détails",
		"MultiLanguageSetting=English,French",
		"clear calls: Set EndUserSpamNotificationCustomFromAddress=<null> EsnCustomSubject=<empty> MultiLanguageCustomDisclaimer=<empty> MultiLanguageSenderName=<empty> MultiLanguageSetting=<empty>",
		`get virtual: {"EndUserSpamNotificationFrequency":"1.00:00:00","EndUserSpamNotificationCustomFromAddress":"","OrganizationBrandingEnabled":false,"Languages":[]}`,
		`{"Language":"English","SenderName":"Sender","Subject":"","Disclaimer":"Help"}`,
		`{"Language":"French","SenderName":"Détails","Subject":"Objet","Disclaimer":"d'assistance"}`,
	)
	if strings.Contains(out, "real calls: New") {
		t.Errorf("a real object must not be created again:\n%s", out)
	}
}
