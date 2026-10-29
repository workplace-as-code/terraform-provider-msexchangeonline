#!/usr/bin/env bash
# run.sh — build the provider and run the full integration test cycle against
# a real tenant:
#   build → pre-cleanup → apply → drift check → update → drift check →
#   import of every resource → drift check → destroy → deletion check
#
# Test objects are named TF-CI-*. The initial accepted domain and the
# organization config are only imported: they are removed from state before
# the destroy, so the tenant keeps them unchanged.
#
# Usage:
#   ./test/run.sh
#
# Required env vars (exported, or read from .env at repo root; exported values win):
#   EXCHANGE_TENANT_ID
#   EXCHANGE_CLIENT_ID      (not with managed_identity, except for a user-assigned identity)
#   TF_VAR_primary_domain   (e.g. "yourcompany.onmicrosoft.com")
#   and the credentials of the authentication method (EXCHANGE_AUTH_METHOD):
#     client_certificate (default): EXCHANGE_CERTIFICATE_PATH or EXCHANGE_CERTIFICATE_BASE64,
#                                   and EXCHANGE_CERTIFICATE_PASSWORD
#     oidc_github                 : nothing more, inside a GitHub Actions job with id-token: write
#     oidc                        : EXCHANGE_OIDC_TOKEN or EXCHANGE_OIDC_TOKEN_FILE_PATH
#     managed_identity            : nothing more, on an Azure resource or Azure Arc-enabled server
#
# Requires terraform, go, pwsh and jq on the PATH.
#
# Optional:
#   TF_VAR_test_mailbox     (e.g. "user@yourcompany.com") — tests mailbox data source
#   TF_VAR_test_dkim_domain (e.g. "dkim.yourcompany.com") — tests dkim_signing_config on an
#                           accepted domain; TF_VAR_test_dkim_enabled=true also enables signing,
#                           once the two CNAME records printed by a first run are published
#   EXCHANGE_TEST_DEFENDER  (any value) — also tests the Defender for Office 365 resources
#                           (test/integration/defender.tf): Safe Links, Safe Attachments and
#                           the anti-phishing settings that need a Defender license
#   TF_DEV_PROVIDER_DIR     folder written in a new ~/.terraformrc (default /tmp). When
#                           ~/.terraformrc already has a dev override for this provider, the
#                           provider is built into the folder it names.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTDIR="$ROOT/test/integration"
TERRAFORMRC="$HOME/.terraformrc"
PROVIDER_ADDR="workplace-as-code/msexchangeonline"

# Colours
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

pass() { echo -e "${GREEN}  ✓ $*${NC}"; }
fail() { echo -e "${RED}  ✗ $*${NC}"; exit 1; }
step() { echo -e "\n${YELLOW}==> $*${NC}"; }
info() { echo -e "${CYAN}    $*${NC}"; }

# ---------------------------------------------------------------------------
# 1. Load credentials from .env if present
# ---------------------------------------------------------------------------
if [[ -f "$ROOT/.env" ]]; then
  # Variables already exported in the shell take precedence over .env, so a
  # stale .env can never silently replace the credentials you exported.
  preset="$(export -p)"
  set -a && source "$ROOT/.env" && set +a
  eval "$preset"
fi

# ---------------------------------------------------------------------------
# 2. Check prerequisites
# ---------------------------------------------------------------------------
step "Checking prerequisites"
for tool in terraform go pwsh jq; do
  command -v "$tool" > /dev/null || fail "$tool is not installed or not on the PATH"
done
AUTH_METHOD="${EXCHANGE_AUTH_METHOD:-client_certificate}"
REQUIRED_VARS=(EXCHANGE_TENANT_ID)
# A system-assigned managed identity has no client ID.
[[ "$AUTH_METHOD" != "managed_identity" ]] && REQUIRED_VARS+=(EXCHANGE_CLIENT_ID)
for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    fail "$var is not set. Add it to $ROOT/.env or export it."
  fi
