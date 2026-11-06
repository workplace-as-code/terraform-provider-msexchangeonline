# Users can change their options but not their distribution groups
resource "msexchangeonline_role_assignment_policy" "default" {
  name  = "Default Role Assignment Policy"
  roles = ["MyBaseOptions", "MyContactInformation", "MyProfileInformation", "MyRetentionPolicies"]
}
