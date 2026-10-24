# Always accept mail from the on-premises relay, always reject a known spam
# source.
resource "msexchangeonline_hosted_connection_filter_policy" "this" {
  ip_allow_list    = ["203.0.113.10"]
  ip_block_list    = ["198.51.100.0/24"]
  enable_safe_list = false
}
