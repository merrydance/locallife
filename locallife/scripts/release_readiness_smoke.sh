#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

mode="target"
format="text"
dry_run=0

usage() {
  cat <<'USAGE'
Usage:
  scripts/release_readiness_smoke.sh [--target|--static] [--format text|json] [--dry-run]

Modes:
  --target  Run the release-environment gate: config, Redis/Asynq, Baofu client construction, and rollback-only fixture claimability.
  --static  Run the side-effect-free source registration report only.

Target mode requires:
  PAYMENT_FACT_APPLICATION_FIXTURE_ID
  PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID

Fixture IDs must point at disposable release-smoke rows prepared by the release operator.
USAGE
}

validate_positive_integer() {
  local name="$1"
  local value="$2"
  if [[ ! "$value" =~ ^[1-9][0-9]*$ ]]; then
    echo "$name must be a positive integer" >&2
    return 1
  fi
  return 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      mode="target"
      shift
      ;;
    --static)
      mode="static"
      shift
      ;;
    --format)
      if [[ $# -lt 2 ]]; then
        echo "--format requires text or json" >&2
        exit 2
      fi
      format="$2"
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

if [[ "$format" != "text" && "$format" != "json" ]]; then
  echo "--format must be text or json" >&2
  exit 2
fi

cmd=(go run ./cmd/release_readiness_smoke -format "$format")

if [[ "$mode" == "target" ]]; then
  missing=0
  if [[ -z "${PAYMENT_FACT_APPLICATION_FIXTURE_ID:-}" ]]; then
    echo "PAYMENT_FACT_APPLICATION_FIXTURE_ID is required for target release readiness smoke" >&2
    missing=1
  elif ! validate_positive_integer "PAYMENT_FACT_APPLICATION_FIXTURE_ID" "$PAYMENT_FACT_APPLICATION_FIXTURE_ID"; then
    missing=1
  fi
  if [[ -z "${PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID:-}" ]]; then
    echo "PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID is required for target release readiness smoke" >&2
    missing=1
  elif ! validate_positive_integer "PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID" "$PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID"; then
    missing=1
  fi
  if [[ "$missing" -ne 0 ]]; then
    exit 2
  fi
  cmd=(
    go run ./cmd/release_readiness_smoke
    -include-config
    -include-redis
    -include-provider-clients
    -include-fixture-claimability
    -payment-fact-application-fixture-id "$PAYMENT_FACT_APPLICATION_FIXTURE_ID"
    -payment-domain-outbox-fixture-id "$PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID"
    -format "$format"
  )
fi

if [[ "$dry_run" -eq 1 ]]; then
  printf '%q ' "${cmd[@]}"
  printf '\n'
  exit 0
fi

cd "$BACKEND_ROOT"
exec "${cmd[@]}"
