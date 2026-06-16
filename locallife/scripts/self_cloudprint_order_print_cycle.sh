#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

LL_API_BASE="${LL_API_BASE:-http://127.0.0.1:8080/v1}"
MERCHANT_ID="${MERCHANT_ID:-12}"
TOKEN_USER_ID="${TOKEN_USER_ID:-2}"
PRINTER_SN="${PRINTER_SN:-DWTEST000001}"
DISH_ID="${DISH_ID:-44}"
DISH_NAME="${DISH_NAME:-招牌卤肉饭}"
DISH_PRICE="${DISH_PRICE:-2800}"
USER_ID="${USER_ID:-877}"
STOP_FILE="${STOP_FILE:-$BACKEND_ROOT/ops/STOP_SELF_CLOUD_PRINT_E2E}"
LOG_DIR="${LOG_DIR:-$BACKEND_ROOT/ops/logs}"

usage() {
  cat <<'USAGE'
Usage:
  scripts/self_cloudprint_order_print_cycle.sh [--dry-run]

Environment:
  LL_API_BASE    default http://127.0.0.1:8080/v1
  MERCHANT_ID    default 12
  TOKEN_USER_ID  default 2
  STOP_FILE      default <repo>/ops/STOP_SELF_CLOUD_PRINT_E2E
  LOG_DIR        default <repo>/ops/logs

Behavior:
  - If the stop file exists, exit 0 without creating a new order.
  - If the printer exists, create a fresh paid dine-in order and accept it.
  - Do not pass expires_at; let print_server use its default TTL.
USAGE
}

dry_run=0
while [[ $# -gt 0 ]]; do
  case "$1" in
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

mkdir -p "$LOG_DIR"

if [[ -f "$STOP_FILE" ]]; then
  printf '%s stop file present: %s\n' "$(date '+%F %T')" "$STOP_FILE"
  exit 0
fi

if ! [[ "$DISH_ID" =~ ^[1-9][0-9]*$ ]]; then
  echo "DISH_ID must be a positive integer" >&2
  exit 2
fi
if ! [[ "$DISH_PRICE" =~ ^[1-9][0-9]*$ ]]; then
  echo "DISH_PRICE must be a positive integer" >&2
  exit 2
fi
if ! [[ "$USER_ID" =~ ^[1-9][0-9]*$ ]]; then
  echo "USER_ID must be a positive integer" >&2
  exit 2
fi
if ! [[ "$MERCHANT_ID" =~ ^[1-9][0-9]*$ ]]; then
  echo "MERCHANT_ID must be a positive integer" >&2
  exit 2
fi

log() {
  printf '%s %s\n' "$(date '+%F %T')" "$*"
}

mint_token() {
  local token_src
  token_src="$(mktemp /tmp/ll-selfcloud-token-XXXXXX.go)"
  cat > "$token_src" <<'GO'
package main

import (
  "fmt"
  "os"
  "strconv"
  "time"

  "github.com/merrydance/locallife/token"
)

func main() {
  userID, err := strconv.ParseInt(os.Getenv("TOKEN_USER_ID"), 10, 64)
  if err != nil {
    panic(err)
  }
  maker, err := token.NewPasetoMaker(os.Getenv("TOKEN_SYMMETRIC_KEY"))
  if err != nil {
    panic(err)
  }
  tok, _, err := maker.CreateToken(userID, 15*time.Minute, token.TokenTypeAccessToken)
  if err != nil {
    panic(err)
  }
  fmt.Print(tok)
}
GO
  local token
  token="$(TOKEN_USER_ID="$TOKEN_USER_ID" /usr/local/go/bin/go run "$token_src")"
  rm -f "$token_src"
  printf '%s' "$token"
}

api_call() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local out
  out="$(mktemp /tmp/ll-selfcloud-api-XXXXXX)"
  local code
  if [[ -n "$body" ]]; then
    code="$(curl -sS -m 30 -o "$out" -w '%{http_code}' -X "$method" "$LL_API_BASE$path" \
      -H "Authorization: Bearer $TOKEN" \
      -H "X-Merchant-ID: $MERCHANT_ID" \
      -H 'Content-Type: application/json' \
      --data "$body")"
  else
    code="$(curl -sS -m 30 -o "$out" -w '%{http_code}' -X "$method" "$LL_API_BASE$path" \
      -H "Authorization: Bearer $TOKEN" \
      -H "X-Merchant-ID: $MERCHANT_ID")"
  fi
  printf '%s\t%s\n' "$code" "$out"
}

cd "$BACKEND_ROOT"
set -a
. ./app.env >/dev/null 2>&1
set +a
TOKEN="$(mint_token)"

printer_id="$(
  psql "$DB_SOURCE" -Atqc "select id from cloud_printers where printer_sn='${PRINTER_SN}' and deleted_at is null order by id desc limit 1;"
)"

