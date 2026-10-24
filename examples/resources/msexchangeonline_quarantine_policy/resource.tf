# End-users can view and request release
resource "msexchangeonline_quarantine_policy" "user_request" {
  name                                  = "UserRequestRelease"
  end_user_quarantine_permissions_value = 17
  esn_enabled                           = true
  quarantine_retention_days             = 30
}

# Notify-only — security team reviews all releases
resource "msexchangeonline_quarantine_policy" "admin_only" {
  name                                  = "AdminOnlyAccess"
  end_user_quarantine_permissions_value = 0
  esn_enabled                           = false
  quarantine_retention_days             = 30
}

# Full end-user control for low-risk detections
resource "msexchangeonline_quarantine_policy" "full_access" {
  name                                  = "FullAccess"
  end_user_quarantine_permissions_value = 23
  esn_enabled                           = true
  quarantine_retention_days             = 15
}