done
# The provider checks the other methods' settings itself, with clearer messages.
if [[ "$AUTH_METHOD" == "client_certificate" && -z "${EXCHANGE_CERTIFICATE_PATH:-}" && -z "${EXCHANGE_CERTIFICATE_BASE64:-}" ]]; then
  fail "EXCHANGE_CERTIFICATE_PATH or EXCHANGE_CERTIFICATE_BASE64 is not set. Add one to $ROOT/.env or export it."
fi
if [[ -z "${TF_VAR_primary_domain:-}" ]]; then
  fail "TF_VAR_primary_domain is not set. Add it to $ROOT/.env or export it."
fi
pass "Credentials OK (tenant: $EXCHANGE_TENANT_ID, auth: $AUTH_METHOD)"
pass "Primary domain: $TF_VAR_primary_domain"

# ---------------------------------------------------------------------------
# 3. Check ~/.terraformrc dev_overrides
# ---------------------------------------------------------------------------
# The provider is built into the folder of its dev override, so Terraform
# always runs the binary built here.
DEV_DIR="$(sed -n "s#^ *\"$PROVIDER_ADDR\" *= *\"\([^\"]*\)\".*#\1#p" "$TERRAFORMRC" 2>/dev/null | head -1 || true)"
if [[ -n "$DEV_DIR" ]]; then
  pass "~/.terraformrc already configured ($DEV_DIR)"
elif grep -q 'dev_overrides' "$TERRAFORMRC" 2>/dev/null; then
  fail "~/.terraformrc has dev_overrides but no \"$PROVIDER_ADDR\" entry. Add one, for example \"$PROVIDER_ADDR\" = \"/tmp\"."
else
  DEV_DIR="${TF_DEV_PROVIDER_DIR:-/tmp}"
  step "Configuring ~/.terraformrc dev_overrides"
  cat > "$TERRAFORMRC" <<EOF
provider_installation {
  dev_overrides {
    "$PROVIDER_ADDR" = "$DEV_DIR"
  }
  direct {}
}
EOF
  pass "~/.terraformrc created ($DEV_DIR)"
fi
BINARY="$DEV_DIR/terraform-provider-msexchangeonline"

# ---------------------------------------------------------------------------
# 4. Build provider binary
# ---------------------------------------------------------------------------
step "Building provider binary → $BINARY"
cd "$ROOT"
go build -o "$BINARY" .
pass "Build OK"

# ---------------------------------------------------------------------------
# Objects created by the test, as "<resource address>=<import id>".
# ---------------------------------------------------------------------------
CREATED=(
  "msexchangeonline_transport_rule.test=TF-CI-Rule"
  "msexchangeonline_remote_domain.test=TF-CI-Domain"
  "msexchangeonline_inbound_connector.test=TF-CI-Inbound"
  "msexchangeonline_outbound_connector.test=TF-CI-Outbound"
  "msexchangeonline_anti_phish_rule.test=TF-CI-AntiPhishRule"
  "msexchangeonline_anti_phish_policy.test=TF-CI-AntiPhish"
  "msexchangeonline_anti_phish_policy.unscoped=TF-CI-AntiPhish-Unscoped"
  "msexchangeonline_malware_filter_rule.test=TF-CI-MalwareRule"
  "msexchangeonline_malware_filter_policy.test=TF-CI-MalwareFilter"
  "msexchangeonline_malware_filter_policy.unscoped=TF-CI-MalwareFilter-Unscoped"
  "msexchangeonline_hosted_content_filter_rule.test=TF-CI-SpamRule"
  "msexchangeonline_hosted_content_filter_policy.test=TF-CI-SpamFilter"
  "msexchangeonline_hosted_content_filter_policy.unscoped=TF-CI-SpamFilter-Unscoped"
  "msexchangeonline_outbound_spam_filter_policy.test=TF-CI-OutboundSpam"
  "msexchangeonline_quarantine_policy.test=TF-CI-Quarantine"
  "msexchangeonline_owa_mailbox_policy.test=TF-CI-OWA"
  "msexchangeonline_mobile_device_mailbox_policy.test=TF-CI-Mobile"
  "msexchangeonline_email_address_policy.test=TF-CI-AddressPolicy"
  "msexchangeonline_retention_policy.test=TF-CI-Retention"
  "msexchangeonline_retention_policy_tag.test=TF-CI-RetentionTag"
  "msexchangeonline_hosted_outbound_spam_filter_rule.test=TF-CI-OutboundSpamRule"
  "msexchangeonline_outbound_spam_filter_policy.unscoped=TF-CI-OutboundSpam-Unscoped"
  "msexchangeonline_journal_rule.test=TF-CI-Journal"
  "msexchangeonline_tenant_allow_block_list_item.sender=Sender:tf-ci-blocked@tf-ci-test.example"
  "msexchangeonline_tenant_allow_block_list_item.url=Url:tf-ci-test.example/blocked"
)
if [[ -n "${EXCHANGE_TEST_DEFENDER:-}" ]]; then
  export TF_VAR_test_defender=true
  CREATED+=(
    "msexchangeonline_safe_links_policy.test[0]=TF-CI-SafeLinks"
    "msexchangeonline_safe_links_rule.test[0]=TF-CI-SafeLinksRule"
    "msexchangeonline_safe_links_policy.unscoped[0]=TF-CI-SafeLinks-Unscoped"
    "msexchangeonline_safe_attachment_policy.test[0]=TF-CI-SafeAttachments"
    "msexchangeonline_safe_attachment_rule.test[0]=TF-CI-SafeAttachmentRule"
    "msexchangeonline_safe_attachment_policy.unscoped[0]=TF-CI-SafeAttachments-Unscoped"
    "msexchangeonline_anti_phish_policy.defender[0]=TF-CI-AntiPhish-Defender"
  )
