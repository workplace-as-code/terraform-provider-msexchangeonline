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

// TestOMEConfigurationSpec checks the branding templates: a script of their
// own, the logo as base64 text, and a list by Identity (they have no Name).
func TestOMEConfigurationSpec(t *testing.T) {
	spec, ok := findSpec("ome_configuration")
	if !ok {
		t.Fatal("no spec for ome_configuration")
	}
	if spec.script != "ome_configuration.ps1" {
		t.Errorf("ome_configuration needs its own script, got %q", spec.script)
	}
	if q := spec.listQuery(); q.property != "Identity" {
		t.Errorf("branding templates are listed by Identity, got %q", q.property)
	}
	have := map[string]field{}
	for _, f := range spec.fields {
		have[f.name] = f
	}
	for _, name := range []string{"background_color", "disclaimer_text", "email_text", "introduction_text",
		"portal_text", "privacy_statement_url", "read_button_text", "otp_enabled", "social_id_sign_in",
		"external_mail_expiry_in_days", "image"} {
		if _, ok := have[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
	// Exchange re-encodes the logo (it adds PNG chunks and padding): like an
	// OPATH filter, the state keeps the configured value as long as Exchange
	// returns what it returned after the last write.
	if f := have["image"]; f.kind != filterField {
		t.Error("image is base64 text that Exchange rewrites: it must be a filterField")
	}
}
