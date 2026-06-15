#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/check_release_readiness_target_evidence.sh <evidence.md>" >&2
  exit 2
fi

evidence_file="$1"
if [[ ! -f "$evidence_file" ]]; then
  echo "evidence file not found: $evidence_file" >&2
  exit 2
fi

content="$(cat "$evidence_file")"

fail() {
  echo "$1" >&2
  exit 1
}

require_contains() {
  local needle="$1"
  local message="${2:-missing required evidence: $needle}"
  if [[ "$content" != *"$needle"* ]]; then
    fail "$message"
  fi
}

require_section() {
  require_contains "$1" "missing required section: $1"
}

require_positive_field() {
  local field="$1"
  local value
  value="$(printf '%s\n' "$content" | awk -F':' -v key="$field" '$1 == key { sub(/^[[:space:]]+/, "", $2); sub(/[[:space:]]+$/, "", $2); print $2; exit }')"
  if [[ ! "$value" =~ ^[1-9][0-9]*$ ]]; then
    fail "$field must be a positive integer"
  fi
}

for section in \
  "## Target Environment" \
  "## Release Smoke Command" \
  "## Fixture Rows" \
  "## Readiness Results" \
  "## Alert Evidence" \
  "## Result"; do
  require_section "$section"
done

for needle in \
  "Target environment:" \
  "Backend commit:" \
  "Release operator:" \
  "Smoke command:" \
  "Smoke output artifact:" \
  "Target smoke status:" \
  "PAYMENT_FACT_APPLICATION_FIXTURE_ID:" \
  "PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID:" \
  "Fixture source:" \
  "Dine-in recovery alert evidence:" \
  "Verdict:"; do
  require_contains "$needle"
done

if printf '%s\n' "$content" | grep -Eiq 'Status:[[:space:]]*template only|no target run recorded|Keep this template|<fill|<todo|<required|TBD|TODO'; then
  fail "template evidence cannot be used as release evidence"
fi

if ! printf '%s\n' "$content" | grep -Eq 'Target environment:[[:space:]]*production\b'; then
  fail "Target environment must be production for release readiness target evidence"
fi
if ! printf '%s\n' "$content" | grep -Eq 'Target smoke status:[[:space:]]*pass\b'; then
  fail "Target smoke status must be pass"
fi
if ! printf '%s\n' "$content" | grep -Eq 'Verdict:[[:space:]]*pass\b'; then
  fail "Verdict must be pass"
fi
if ! printf '%s\n' "$content" | grep -Eq 'Smoke command:.*scripts/release_readiness_smoke\.sh'; then
  fail "Smoke command must use scripts/release_readiness_smoke.sh"
fi
if ! printf '%s\n' "$content" | grep -Eq 'Smoke command:.*--target\b'; then
  fail "Smoke command must run --target"
fi
if printf '%s\n' "$content" | grep -Eq 'Smoke command:.*--static\b'; then
  fail "Smoke command must run --target, not --static"
fi
if printf '%s\n' "$content" | grep -Eq 'Smoke command:.*--dry-run\b'; then
  fail "Smoke command must not be dry-run"
fi
if ! printf '%s\n' "$content" | grep -Eq 'Smoke command:.*PAYMENT_FACT_APPLICATION_FIXTURE_ID=[1-9][0-9]*'; then
  fail "Smoke command must include PAYMENT_FACT_APPLICATION_FIXTURE_ID"
fi
if ! printf '%s\n' "$content" | grep -Eq 'Smoke command:.*PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=[1-9][0-9]*'; then
  fail "Smoke command must include PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID"
fi

require_positive_field "PAYMENT_FACT_APPLICATION_FIXTURE_ID"
require_positive_field "PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID"

required_pass_rows=(
  "config:production_allowed_origins: pass"
  "config:production_redis_address: pass"
  "config:production_data_encryption_key: pass"
  "config:production_payment_runtime: pass"
  "redis:connection: pass"
  "asynq:queue:critical: pass"
  "asynq:queue:default: pass"
  "provider:baofu:root: pass"
  "provider:baofu:aggregate: pass"
  "provider:baofu:account: pass"
  "provider:baofu:merchant_report: pass"
  "fixture:payment_fact_application: pass"
  "fixture:payment_domain_outbox: pass"
  "scheduler:dine-in-checkout-recovery: pass"
  "worker:payment:process_fact_application: pass"
  "worker:payment:process_domain_outbox: pass"
)

for row in "${required_pass_rows[@]}"; do
  require_contains "$row" "missing required pass row: $row"
done

echo "release readiness target evidence is complete: $evidence_file"
