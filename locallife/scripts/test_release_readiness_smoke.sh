#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
missing_fixture_output="$(mktemp)"
invalid_fixture_output="$(mktemp)"
trap 'rm -f "$missing_fixture_output" "$invalid_fixture_output"' EXIT

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

target_output="$(
  cd "$BACKEND_ROOT"
  PAYMENT_FACT_APPLICATION_FIXTURE_ID=101 \
    PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=202 \
    scripts/release_readiness_smoke.sh --dry-run
)"

assert_contains "$target_output" "go run ./cmd/release_readiness_smoke"
assert_contains "$target_output" "-include-config"
assert_contains "$target_output" "-include-redis"
assert_contains "$target_output" "-include-provider-clients"
assert_contains "$target_output" "-include-fixture-claimability"
assert_contains "$target_output" "-require-production"
assert_contains "$target_output" "-payment-fact-application-fixture-id 101"
assert_contains "$target_output" "-payment-domain-outbox-fixture-id 202"
assert_contains "$target_output" "-format text"

static_output="$(
  cd "$BACKEND_ROOT"
  scripts/release_readiness_smoke.sh --static --dry-run
)"

assert_contains "$static_output" "go run ./cmd/release_readiness_smoke"
assert_contains "$static_output" "-format text"
if [[ "$static_output" == *"-include-fixture-claimability"* ]]; then
  echo "static dry-run must not include fixture claimability" >&2
  echo "$static_output" >&2
  exit 1
fi

if (
  cd "$BACKEND_ROOT"
  unset PAYMENT_FACT_APPLICATION_FIXTURE_ID
  unset PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID
  scripts/release_readiness_smoke.sh --dry-run
) >"$missing_fixture_output" 2>&1; then
  echo "target dry-run without fixture IDs unexpectedly succeeded" >&2
  cat "$missing_fixture_output" >&2
  exit 1
fi

missing_output="$(cat "$missing_fixture_output")"
assert_contains "$missing_output" "PAYMENT_FACT_APPLICATION_FIXTURE_ID is required"
assert_contains "$missing_output" "PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID is required"

if (
  cd "$BACKEND_ROOT"
  PAYMENT_FACT_APPLICATION_FIXTURE_ID=abc \
    PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=0 \
    scripts/release_readiness_smoke.sh --dry-run
) >"$invalid_fixture_output" 2>&1; then
  echo "target dry-run with invalid fixture IDs unexpectedly succeeded" >&2
  cat "$invalid_fixture_output" >&2
  exit 1
fi

invalid_output="$(cat "$invalid_fixture_output")"
assert_contains "$invalid_output" "PAYMENT_FACT_APPLICATION_FIXTURE_ID must be a positive integer"
assert_contains "$invalid_output" "PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID must be a positive integer"

echo "release readiness smoke wrapper contract passed"
