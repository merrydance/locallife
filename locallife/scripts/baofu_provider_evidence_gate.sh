#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

capability=""
evidence_kind="query"
dry_run=0
ledger_row=0

fact_id=""
application_id=""
payment_order_id=""
profit_sharing_order_id=""
refund_order_id=""
withdrawal_order_id=""
command_id=""

ledger_date=""
ledger_env=""
ledger_endpoint=""
ledger_ack=""
ledger_commit=""
ledger_notes=""

withdrawal_approver=""
withdrawal_amount_bound=""
withdrawal_monitoring_owner=""

usage() {
  cat <<'USAGE'
Usage:
  scripts/baofu_provider_evidence_gate.sh --capability <payment|profit-sharing|refund|withdrawal> [options]

Purpose:
  Run the release-safe Baofu provider evidence preflight and then execute the
  matching read-only local evidence collector. This script never calls Baofoo
  directly and never executes funds actions.

Common options:
  --fact-id <id>
  --application-id <id>
  --payment-order-id <id>
  --profit-sharing-order-id <id>
  --refund-order-id <id>
  --withdrawal-order-id <id>
  --command-id <id>
  --ledger-row
  --evidence-kind <callback|query|manual-reconciliation|funds-action>
  --ledger-date <yyyy-mm-dd>
  --ledger-env <sandbox|production|provider-real-transaction-env>
  --ledger-endpoint <callback-url-or-query-endpoint>
  --ledger-ack <OK>                      Required for callback evidence; allowed for withdrawal funds-action callbacks.
  --ledger-commit <sha>
  --ledger-notes <controlled-run-notes>
  --withdrawal-approver <name-or-ticket> Required for withdrawal funds-action evidence.
  --withdrawal-amount-bound <amount>     Required for withdrawal funds-action evidence.
  --withdrawal-monitoring-owner <owner>  Required for withdrawal funds-action evidence.
  --dry-run                              Print commands without executing them.

Examples:
  scripts/baofu_provider_evidence_gate.sh --capability payment --fact-id 11 --application-id 21 --payment-order-id 31
  scripts/baofu_provider_evidence_gate.sh --capability refund --fact-id 41 --application-id 51 --refund-order-id 71 --payment-order-id 31 --ledger-row --evidence-kind query ...
USAGE
}

require_value() {
  local flag="$1"
  if [[ $# -lt 2 || -z "${2:-}" ]]; then
    echo "$flag requires a value" >&2
    exit 2
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --capability)
      require_value "$1" "${2:-}"
      capability="$2"
      shift 2
      ;;
    --evidence-kind)
      require_value "$1" "${2:-}"
      evidence_kind="$2"
      shift 2
      ;;
    --fact-id)
      require_value "$1" "${2:-}"
      fact_id="$2"
      shift 2
      ;;
    --application-id)
      require_value "$1" "${2:-}"
      application_id="$2"
      shift 2
      ;;
    --payment-order-id)
      require_value "$1" "${2:-}"
      payment_order_id="$2"
      shift 2
      ;;
    --profit-sharing-order-id)
      require_value "$1" "${2:-}"
      profit_sharing_order_id="$2"
      shift 2
      ;;
    --refund-order-id)
      require_value "$1" "${2:-}"
      refund_order_id="$2"
      shift 2
      ;;
    --withdrawal-order-id)
      require_value "$1" "${2:-}"
      withdrawal_order_id="$2"
      shift 2
      ;;
    --command-id)
      require_value "$1" "${2:-}"
      command_id="$2"
      shift 2
      ;;
    --ledger-row)
      ledger_row=1
      shift
      ;;
    --ledger-date)
      require_value "$1" "${2:-}"
      ledger_date="$2"
      shift 2
      ;;
    --ledger-env)
      require_value "$1" "${2:-}"
      ledger_env="$2"
      shift 2
      ;;
    --ledger-endpoint)
      require_value "$1" "${2:-}"
      ledger_endpoint="$2"
      shift 2
      ;;
    --ledger-ack)
      require_value "$1" "${2:-}"
      ledger_ack="$2"
      shift 2
      ;;
    --ledger-commit)
      require_value "$1" "${2:-}"
      ledger_commit="$2"
      shift 2
      ;;
    --ledger-notes)
      require_value "$1" "${2:-}"
      ledger_notes="$2"
      shift 2
      ;;
    --withdrawal-approver)
      require_value "$1" "${2:-}"
      withdrawal_approver="$2"
      shift 2
      ;;
    --withdrawal-amount-bound)
      require_value "$1" "${2:-}"
      withdrawal_amount_bound="$2"
      shift 2
      ;;
    --withdrawal-monitoring-owner)
      require_value "$1" "${2:-}"
      withdrawal_monitoring_owner="$2"
      shift 2
      ;;
    --dry-run)
      dry_run=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

append_arg() {
  local -n target="$1"
  local flag="$2"
  local value="${3:-}"
  if [[ -n "$value" ]]; then
    target+=("$flag" "$value")
  fi
}

append_required_arg() {
  local -n target="$1"
  local flag="$2"
  local value="${3:-}"
  local message="$4"
  if [[ -z "$value" ]]; then
    echo "$message" >&2
    exit 2
  fi
  target+=("$flag" "$value")
}

if [[ -z "$capability" ]]; then
  echo "capability is required" >&2
  usage >&2
  exit 2
fi

case "$evidence_kind" in
  callback|query|manual-reconciliation|funds-action)
    ;;
  *)
    echo "unknown evidence kind: $evidence_kind" >&2
    exit 2
    ;;
