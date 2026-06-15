#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
missing_ack_output="$(mktemp)"
missing_withdrawal_output="$(mktemp)"
unknown_capability_output="$(mktemp)"
trap 'rm -f "$missing_ack_output" "$missing_withdrawal_output" "$unknown_capability_output"' EXIT

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

payment_output="$(
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --profit-sharing-order-id 61 \
    --ledger-row \
    --evidence-kind callback \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/payment \
    --ledger-ack OK \
    --ledger-commit b6507961 \
    --ledger-notes "controlled payment callback" \
    --dry-run
)"

assert_contains "$payment_output" "make check-baofu-contract"
assert_contains "$payment_output" "scripts/release_readiness_smoke.sh --static --format text"
assert_contains "$payment_output" "go run ./cmd/baofu_payment_evidence"
assert_contains "$payment_output" "-fact-id 11"
assert_contains "$payment_output" "-application-id 21"
assert_contains "$payment_output" "-payment-order-id 31"
assert_contains "$payment_output" "-profit-sharing-order-id 61"
assert_contains "$payment_output" "-ledger-row"
assert_contains "$payment_output" "-ledger-ack OK"

refund_query_output="$(
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability refund \
    --fact-id 41 \
    --application-id 51 \
    --refund-order-id 71 \
    --payment-order-id 31 \
    --command-id 601 \
    --ledger-row \
    --evidence-kind query \
    --ledger-date 2026-06-15 \
    --ledger-env provider-real-transaction-env \
    --ledger-endpoint "https://mch-juhe.baofoo.com/api refund_query" \
    --ledger-commit 7c325e4d \
    --ledger-notes "controlled refund query recovery" \
    --dry-run
)"

assert_contains "$refund_query_output" "go run ./cmd/baofu_refund_evidence"
assert_contains "$refund_query_output" "-refund-order-id 71"
assert_contains "$refund_query_output" "-payment-order-id 31"
assert_contains "$refund_query_output" "-command-id 601"
if [[ "$refund_query_output" == *"-ledger-ack"* ]]; then
  echo "query evidence dry-run must not invent a callback ACK" >&2
  echo "$refund_query_output" >&2
  exit 1
fi

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability profit-sharing \
    --fact-id 101 \
    --application-id 201 \
    --profit-sharing-order-id 61 \
    --ledger-row \
    --evidence-kind callback \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/profit-sharing \
    --ledger-commit 2d6ebbdf \
    --ledger-notes "missing ack" \
    --dry-run
) >"$missing_ack_output" 2>&1; then
  echo "callback ledger-row without ACK unexpectedly succeeded" >&2
  cat "$missing_ack_output" >&2
  exit 1
fi

assert_contains "$(cat "$missing_ack_output")" "ledger ack is required for callback evidence"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability withdrawal \
    --fact-id 301 \
    --withdrawal-order-id 401 \
    --ledger-row \
    --evidence-kind funds-action \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/withdrawal \
    --ledger-ack OK \
    --ledger-commit 8f0a3b1c \
    --ledger-notes "missing withdrawal approval context" \
    --dry-run
) >"$missing_withdrawal_output" 2>&1; then
  echo "withdrawal ledger-row without funds approval context unexpectedly succeeded" >&2
  cat "$missing_withdrawal_output" >&2
  exit 1
fi

assert_contains "$(cat "$missing_withdrawal_output")" "withdrawal approver is required"
assert_contains "$(cat "$missing_withdrawal_output")" "withdrawal amount bound is required"
assert_contains "$(cat "$missing_withdrawal_output")" "withdrawal monitoring owner is required"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh --capability unknown --dry-run
) >"$unknown_capability_output" 2>&1; then
  echo "unknown capability unexpectedly succeeded" >&2
  cat "$unknown_capability_output" >&2
  exit 1
fi

assert_contains "$(cat "$unknown_capability_output")" "unknown capability: unknown"

echo "baofu provider evidence gate wrapper contract passed"
