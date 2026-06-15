#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
missing_ack_output="$(mktemp)"
malformed_context_output="$(mktemp)"
missing_withdrawal_output="$(mktemp)"
missing_manual_output="$(mktemp)"
unsupported_evidence_kind_output="$(mktemp)"
unknown_capability_output="$(mktemp)"
wrong_callback_endpoint_output="$(mktemp)"
callback_endpoint_prefix_output="$(mktemp)"
query_endpoint_callback_output="$(mktemp)"
wrong_funds_action_endpoint_output="$(mktemp)"
missing_release_target_output="$(mktemp)"
trap 'rm -f "$missing_ack_output" "$malformed_context_output" "$missing_withdrawal_output" "$missing_manual_output" "$unsupported_evidence_kind_output" "$unknown_capability_output" "$wrong_callback_endpoint_output" "$callback_endpoint_prefix_output" "$query_endpoint_callback_output" "$wrong_funds_action_endpoint_output" "$missing_release_target_output"' EXIT

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
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
)"

assert_contains "$payment_output" "make check-baofu-contract"
assert_contains "$payment_output" "scripts/release_readiness_smoke.sh --static --format text"
assert_contains "$payment_output" "scripts/check_release_readiness_target_evidence.sh ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md"
assert_contains "$payment_output" "go run ./cmd/baofu_payment_evidence"
assert_contains "$payment_output" "-fact-id 11"
assert_contains "$payment_output" "-application-id 21"
assert_contains "$payment_output" "-payment-order-id 31"
assert_contains "$payment_output" "-profit-sharing-order-id 61"
assert_contains "$payment_output" "-ledger-row"
assert_contains "$payment_output" "-ledger-ack OK"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind callback \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/payment \
    --ledger-ack OK \
    --ledger-commit b6507961 \
    --ledger-notes "production payment callback without target smoke evidence" \
    --dry-run
) >"$missing_release_target_output" 2>&1; then
  echo "production provider evidence without release target evidence unexpectedly succeeded" >&2
  cat "$missing_release_target_output" >&2
  exit 1
fi

assert_contains "$(cat "$missing_release_target_output")" "release target evidence is required for production ledger evidence"

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
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
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
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind callback \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/refund \
    --ledger-ack OK \
    --ledger-commit b6507961 \
    --ledger-notes "payment callback endpoint mismatch" \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
) >"$wrong_callback_endpoint_output" 2>&1; then
  echo "payment callback evidence with refund endpoint unexpectedly succeeded" >&2
  cat "$wrong_callback_endpoint_output" >&2
  exit 1
fi

assert_contains "$(cat "$wrong_callback_endpoint_output")" "callback endpoint does not match payment evidence"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind callback \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/payment-extra \
    --ledger-ack OK \
    --ledger-commit b6507961 \
    --ledger-notes "payment callback endpoint prefix must not count" \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
) >"$callback_endpoint_prefix_output" 2>&1; then
  echo "payment callback evidence with prefixed endpoint unexpectedly succeeded" >&2
  cat "$callback_endpoint_prefix_output" >&2
  exit 1
fi

assert_contains "$(cat "$callback_endpoint_prefix_output")" "callback endpoint does not match payment evidence"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability refund \
    --fact-id 41 \
    --application-id 51 \
    --refund-order-id 71 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind callback \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint "https://mch-juhe.baofoo.com/api refund_query" \
    --ledger-ack OK \
    --ledger-commit 7c325e4d \
    --ledger-notes "refund query endpoint must not be marked callback evidence" \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
) >"$query_endpoint_callback_output" 2>&1; then
  echo "refund callback evidence with query endpoint unexpectedly succeeded" >&2
  cat "$query_endpoint_callback_output" >&2
  exit 1
fi

assert_contains "$(cat "$query_endpoint_callback_output")" "callback endpoint does not match refund evidence"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind query \
    --ledger-date 06/15/2026 \
    --ledger-env prod \
    --ledger-endpoint "https://mch-juhe.baofoo.com/api order_query" \
    --ledger-commit not-a-sha \
    --ledger-notes "malformed context" \
    --dry-run
) >"$malformed_context_output" 2>&1; then
  echo "ledger row with malformed runtime context unexpectedly succeeded" >&2
  cat "$malformed_context_output" >&2
  exit 1
fi

assert_contains "$(cat "$malformed_context_output")" "ledger date must use yyyy-mm-dd"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind query \
    --ledger-date 2026-06-15 \
    --ledger-env prod \
    --ledger-endpoint "https://mch-juhe.baofoo.com/api order_query" \
    --ledger-commit b6507961 \
    --ledger-notes "malformed env" \
    --dry-run
) >"$malformed_context_output" 2>&1; then
  echo "ledger row with malformed environment unexpectedly succeeded" >&2
  cat "$malformed_context_output" >&2
  exit 1
fi

assert_contains "$(cat "$malformed_context_output")" "ledger env is not supported"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind query \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint "https://mch-juhe.baofoo.com/api order_query" \
    --ledger-commit not-a-sha \
    --ledger-notes "malformed commit" \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
) >"$malformed_context_output" 2>&1; then
  echo "ledger row with malformed commit unexpectedly succeeded" >&2
  cat "$malformed_context_output" >&2
  exit 1
