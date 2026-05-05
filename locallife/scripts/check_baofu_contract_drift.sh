#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

check_absent() {
  local description="$1"
  local pattern="$2"
  shift 2
  local output
  if output=$(rg -n --glob '!**/*_test.go' "$pattern" "$@" 2>/dev/null); then
    echo "baofu contract drift detected: ${description}" >&2
    echo "$output" >&2
    exit 1
  fi
}

check_present() {
  local description="$1"
  local pattern="$2"
  shift 2
  if ! rg -n "$pattern" "$@" >/dev/null 2>&1; then
    echo "baofu contract drift detected: ${description}" >&2
    exit 1
  fi
}

check_absent "public response callers must not read legacy BizContent directly" 'responseEnvelope\.BizContent' baofu
check_absent "official endpoint profiles must not use deprecated api.baofoo.com" 'https://api\.baofoo\.com' baofu util app.env.example
check_absent "share_after_pay must not use WeChat subMchId" 'subMchId.*share|share.*subMchId' baofu/aggregatepay logic
check_absent "sharingMerId must not come from Baofoo level-1 merchant ids" 'CollectMerchantID.*sharingMerId|PayoutMerchantID.*sharingMerId' baofu logic
check_absent "union-gw verifyType=1 must not require static BAOFU_AES_KEY" 'BAOFU_AES_KEY' baofu util app.env.example
check_absent "merchant_report address_info must use official *_code field names" 'json:"(province|city|district)"|LocationPoint' baofu/merchantreport
check_absent "merchant_report bankcard_info must use official card_no/card_name field names" 'json:"(account_name|account_no|bank_name)"' baofu/merchantreport
check_absent "merchant_report mini program payment must request JSAPI and APPLET service codes together" 'ServiceCodes:\s*\[\]string\{[^}]*WechatServiceTypeApplet[^}]*\}' baofu/merchantreport logic
check_present "unified_order must keep environment-aware subMchId validation" 'ValidateForEnvironment' baofu/aggregatepay
check_present "unified_order sandbox transport must omit subMchId before posting" 'WithoutSubMchID' baofu/aggregatepay

echo "baofu contract drift guard passed"