if [[ -z "$printer_id" ]]; then
  echo "printer not registered: $PRINTER_SN" >&2
  exit 1
fi

if [[ "$dry_run" -eq 1 ]]; then
  printf 'dry_run=1 printer_id=%s printer_sn=%s\n' "$printer_id" "$PRINTER_SN"
  exit 0
fi

order_no="LL-SELF-CLOUD-CYCLE-$(date +%Y%m%d%H%M%S%N)"
log "creating order $order_no for printer $printer_id"
psql "$DB_SOURCE" -Atqc "
WITH new_order AS (
  INSERT INTO orders (
    order_no, user_id, merchant_id, order_type, subtotal, total_amount, status, fulfillment_status,
    payment_method, paid_at, notes, delivery_contact_name_snapshot, delivery_contact_phone_snapshot, delivery_address_snapshot
  ) VALUES (
    '$order_no', $USER_ID, $MERCHANT_ID, 'dine_in', $DISH_PRICE, $DISH_PRICE, 'paid', 'scheduled',
    'wechat', now(), 'LL self-cloud periodic cycle print', '', '', ''
  )
  RETURNING id
)
INSERT INTO order_items (order_id, dish_id, name, unit_price, quantity, subtotal, customizations)
SELECT id, $DISH_ID, '$DISH_NAME', $DISH_PRICE, 1, $DISH_PRICE, '{}'::jsonb
FROM new_order
RETURNING order_id;
"
order_id="$(
  psql "$DB_SOURCE" -Atqc "select id from orders where order_no='$order_no' limit 1;"
)"
if [[ -z "$order_id" ]]; then
  echo "order creation failed" >&2
  exit 1
fi

result="$(api_call POST "/merchant/orders/$order_id/accept")"
code="${result%%$'\t'*}"
body="${result#*$'\t'}"
if [[ "$code" != "200" ]]; then
  printf 'accept_failed http_status=%s body=' "$code"
  tr -d '\n' < "$body"
  printf '\n'
  rm -f "$body"
  exit 1
fi
rm -f "$body"

log "accepted order_id=$order_id order_no=$order_no printer_id=$printer_id"
log "waiting for print log success"

print_log=""
for _ in $(seq 1 90); do
  print_log="$(
    psql "$DB_SOURCE" -Atqc "select id || '|' || status || '|' || coalesce(vendor_order_id,'') || '|' || coalesce(task_key,'') || '|' || coalesce(error_message,'') from print_logs where order_id=$order_id order by id desc limit 1;"
  )"
  if [[ -n "$print_log" ]]; then
    print_status="${print_log#*|}"
    print_status="${print_status%%|*}"
    vendor_id="$(printf '%s' "$print_log" | cut -d'|' -f3)"
    if [[ "$print_status" == "success" || "$print_status" == "failed" || "$print_status" == "cancelled" ]]; then
      break
    fi
  fi
  sleep 2
done

if [[ -z "$print_log" ]]; then
  echo "no print log created" >&2
  exit 1
fi

print_log_id="$(printf '%s' "$print_log" | cut -d'|' -f1)"
print_status="$(printf '%s' "$print_log" | cut -d'|' -f2)"
vendor_id="$(printf '%s' "$print_log" | cut -d'|' -f3)"
task_key="$(printf '%s' "$print_log" | cut -d'|' -f4)"
error_message="$(printf '%s' "$print_log" | cut -d'|' -f5)"

log "print_log_id=$print_log_id status=$print_status vendor_order_id=$vendor_id task_key=$task_key error=$error_message"

if [[ "$print_status" != "success" ]]; then
  exit 1
fi

log "done"
