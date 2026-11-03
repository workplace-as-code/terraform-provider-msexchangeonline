# Keep the unified audit log on.
resource "msexchangeonline_admin_audit_log_config" "this" {
  unified_audit_log_ingestion_enabled = true
}