fi
if [[ -n "${TF_VAR_test_mail_enabled_group:-}" && -n "${TF_VAR_test_app_id:-}" ]]; then
  # The policy ID is only known once it exists: it is imported by the loop below
  # from the state, and the deletion check cannot look it up by name.
  APP_POLICY="msexchangeonline_application_access_policy.test[0]"
fi
if [[ -n "${TF_VAR_test_dkim_domain:-}" ]]; then
  # A DKIM configuration cannot be deleted: the resource adopts it on create and
  # disables it on destroy, so it is imported and checked on its own.
  DKIM="msexchangeonline_dkim_signing_config.test[0]"
fi
# Tenant-wide objects imported by main.tf and never destroyed.
KEEP=(
  "msexchangeonline_organization_config.this"
  "msexchangeonline_hosted_connection_filter_policy.this"
  "msexchangeonline_external_in_outlook.this"
  "msexchangeonline_transport_config.this"
  "msexchangeonline_remote_domain.default"
  "msexchangeonline_hosted_content_filter_policy.default"
  "msexchangeonline_mobile_device_mailbox_policy.default"
)

# expect_no_changes <label> [terraform plan args...]
# Fails unless terraform plan reports no changes.
expect_no_changes() {
  local label="$1"; shift
  local out
  out="$(mktemp)"
  set +e
  terraform plan -detailed-exitcode -no-color "$@" > "$out" 2>&1
  local code=$?
  set -e
  case $code in
    0) pass "$label: no changes" ;;
    2) cat "$out"; fail "$label: plan shows changes" ;;
    *) cat "$out"; fail "$label: terraform plan failed (exit $code)" ;;
  esac
  rm -f "$out"
}

# import_orphans imports every test object that still exists in Exchange and
# prints the matching -target arguments.
import_orphans() {
  local spec
  for spec in "${CREATED[@]}"; do
    if terraform import -no-color "$@" "${spec%%=*}" "${spec#*=}" > /dev/null 2>&1; then
      echo "-target=${spec%%=*}"
    fi
  done
}

