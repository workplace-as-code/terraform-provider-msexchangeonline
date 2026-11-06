package resources

import "testing"

// TestIRMConfigurationSpec checks the IRM configuration: a tenant-wide
// setting with every Exchange Online parameter of Set-IRMConfiguration except
// the legacy ones (AD RMS locations, Exchange Server only parameters).
func TestIRMConfigurationSpec(t *testing.T) {
	spec, ok := findSpec("irm_configuration")
	if !ok {
		t.Fatal("no spec for irm_configuration")
	}
	if !spec.singleton {
		t.Error("irm_configuration is a tenant-wide setting")
	}
	want := []string{"automatic_service_update_enabled", "azure_rms_licensing_enabled",
		"decrypt_attachment_for_encrypt_only", "ediscovery_super_user_enabled", "enable_pdf_encryption",
		"enable_portal_tracking_logs", "internal_licensing_enabled", "journal_report_decryption_enabled",
		"reject_if_recipient_has_no_rights", "search_enabled", "simplified_client_access_enabled",
		"simplified_client_access_encrypt_only_disabled", "simplified_client_access_do_not_forward_disabled",
		"transport_decryption_setting"}
	have := map[string]field{}
	for _, f := range spec.fields {
		have[f.name] = f
	}
	for _, name := range want {
		if _, ok := have[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
	for _, legacy := range []string{"LicensingLocation", "RMSOnlineKeySharingLocation", "ExternalLicensingEnabled",
		"ClientAccessServerEnabled"} {
		for _, f := range spec.fields {
			if f.param == legacy {
				t.Errorf("%s is a legacy or Exchange Server parameter", legacy)
			}
		}
	}
	if f, ok := have["transport_decryption_setting"]; ok && (f.kind != enumField || len(f.values) != 3) {
		t.Error("transport_decryption_setting is Disabled, Optional or Mandatory")
	}
}
