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
check_absent "payment notification must not require optional outTradeNo" 'ErrPaymentNotificationOutTradeNoRequired|payment notification outTradeNo is required' baofu/aggregatepay/notification api
check_absent "aggregate callback notifyType must use official SHARING value, not SHARE" '"SHARE"' baofu/aggregatepay/notification baofu/envelope.go api
check_present "unified_order must keep environment-aware subMchId validation" 'ValidateForEnvironment' baofu/aggregatepay
check_present "unified_order sandbox transport must omit subMchId before posting" 'WithoutSubMchID' baofu/aggregatepay
check_present "account query must keep official version 4.0.0" 'OfficialQueryAccountVersion = "4\.0\.0"' baofu/account/contracts
check_present "account query loginNo mode must require official identity/platform fields" 'certificateNo is required when loginNo is used|platformNo is required when loginNo is used' baofu/account/contracts
check_present "withdrawal request must expose official feeMemberId" 'FeeMemberID\s+string' baofu/account/contracts
check_present "withdrawal client must send feeMemberId to Baofoo" 'FeeMemberID:\s+strings\.TrimSpace\(req\.FeeMemberID\)' baofu/account/client.go
check_present "withdrawal command dispatch must populate feeMemberId from account binding" 'FeeMemberID:\s+feeMemberID' worker/task_baofu_withdrawal_command_dispatch.go
check_present "aggregate callbacks must unwrap official dataContent envelope" 'normalizeAggregateNotificationDataContent' baofu/aggregatepay/notification
check_present "aggregate callbacks must support signed public envelope parser" 'NewParserWithPublicKey' baofu/aggregatepay/notification api
check_present "aggregate callback envelope must verify dataContent signature" 'PublicNotificationEnvelope.*VerifySignature|VerifySignature\(publicKeyPEM string\)' baofu
check_present "aggregate callback envelope must enforce official notifyType enum" 'PublicNotificationTypeSharing = "SHARING"|notifyType is unsupported' baofu/envelope.go
check_present "aggregate callbacks must reject mismatched route notifyType" 'notifyType must be' baofu/aggregatepay/notification
check_present "aggregate callbacks must validate official business resultCode enum" 'resultCode is unsupported|BusinessResultCodeFail' baofu/aggregatepay
check_present "aggregate callbacks must validate official state enums" 'txnState is unsupported|refundState is unsupported|IsSupportedPaymentState|IsSupportedShareState|IsSupportedRefundState' baofu/aggregatepay
check_present "aggregate callbacks must normalize numeric JSON scalars for documented string fields" 'normalizeAggregateNotificationStringScalars|isAggregateNotificationStringField' baofu/aggregatepay/notification
check_present "public envelope serial fields must enforce official S(10)" 'signSn must be at most 10 characters|ncrptnSn must be at most 10 characters' baofu
check_present "public aggregate response must verify dataContent signature" 'responseEnvelope\.VerifySignature' baofu/client.go
check_present "payment notification must require official payCode" 'ErrPaymentNotificationPayCodeRequired' baofu/aggregatepay/notification
check_present "personal two-factor open account must remain rejected in runtime contract" 'personal two-factor is not supported' baofu/account/contracts
check_present "business open account DTO must include official corporateMobile conditional field" 'CorporateMobile.*json:"corporateMobile,omitempty"' baofu/account/contracts
check_present "withdrawal recovery must send official tradeTime" 'TradeTime:\s+order\.CreatedAt\.Format\("2006-01-02"\)' worker

echo "baofu contract drift guard passed"
