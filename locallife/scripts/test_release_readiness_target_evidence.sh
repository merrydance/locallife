#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHECKER="$BACKEND_ROOT/scripts/check_release_readiness_target_evidence.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "expected output to contain: $needle" >&2
    echo "actual output:" >&2
    echo "$haystack" >&2
    exit 1
  fi
}

write_valid_evidence() {
  local path="$1"
  cat >"$path" <<'EVIDENCE'
# Release Readiness Target Evidence

Date: 2026-06-15
Risk theme: release configuration
Risk class: G3 - scheduler and worker convergence
Status: target evidence recorded

## Target Environment

Target environment: production
Backend commit: 2caaeaa7
Release operator: release-owner

## Release Smoke Command

Smoke command: ENVIRONMENT=production PAYMENT_FACT_APPLICATION_FIXTURE_ID=101 PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=202 PATH="/usr/local/go/bin:$PATH" scripts/release_readiness_smoke.sh --target --format text
Smoke output artifact: artifacts/private/release-readiness-target-2026-06-15.txt
Target smoke status: pass

## Fixture Rows

PAYMENT_FACT_APPLICATION_FIXTURE_ID: 101
PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID: 202
Fixture source: disposable release-smoke rows prepared before the run

## Readiness Results

config:production_allowed_origins: pass
config:production_redis_address: pass
config:production_data_encryption_key: pass
config:production_payment_runtime: pass
redis:connection: pass
asynq:queue:critical: pass
asynq:queue:default: pass
provider:baofu:root: pass
provider:baofu:aggregate: pass
provider:baofu:account: pass
provider:baofu:merchant_report: pass
fixture:payment_fact_application: pass
fixture:payment_domain_outbox: pass
scheduler:dine-in-checkout-recovery: pass
worker:payment:process_fact_application: pass
worker:payment:process_domain_outbox: pass

## Alert Evidence

Dine-in recovery alert evidence: artifacts/production-risk-audit/flows/dine-in-checkout-recovery-alert-evidence-2026-06-15.md

## Result

Verdict: pass
EVIDENCE
}

valid_evidence="$tmp_dir/release-readiness-target-evidence.md"
write_valid_evidence "$valid_evidence"

valid_output="$("$CHECKER" "$valid_evidence")"
assert_contains "$valid_output" "release readiness target evidence is complete"

template_evidence="$tmp_dir/template.md"
cp "$valid_evidence" "$template_evidence"
{
  printf '\nStatus: template only; no target run recorded\n'
  printf 'Verdict: pass\n'
} >>"$template_evidence"
if "$CHECKER" "$template_evidence" >"$tmp_dir/template.out" 2>&1; then
  echo "template evidence unexpectedly passed" >&2
  exit 1
fi
assert_contains "$(cat "$tmp_dir/template.out")" "template evidence cannot be used"

static_evidence="$tmp_dir/static.md"
sed 's/scripts\/release_readiness_smoke.sh --target --format text/scripts\/release_readiness_smoke.sh --static --format text/' "$valid_evidence" >"$static_evidence"
if "$CHECKER" "$static_evidence" >"$tmp_dir/static.out" 2>&1; then
  echo "static smoke evidence unexpectedly passed" >&2
  exit 1
fi
assert_contains "$(cat "$tmp_dir/static.out")" "Smoke command must run --target"

dry_run_evidence="$tmp_dir/dry-run.md"
sed 's/--target --format text/--target --format text --dry-run/' "$valid_evidence" >"$dry_run_evidence"
if "$CHECKER" "$dry_run_evidence" >"$tmp_dir/dry-run.out" 2>&1; then
  echo "dry-run smoke evidence unexpectedly passed" >&2
  exit 1
fi
assert_contains "$(cat "$tmp_dir/dry-run.out")" "Smoke command must not be dry-run"

fixture_evidence="$tmp_dir/bad-fixture.md"
sed 's/PAYMENT_FACT_APPLICATION_FIXTURE_ID: 101/PAYMENT_FACT_APPLICATION_FIXTURE_ID: 0/' "$valid_evidence" >"$fixture_evidence"
if "$CHECKER" "$fixture_evidence" >"$tmp_dir/fixture.out" 2>&1; then
  echo "non-positive fixture evidence unexpectedly passed" >&2
  exit 1
fi
assert_contains "$(cat "$tmp_dir/fixture.out")" "PAYMENT_FACT_APPLICATION_FIXTURE_ID must be a positive integer"

missing_result_evidence="$tmp_dir/missing-result.md"
sed 's/redis:connection: pass/redis:connection: fail/' "$valid_evidence" >"$missing_result_evidence"
if "$CHECKER" "$missing_result_evidence" >"$tmp_dir/missing-result.out" 2>&1; then
  echo "failed readiness row evidence unexpectedly passed" >&2
  exit 1
fi
assert_contains "$(cat "$tmp_dir/missing-result.out")" "missing required pass row: redis:connection: pass"

verdict_evidence="$tmp_dir/fail-verdict.md"
sed 's/Verdict: pass/Verdict: fail/' "$valid_evidence" >"$verdict_evidence"
if "$CHECKER" "$verdict_evidence" >"$tmp_dir/verdict.out" 2>&1; then
  echo "failed verdict evidence unexpectedly passed" >&2
  exit 1
fi
assert_contains "$(cat "$tmp_dir/verdict.out")" "Verdict must be pass"

echo "release readiness target evidence contract passed"