fi

assert_contains "$(cat "$malformed_context_output")" "ledger commit must be a git SHA"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability payment \
    --fact-id 11 \
    --application-id 21 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind manual-reconciliation \
    --ledger-date 2026-06-15 \
    --ledger-env provider-real-transaction-env \
    --ledger-endpoint "https://mch-juhe.baofoo.com/api order_query" \
    --ledger-commit b6507961 \
    --ledger-notes "payment query must not be marked manual recovery" \
    --dry-run
) >"$unsupported_evidence_kind_output" 2>&1; then
  echo "non-withdrawal manual-reconciliation evidence unexpectedly succeeded" >&2
  cat "$unsupported_evidence_kind_output" >&2
  exit 1
fi

assert_contains "$(cat "$unsupported_evidence_kind_output")" "manual-reconciliation evidence is only valid for withdrawal"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability refund \
    --fact-id 41 \
    --application-id 51 \
    --refund-order-id 71 \
    --payment-order-id 31 \
    --ledger-row \
    --evidence-kind funds-action \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/refund \
    --ledger-ack OK \
    --ledger-commit 7c325e4d \
    --ledger-notes "refund callback must not be marked withdrawal funds action" \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
) >"$unsupported_evidence_kind_output" 2>&1; then
  echo "non-withdrawal funds-action evidence unexpectedly succeeded" >&2
  cat "$unsupported_evidence_kind_output" >&2
  exit 1
fi

assert_contains "$(cat "$unsupported_evidence_kind_output")" "funds-action evidence is only valid for withdrawal"

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
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/withdraw \
    --ledger-ack OK \
    --ledger-commit 8f0a3b1c \
    --ledger-notes "missing withdrawal approval context" \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
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
  scripts/baofu_provider_evidence_gate.sh \
    --capability withdrawal \
    --fact-id 301 \
    --withdrawal-order-id 401 \
    --ledger-row \
    --evidence-kind funds-action \
    --ledger-date 2026-06-15 \
    --ledger-env production \
    --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/payment \
    --ledger-ack OK \
    --ledger-commit 8f0a3b1c \
    --ledger-notes "withdrawal funds-action endpoint mismatch" \
    --withdrawal-approver finance-ticket-1 \
    --withdrawal-amount-bound 100 \
    --withdrawal-monitoring-owner finance-oncall \
    --release-target-evidence ../artifacts/production-risk-audit/flows/release-readiness-target-evidence-2026-06-15.md \
    --dry-run
) >"$wrong_funds_action_endpoint_output" 2>&1; then
  echo "withdrawal funds-action callback evidence with payment endpoint unexpectedly succeeded" >&2
  cat "$wrong_funds_action_endpoint_output" >&2
  exit 1
fi

assert_contains "$(cat "$wrong_funds_action_endpoint_output")" "callback endpoint does not match withdrawal evidence"

if (
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability withdrawal \
    --fact-id 302 \
    --withdrawal-order-id 402 \
    --ledger-row \
    --evidence-kind manual-reconciliation \
    --ledger-date 2026-06-15 \
    --ledger-env provider-real-transaction-env \
    --ledger-endpoint "https://vgw.baofoo.com/union-gw/api/T-1001-013-15/transReq.do" \
    --ledger-commit 8f0a3b1d \
    --ledger-notes "missing manual recovery context" \
    --dry-run
) >"$missing_manual_output" 2>&1; then
  echo "withdrawal manual-reconciliation without recovery context unexpectedly succeeded" >&2
  cat "$missing_manual_output" >&2
  exit 1
fi

assert_contains "$(cat "$missing_manual_output")" "manual recovery owner is required"
assert_contains "$(cat "$missing_manual_output")" "provider query result is required"

withdrawal_manual_output="$(
  cd "$BACKEND_ROOT"
  scripts/baofu_provider_evidence_gate.sh \
    --capability withdrawal \
    --fact-id 302 \
    --withdrawal-order-id 402 \
    --command-id 602 \
    --ledger-row \
    --evidence-kind manual-reconciliation \
    --ledger-date 2026-06-15 \
    --ledger-env provider-real-transaction-env \
    --ledger-endpoint "https://vgw.baofoo.com/union-gw/api/T-1001-013-15/transReq.do" \
    --ledger-commit 8f0a3b1d \
    --ledger-notes "controlled withdrawal manual recovery" \
    --manual-recovery-owner finance-oncall \
    --provider-query-result "query returned state=1 with masked baofu withdraw no" \
    --dry-run
)"

assert_contains "$withdrawal_manual_output" "go run ./cmd/baofu_withdrawal_evidence"
assert_contains "$withdrawal_manual_output" "-command-id 602"
assert_contains "$withdrawal_manual_output" "manual_recovery_owner=finance-oncall"
assert_contains "$withdrawal_manual_output" "provider_query_result=query\\ returned\\ state=1\\ with\\ masked\\ baofu\\ withdraw\\ no"
if [[ "$withdrawal_manual_output" == *"-ledger-ack"* ]]; then
  echo "manual-reconciliation evidence dry-run must not invent a callback ACK" >&2
  echo "$withdrawal_manual_output" >&2
  exit 1
fi

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