# leftover_app_policies prints the ID of every application access policy of the
# test app. Such a policy has no name, so import_orphans cannot find it: a run that
# stopped early leaves it behind and the next apply cannot create it again.
leftover_app_policies() {
  local dir out
  dir="$(mktemp -d)"
  printf 'terraform {\n  required_providers {\n    msexchangeonline = { source = "workplace-as-code/msexchangeonline" }\n  }\n}\nprovider "msexchangeonline" {}\n' > "$dir/main.tf"
  printf 'list "msexchangeonline_application_access_policy" "all" {\n  provider = msexchangeonline\n}\n' > "$dir/aap.tfquery.hcl"
  if ! (cd "$dir" && terraform query -no-color -generate-config-out=generated.tf > query.log 2>&1); then
    cat "$dir/query.log" >&2
    rm -rf "$dir"
    return 1
  fi
  # HCL doubles the backslash of "<tenant>\<app id>:<group>": undo it for terraform import.
  out=""
  # Only the "id" key: "app_id" also holds the app ID.
  [[ -f "$dir/generated.tf" ]] && out="$(grep -oE '(^|[[:space:]{])id *= *"[^"]*"' "$dir/generated.tf" |
    sed -E 's/^[[:space:]{]*id *= *"(.*)"$/\1/; s/\\\\/\\/g' | grep -iF "$TF_VAR_test_app_id" || true)"
  rm -rf "$dir"
  [[ -z "$out" ]] || echo "$out"
}

reset_state() { rm -f terraform.tfstate terraform.tfstate.*; }

# state_value <address> <attribute> prints an attribute from the state.
state_value() {
  local state
  # Capture first: with pipefail, a reader closing the pipe early would make
  # terraform exit with SIGPIPE.
  state="$(terraform state show -no-color "$1")"
  sed -n "s/^ *$2 *= *//p" <<< "$state" | head -1
}

# state_string <address> <attribute> prints a string attribute from the state as
# is. state show prints HCL, where a backslash is doubled: the application access
# policy ID ("<tenant>\<app id>:<group>") would then no longer be found.
state_string() {
  local state
  state="$(terraform show -json)"
  jq -r --arg a "$1" --arg k "$2" '.values.root_module.resources[] | select(.address == $a) | .values[$k]' <<< "$state"
}

# write_and_restore <address> <attribute> <variable> <test value> <original value>
# Writes a test value to a tenant-wide setting through a variable, checks it,
# then writes the original value back.
write_and_restore() {
  local addr="$1" attr="$2" var="$3" value="$4" original="$5"
  terraform apply -auto-approve -no-color -var phase=update -var "$var=$value" | tail -1
  [[ "$(state_value "$addr" "$attr")" != "$original" ]] || fail "$addr.$attr was not changed"
  terraform apply -auto-approve -no-color -var phase=update -var "$var=$original" | tail -1
  [[ "$(state_value "$addr" "$attr")" == "$original" ]] || fail "$addr.$attr was not restored"
  pass "$addr.$attr changed and restored"
}

# ---------------------------------------------------------------------------
# 5. Start from an empty state
# ---------------------------------------------------------------------------
step "Cleaning up previous test state"
cd "$TESTDIR"
rm -rf .terraform .terraform.lock.hcl
reset_state
pass "Clean"

step "terraform init"
# No -upgrade: dev_overrides bypass the registry, no download needed
terraform init -no-color 2>&1 | tail -1
pass "Init OK"