esac

if [[ "$ledger_row" -eq 1 ]]; then
  if [[ -z "$ledger_date" ]]; then
    echo "ledger date is required with --ledger-row" >&2
    exit 2
  fi
  if [[ -z "$ledger_env" ]]; then
    echo "ledger env is required with --ledger-row" >&2
    exit 2
  fi
  if [[ -z "$ledger_endpoint" ]]; then
    echo "ledger endpoint is required with --ledger-row" >&2
    exit 2
  fi
  if [[ -z "$ledger_commit" ]]; then
    echo "ledger commit is required with --ledger-row" >&2
    exit 2
  fi
  if [[ -z "$ledger_notes" ]]; then
    echo "ledger notes are required with --ledger-row" >&2
    exit 2
  fi
  if [[ "$evidence_kind" == "callback" && -z "$ledger_ack" ]]; then
    echo "ledger ack is required for callback evidence" >&2
    exit 2
  fi
  if [[ "$evidence_kind" != "callback" && "$evidence_kind" != "funds-action" && -n "$ledger_ack" ]]; then
    echo "ledger ack is only valid for callback or funds-action callback evidence" >&2
    exit 2
  fi
fi

if [[ "$capability" == "withdrawal" && "$evidence_kind" == "funds-action" ]]; then
  missing_withdrawal=0
  if [[ -z "$withdrawal_approver" ]]; then
    echo "withdrawal approver is required for funds-action evidence" >&2
    missing_withdrawal=1
  fi
  if [[ -z "$withdrawal_amount_bound" ]]; then
    echo "withdrawal amount bound is required for funds-action evidence" >&2
    missing_withdrawal=1
  fi
  if [[ -z "$withdrawal_monitoring_owner" ]]; then
    echo "withdrawal monitoring owner is required for funds-action evidence" >&2
    missing_withdrawal=1
  fi
  if [[ "$missing_withdrawal" -ne 0 ]]; then
    exit 2
  fi
  ledger_notes="${ledger_notes}; withdrawal_approver=${withdrawal_approver}; withdrawal_amount_bound=${withdrawal_amount_bound}; withdrawal_monitoring_owner=${withdrawal_monitoring_owner}"
fi

preflight_contract=(make check-baofu-contract)
preflight_release=(scripts/release_readiness_smoke.sh --static --format text)

collector=()
case "$capability" in
  payment)
    collector=(go run ./cmd/baofu_payment_evidence)
    append_required_arg collector "-fact-id" "$fact_id" "fact-id is required for payment evidence"
    append_required_arg collector "-application-id" "$application_id" "application-id is required for payment evidence"
    append_required_arg collector "-payment-order-id" "$payment_order_id" "payment-order-id is required for payment evidence"
    append_arg collector "-profit-sharing-order-id" "$profit_sharing_order_id"
    ;;
  profit-sharing)
    collector=(go run ./cmd/baofu_profit_sharing_evidence)
    append_required_arg collector "-fact-id" "$fact_id" "fact-id is required for profit-sharing evidence"
    append_required_arg collector "-application-id" "$application_id" "application-id is required for profit-sharing evidence"
    append_required_arg collector "-profit-sharing-order-id" "$profit_sharing_order_id" "profit-sharing-order-id is required for profit-sharing evidence"
    append_arg collector "-command-id" "$command_id"
    ;;
  refund)
    collector=(go run ./cmd/baofu_refund_evidence)
    append_required_arg collector "-fact-id" "$fact_id" "fact-id is required for refund evidence"
    append_required_arg collector "-application-id" "$application_id" "application-id is required for refund evidence"
    append_required_arg collector "-refund-order-id" "$refund_order_id" "refund-order-id is required for refund evidence"
    append_required_arg collector "-payment-order-id" "$payment_order_id" "payment-order-id is required for refund evidence"
    append_arg collector "-command-id" "$command_id"
    ;;
  withdrawal)
    collector=(go run ./cmd/baofu_withdrawal_evidence)
    append_required_arg collector "-fact-id" "$fact_id" "fact-id is required for withdrawal evidence"
    append_required_arg collector "-withdrawal-order-id" "$withdrawal_order_id" "withdrawal-order-id is required for withdrawal evidence"
    append_arg collector "-command-id" "$command_id"
    ;;
  *)
    echo "unknown capability: $capability" >&2
    usage >&2
    exit 2
    ;;
esac

if [[ "$ledger_row" -eq 1 ]]; then
  collector+=("-ledger-row")
  append_required_arg collector "-ledger-date" "$ledger_date" "ledger date is required with --ledger-row"
  append_required_arg collector "-ledger-env" "$ledger_env" "ledger env is required with --ledger-row"
  append_required_arg collector "-ledger-endpoint" "$ledger_endpoint" "ledger endpoint is required with --ledger-row"
  append_required_arg collector "-ledger-commit" "$ledger_commit" "ledger commit is required with --ledger-row"
  append_required_arg collector "-ledger-notes" "$ledger_notes" "ledger notes are required with --ledger-row"
  append_arg collector "-ledger-ack" "$ledger_ack"
fi

print_command() {
  local -n cmd_ref="$1"
  printf '%q ' "${cmd_ref[@]}"
  printf '\n'
}

cd "$BACKEND_ROOT"

if [[ "$dry_run" -eq 1 ]]; then
  print_command preflight_contract
  print_command preflight_release
  print_command collector
  exit 0
fi

"${preflight_contract[@]}"
"${preflight_release[@]}"
exec "${collector[@]}"