# ---------------------------------------------------------------------------
# 6. Pre-cleanup: delete test objects left over by an interrupted run
# ---------------------------------------------------------------------------
step "Pre-cleanup: removing any orphaned test objects"
mapfile -t ORPHANS < <(import_orphans)
if [[ ${#ORPHANS[@]} -gt 0 ]]; then
  info "${#ORPHANS[@]} orphaned object(s) found, destroying..."
  terraform destroy -auto-approve -no-color "${ORPHANS[@]}" | tail -1
  pass "Orphans destroyed"
else
  pass "No orphaned objects found"
fi
reset_state
if [[ -n "${APP_POLICY:-}" ]]; then
  leftover="$(leftover_app_policies)" || fail "Listing the application access policies failed"
  if [[ -n "$leftover" ]]; then
    while IFS= read -r id; do
      terraform import -no-color "$APP_POLICY" "$id" > /dev/null
      terraform destroy -auto-approve -no-color -target="$APP_POLICY" | tail -1
      reset_state
    done <<< "$leftover"
    pass "Orphaned application access policy of the test app destroyed"
  else
    pass "No orphaned application access policy"
  fi
fi

# ---------------------------------------------------------------------------
# 7. Apply: create every test object and import the tenant-wide ones
# ---------------------------------------------------------------------------
step "terraform apply"
terraform apply -auto-approve -no-color | tail -1
pass "Apply OK"
if [[ -n "${DKIM:-}" ]]; then
  info "DKIM CNAME records to publish for $TF_VAR_test_dkim_domain:"
  info "  selector1._domainkey.$TF_VAR_test_dkim_domain -> $(state_string "$DKIM" selector1_cname)"
  info "  selector2._domainkey.$TF_VAR_test_dkim_domain -> $(state_string "$DKIM" selector2_cname)"
fi
expect_no_changes "Drift check after apply"

# ---------------------------------------------------------------------------
# 8. In-place update of every test object
# ---------------------------------------------------------------------------
step "Update test (phase=update)"
terraform apply -auto-approve -no-color -var phase=update | tail -1
pass "Update applied"
expect_no_changes "Drift check after update" -var phase=update

# ---------------------------------------------------------------------------
# 9. Writes to tenant-wide settings, then restore the original values
# ---------------------------------------------------------------------------
step "Tenant-wide settings write test"
hours="$(state_value msexchangeonline_transport_config.this reply_all_storm_block_duration_hours)"
write_and_restore msexchangeonline_transport_config.this reply_all_storm_block_duration_hours \
  reply_all_storm_block_duration_hours "$(( hours == 12 ? 11 : hours + 1 ))" "$hours"
# Only touch empty lists, so the restore puts back exactly what was there.
if [[ "$(state_value msexchangeonline_hosted_connection_filter_policy.this ip_block_list)" != *'"'* ]]; then
  write_and_restore msexchangeonline_hosted_connection_filter_policy.this ip_block_list \
    connection_filter_ip_block_list '["192.0.2.1"]' '[]'
else
  info "Skipped connection filter: the tenant already blocks IP addresses"
fi
if [[ "$(state_value msexchangeonline_external_in_outlook.this allow_list)" != *'"'* ]]; then
  write_and_restore msexchangeonline_external_in_outlook.this allow_list \
    external_tag_allow_list '["tf-ci-test.example"]' '[]'
else
  info "Skipped External tag: the tenant already has an allow list"
fi
expect_no_changes "Drift check after restore" -var phase=update

# ---------------------------------------------------------------------------
# 10. Import test: every object exists in Exchange but not in state
# ---------------------------------------------------------------------------
step "Import test: every resource"
for spec in "${CREATED[@]}"; do
  addr="${spec%%=*}"
  terraform state rm -no-color "$addr" > /dev/null
  terraform import -no-color -var phase=update "$addr" "${spec#*=}" > /dev/null || fail "Import of $addr failed"
done
if [[ -n "${APP_POLICY:-}" ]]; then
  id="$(state_string "$APP_POLICY" id)"
  terraform state rm -no-color "$APP_POLICY" > /dev/null
  terraform import -no-color -var phase=update "$APP_POLICY" "$id" > /dev/null || fail "Import of $APP_POLICY failed"
fi
if [[ -n "${DKIM:-}" ]]; then
  terraform state rm -no-color "$DKIM" > /dev/null
  terraform import -no-color -var phase=update "$DKIM" "$TF_VAR_test_dkim_domain" > /dev/null || fail "Import of $DKIM failed"
fi
pass "${#CREATED[@]} resources imported"
expect_no_changes "Drift check after import" -var phase=update

# ---------------------------------------------------------------------------
# 10b. terraform query: list every resource type, generate the configuration
#      of what exists (test objects and built-in ones) and check that it only
#      plans imports. Read-only: the configuration is never applied.
# ---------------------------------------------------------------------------
step "terraform query: generated configuration of the whole tenant"
QUERYDIR="$(mktemp -d)"
cat > "$QUERYDIR/main.tf" <<'HCL'
terraform {
  required_providers {
    msexchangeonline = { source = "workplace-as-code/msexchangeonline" }
  }
}
provider "msexchangeonline" {}
HCL
for doc in "$ROOT"/docs/resources/*.md; do
  name="$(basename "$doc" .md)"
  # Safe Links and Safe Attachments need Defender for Office 365.
  [[ "$name" == safe_* && -z "${EXCHANGE_TEST_DEFENDER:-}" ]] && continue
  printf 'list "msexchangeonline_%s" "all" {\n  provider = msexchangeonline\n}\n\n' "$name" >> "$QUERYDIR/all.tfquery.hcl"
done
(cd "$QUERYDIR" && terraform query -no-color -generate-config-out=generated.tf > query.log 2>&1) ||
  { cat "$QUERYDIR/query.log"; fail "terraform query failed"; }
listed=$(grep -c '^import {' "$QUERYDIR/generated.tf")
plan=$(cd "$QUERYDIR" && terraform plan -no-color 2>&1 | grep '^Plan:' || true)
[[ "$plan" == "Plan: $listed to import, 0 to add, 0 to change, 0 to destroy." ]] ||
  fail "The generated configuration does not match the tenant: ${plan:-plan failed} (see $QUERYDIR)"
rm -rf "$QUERYDIR"
pass "$listed objects listed; their generated configuration only plans imports"

# ---------------------------------------------------------------------------
# 11. Destroy the test objects, keep the tenant-wide ones
# ---------------------------------------------------------------------------
step "terraform destroy"
for addr in "${KEEP[@]}"; do
  terraform state rm -no-color "$addr" > /dev/null
done
# The import blocks would bring the tenant-wide objects back into the plan.
terraform destroy -auto-approve -no-color -var phase=update $(printf -- '-target=%s ' "${CREATED[@]%%=*}" ${APP_POLICY:+"$APP_POLICY"} ${DKIM:+"$DKIM"}) | tail -1
pass "Destroy OK"

# ---------------------------------------------------------------------------
# 12. Verify the test objects are really gone from Exchange
# ---------------------------------------------------------------------------
step "Verifying the test objects no longer exist"
reset_state
mapfile -t LEFT < <(import_orphans)
if [[ ${#LEFT[@]} -gt 0 ]]; then
  terraform destroy -auto-approve -no-color "${LEFT[@]}" > /dev/null
  fail "Still present after destroy (now removed): ${LEFT[*]#-target=}"
fi
reset_state
pass "All test objects were deleted"
if [[ -n "${DKIM:-}" ]]; then
  terraform import -no-color -var phase=update "$DKIM" "$TF_VAR_test_dkim_domain" > /dev/null || fail "Import of $DKIM after destroy failed"
  [[ "$(state_string "$DKIM" enabled)" == "false" ]] || fail "DKIM signing is still enabled for $TF_VAR_test_dkim_domain after destroy"
  reset_state
  pass "DKIM signing disabled for $TF_VAR_test_dkim_domain (the configuration itself cannot be deleted)"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   All integration tests passed ✓    ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
echo ""
echo "  Resources created, updated, imported and destroyed: ${#CREATED[@]}"
echo "  Imported read-only: organization_config"
echo "  Adopted on create, forgotten on destroy: accepted_domain"
echo "  Adopted with a minimal configuration, no change planned: remote_domain, hosted_content_filter_policy and mobile_device_mailbox_policy Default"
echo "  Written and restored: transport_config, hosted_connection_filter_policy, external_in_outlook"
[[ -z "${APP_POLICY:-}" ]] && echo "  Skipped:          application_access_policy (set TF_VAR_test_mail_enabled_group and TF_VAR_test_app_id)"
if [[ -n "${TF_VAR_test_mailbox:-}" ]]; then
  echo "  Data sources:     accepted_domain, accepted_domains, organization_config, mailbox"
else
  echo "  Data sources:     accepted_domain, accepted_domains, organization_config"
  echo "                    (mailbox skipped: set TF_VAR_test_mailbox to test it)"
fi
if [[ -n "${DKIM:-}" ]]; then
  echo "  Adopted, disabled on destroy: dkim_signing_config $TF_VAR_test_dkim_domain (signing enabled: ${TF_VAR_test_dkim_enabled:-false})"
else
  echo "  Skipped:          dkim_signing_config (set TF_VAR_test_dkim_domain)"
fi
if [[ -n "${EXCHANGE_TEST_DEFENDER:-}" ]]; then
  echo "  Defender:         safe_links_*, safe_attachment_*, anti-phishing Defender settings"
else
  echo "  Not tested:       safe_links_*, safe_attachment_* (set EXCHANGE_TEST_DEFENDER)"
fi
