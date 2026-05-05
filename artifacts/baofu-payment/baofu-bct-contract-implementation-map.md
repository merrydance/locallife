# 宝付宝财通 2.0 契约实现映射

Updated: 2026-05-05
Risk class: G3 - payment/funds/callback/async contract boundary.

This document is a working map for implementing the LocalLife Baofoo BaoCaiTong contract layer. It does not replace the official Baofoo docs. The field-by-field build checklist lives in `artifacts/baofu-payment/baofu-bct-field-contract-matrix.md`; the full row-level audit and current local coverage live in `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md`; the source page ledger lives in `artifacts/baofu-payment/baofu-contract-source-matrix.md`.

## 1. Source Freshness

- Live source: `https://doc.mandao.com/docs/bct?token=I179BCdP579o` authenticated on 2026-05-05 with the supplied documentation account.
- Fresh scrape output: `/tmp/baofu_contract_snapshot_fresh.json` with 68 selected account/report/payment pages.
- Comparison: the fresh scrape matched the earlier `/tmp/baofu_contract_snapshot.json` page text and titles for the selected contract pages; no document text drift was detected in this pass.
- Empty article pages in the scrape are catalog/container pages, not field-truth pages: `APIList`, `bct-1f9o61rnb442i`, `bct-1f9qm38du1agl`, `bct-1f9qm3fvoqleu`, `bct-1f9qlnv7p9ucd`, `bct-1fao0hurn0adk`.

## 2. Contract Truth Rules

- Official doc pages are the only truth for field names, types, requiredness, conditional requiredness, enums, statuses, endpoint URLs, callback ACK, and retry behavior.
- Sandbox responses, Java demo code, and production logs are evidence only. They can create compatibility tests or Baofoo questions, but they cannot redefine production contract fields.
- `M/C/O/R` means mandatory / conditional / optional / returned as defined by Baofoo account docs. For aggregate/report pages, `是/否` maps to mandatory/optional; conditional rules are in field notes and adjacent appendix pages.
- A public-envelope success is not business success. Account `sysRespCode`, aggregate/report `returnCode`, and business `retCode/resultCode/txnState/refundState/state` must be interpreted at separate layers.
- Unknown enum/status/result-code values must fail closed or enter an explicit `unknown/needs_manual_review` state; they must never be normalized into success.

## 3. Document Dependency Graph

| Capability group | Read together | Why they are inseparable |
| --- | --- | --- |
| Account / union-gw | `notice`, `unionGw`, each `T-1001-013-*` page, account notification pages, account appendix/error/FAQ pages | `unionGw` owns endpoint, `verifyType`, URL params, encrypted `header/body`, and system response. Each API page only owns `body` request/response fields. Notifications use query/form `data_content`, not aggregate `dataContent`. |
| Merchant report | product page, interface notice, merchant-report request entry, `merchant_report`, `merchant_report_query`, `bind_sub_config`, report appendix, MCC/经营类目 file, demo as auxiliary evidence | Report request/response use the aggregate-style public envelope, but endpoint is `/mch-service/api`. `merchant_report` creates channel `subMchId`; `bind_sub_config(APPLET)` authorizes the LocalLife mini program. Appendix owns report/auth enums. |
| Aggregate pay | product/scene docs, interface notice, request entry, `unified_order`, `order_query`, `order_close`, `share_after_pay`, `share_query`, `order_refund`, `refund_query`, callback pages, aggregate appendices, FAQ | Request envelope owns `bizContent`; response/callback envelope owns `dataContent`; per-interface pages own business fields; callback pages and notification type/status appendices own async result interpretation and ACK behavior. |

## 4. Runtime Flow Map

```text
merchant/rider/operator account onboarding
  -> account open T-1001-013-01 or supported subtype
  -> open-account notification and/or queryAcc
  -> persist contractNo / sharing_mer_id as BaoCaiTong second-level account
  -> optional balance query
  -> merchant_report(WECHAT, bctMerId=sharing_mer_id)
  -> merchant_report_query until reportState=SUCCESS and subMchId exists
  -> bind_sub_config(authType=APPLET, authContent=LocalLife mini appid)
  -> payment readiness is true only after account + report + APPLET binding are all usable

main-business payment
  -> unified_order(prodType=SHARING, orderType=7, payCode=WECHAT_JSAPI)
  -> synchronous response gives tradeNo / txnState / wc_pay_data when accepted
  -> final state comes from PAYMENT callback or order_query recovery
  -> order_close is used for timeout/local post-create failure cleanup
  -> current BaoCaiTong aggregate docs expose single unified-order creation; no BCT aggregate combined-payment interface is present in the authenticated doc set

settlement after payment terminal success
  -> share_after_pay(originTradeNo or originOutTradeNo, sharingDetails[].sharingMerId)
  -> final share state from SHARING callback or share_query recovery
  -> reservation/reservation-addon payments use the same share_after_pay contract with order_source=reservation and no rider receiver

refund before sharing
  -> order_refund(originTradeNo or originOutTradeNo, outTradeNo, refundAmt, totalAmt)
  -> final refund state from REFUND callback or refund_query recovery
  -> LocalLife first version rejects post-share refund fields and post-share refund workflow

withdrawal
  -> queryBalace to inspect available/pending/current balance
  -> accWithdrawal to request withdrawal
  -> withdrawNotify or queryWithdrawal to converge final state
```

## 5. Identifier Ownership

| Identifier | Baofoo source | Local meaning | Must not be confused with |
| --- | --- | --- | --- |
| Collect `merId/terId` | Baofoo aggregate/report/acquiring contract | Level-1 transaction merchant/terminal for collecting and aggregate payment/report APIs | BaoCaiTong second-level account; WeChat `subMchId`; share receiver |
| Payout `memberId/terminalId` | Baofoo account/payout contract | Level-1 merchant/terminal for account/withdraw union-gw APIs when configured | Aggregate collect merchant identity |
| `contractNo` | Account open/query result | BaoCaiTong customer/account number; current source for local `sharing_mer_id` after Baofoo confirmation | WeChat channel `subMchId` |
| `sharing_mer_id` | Local persisted account binding | BaoCaiTong second-level account used in `sharingDetails[].sharingMerId` | Level-1 `merId`; `subMchId`; openid |
| `subMchId` / `sub_mch_id` | `merchant_report` / `merchant_report_query` channel return | WeChat/Alipay reported channel merchant identity for `unified_order.subMchId` | Share receiver; BaoCaiTong account |
| `sub_openid` | WeChat mini program user identity | Payer identity in `payExtend.sub_openid` | Merchant/channel account identifiers |
| `tradeNo` | Baofoo transaction number | Baofoo-side lookup key for payment/share/refund callbacks and query recovery | Local `outTradeNo` |
| `outTradeNo` | Local merchant order number sent to Baofoo | Idempotency and local correlation key; optional in some callbacks, so `tradeNo` fallback is required | Baofoo `tradeNo` |

## 6. Envelope Map

| Boundary | Endpoint | Request wrapper | Response wrapper | Callback wrapper | Security rule |
| --- | --- | --- | --- | --- | --- |
| Account union-gw | Sandbox `https://vgw.baofoo.com/union-gw/api/{serviceTp}/transReq.do`; production `https://public.baofu.com/union-gw/api/{serviceTp}/transReq.do` | URL params `memberId`, `terminalId`, `verifyType`, `content`, conditional `veryfyString`; plaintext `content` is JSON `{header, body}` | Decrypt by `verifyType`; response plaintext has `header.sysRespCode/sysRespDesc` and business `body` | Account notifications use `member_id`, `terminal_id`, `data_type=JSON`, encrypted `data_content` | Local first version uses `verifyType=1`: base64 plaintext then RSA private-key encryption; validate returned member/terminal/service type. |
| Aggregate payment | Sandbox `https://mch-juhe.baofoo.com/api`; production `https://juhe.baofoo.com/api`; backup `https://juhe-backup.baofoo.com/api` | Public fields `merId/terId/method/charset/version/format/timestamp/signType/signSn/ncrptnSn/dgtlEnvlp/signStr/bizContent` | `returnCode/returnMsg`; on `SUCCESS`, public fields plus signed `dataContent` | Public fields plus `notifyType` and signed `dataContent` | Verify signatures before trusting `dataContent`; `signSn/ncrptnSn` are S(10); SM2 remains fail-closed until implemented. |
| Merchant report | Sandbox `https://mch-juhe.baofoo.com/mch-service/api`; production `https://juhe.baofoo.com/mch-service/api` | Same aggregate public-envelope shape, different endpoint and method values | Same `returnCode` + signed `dataContent` model | No first-version report callback | Do not reuse account union-gw envelope or endpoint. |

## 7. Interface Contract Matrix

| Interface | Official method/code | Async? | Request contract target | Response / callback contract target | Local contract package | Open risk |
| --- | --- | --- | --- | --- | --- | --- |
| Account open | `T-1001-013-01`, plus personal two-factor page for DTO drift audit | Yes | `version=4.1.0`, `accType=1/2`, `accInfo`, `noticeUrl`, `businessType=BCT2.0`; personal four-factor requires identity/card/mobile/cardUserName/needUploadFile; business requires license/corporate/bank fields and self-employed conditional mobile rules | Sync `retCode/errorCode/errorMsg/result[]`; account `state=1/0/-1/2`; notification carries encrypted `data_content` and returns `OK` | `locallife/baofu/account/contracts/official_open.go`, `locallife/baofu/account/notification` | Product decision rejects personal two-factor at runtime; enterprise/credential long-tail needs more real samples. |
| Account query | `T-1001-013-03` | No | `version=4.0.0`, `accType`, either `contractNo`, or `loginNo + certificateNo + certificateType + platformNo`; `certificateType=ID/LICENSE` | `retCode/errorCode/errorMsg/result.contractNo`; successful query with `contractNo` means account exists/active for local normalization | `locallife/baofu/account/contracts/official_query.go` | Sandbox accepts looser loginNo-only query; keep that as sandbox leniency only. |
| Balance query | `T-1001-013-06` | No | `version=4.0.0`, `contractNo`, `accType` | `availableBal/pendingBal/currBal` optional yuan decimals; sample/observed `freezeBal` tolerated as auxiliary field | `locallife/baofu/account/contracts/official_balance.go` | Optional amount omissions should not become false provider success if all balances are absent. |
| Withdrawal | `T-1001-013-14` | Yes | `version=4.2.0`, `contractNo`, `transSerialNo`, `dealAmount` yuan, `returnUrl`, conditional fee/platform/abstract fields | Sync `state=1/2` is only acceptance; final comes from withdraw notify/query | `locallife/baofu/account/contracts/official_withdraw.go`, account notification parser | Real funds-action positive test still requires explicit approval/funding. |
| Withdrawal query | `T-1001-013-15` | No | `version=4.2.0`, `transSerialNo`, `tradeTime=yyyy-MM-dd` | `state=1/0/2/3`, `orderId`, yuan amount/fee/total fields | `locallife/baofu/account/contracts/official_withdraw.go` | Query date must be original trade date, not arbitrary current date in recovery. |
| Merchant report | `merchant_report` | No | `merId/terId`, `reportType=WECHAT`, unique `reportNo`, `bctMerId=sharing_mer_id`, nested `reportInfo` | `resultCode/errCode/errMsg`, `reportState`, `platformBizNo`, `subMchId` or `channelRetParam.sub_mch_id` | `locallife/baofu/merchantreport/contracts` | Real资料映射 and channel-side extra requirements remain to validate. |
| Merchant report query | `merchant_report_query` | No | `merId/terId/reportType/reportNo` | `reportState`; normalize `channelRetParam.sub_mch_id` into local channel `subMchId` | `locallife/baofu/merchantreport/contracts` | Must keep APPLET binding readiness separate from report success. |
| Bind sub config | `bind_sub_config` | No | `merId/terId`, `subMchId`, `authType=APPLET`, `authContent=<LocalLife mini appid>`, `remark` | `resultCode/errCode/errMsg`; success gates payment readiness after any operational propagation wait | `locallife/baofu/merchantreport/contracts` | This does not affect share receivers; it only gates channel payment. |
| Unified order | `unified_order` | Yes | `merId/terId/outTradeNo/txnAmt/txnTime/totalAmt/prodType=SHARING/orderType=7/payCode=WECHAT_JSAPI`; nested `payExtend`; conditional production `subMchId`; `riskInfo.clientIp` | Sync `txnState/tradeNo/reqChlNo/payCode/chlRetParam/resultCode`; final from PAYMENT callback or `order_query` | `locallife/baofu/aggregatepay/contracts` | Sandbox must omit `subMchId`; production must send reported `subMchId`. Sandbox cannot prove real payment. |
| Order query | `order_query` | No | `merId/terId` plus `tradeNo` or `outTradeNo` | `txnState`, `tradeNo/outTradeNo`, amounts/fees, channel return, `resultCode` | `locallife/baofu/aggregatepay/contracts` | Do not invent statuses outside official appendix. |
| Payment notification | `notifyType=PAYMENT` + payment callback page | Callback | Signed public notification envelope; business dataContent has payment fields; `outTradeNo` is optional, `tradeNo` can be recovery key | ACK exact uppercase `OK`; reject route notifyType mismatch, unsupported `txnState`, unsupported `resultCode`, missing required `payCode` | `locallife/baofu/aggregatepay/notification` | Real positive fact application evidence still missing. |
| Share after pay | `share_after_pay` | Yes | `merId/terId`, `originTradeNo` or `originOutTradeNo`, `txnTime`, `outTradeNo`, optional `notifyUrl`, `sharingDetails[].sharingMerId/sharingAmt` | Sync acceptance/result fields; final from SHARING callback or `share_query` | `locallife/baofu/aggregatepay/contracts` | Request must not contain `subMchId`; receiver must be BaoCaiTong `sharing_mer_id`. |
| Share query | `share_query` | No | `merId/terId` plus share `tradeNo` or `outTradeNo` | `txnState`, `succAmt`, `clearingDate`, `resultCode` | `locallife/baofu/aggregatepay/contracts` | Positive query requires real share order. |
| Share notification | `notifyType=SHARING` + share callback page | Callback | Signed public envelope; business dataContent has `txnState`, `resultCode`, amount/date fields | ACK `OK`; reject non-official `SHARE`, unsupported state/result | `locallife/baofu/aggregatepay/notification` | Real callback evidence missing. |
| Refund before sharing | `order_refund` | Yes | `merId/terId`, original payment reference, refund `outTradeNo`, `refundAmt`, `totalAmt`, `txnTime`, `refundReason`; first version rejects post-share `sharingRefundInfo` and `advanceAmt` | Sync acceptance/result; final from REFUND callback or `refund_query` | `locallife/baofu/aggregatepay/contracts` | Real pre-share refund requires real paid order. |
| Refund query | `refund_query` | No | `merId/terId` plus refund `tradeNo` or `outTradeNo` | `refundState`, `succAmt`, `finishTime`, `resultCode` | `locallife/baofu/aggregatepay/contracts` | Fake-order `ABNORMAL` only proves parser/reachability. |
| Refund notification | `notifyType=REFUND` + refund callback page | Callback | Signed public envelope; business dataContent requires `merId/terId/tradeNo/outTradeNo/resultCode/txnTime`; `refundState` optional | ACK `OK`; if `refundState` absent, `resultCode=SUCCESS/FAIL` drives terminal success/failure | `locallife/baofu/aggregatepay/notification` | Real callback evidence missing. |
| Order close | `order_close` | Yes | `merId/terId` plus `tradeNo` or `outTradeNo` | `resultCode/errCode/errMsg`; close terminal state can also be observed through payment callback/query | `locallife/baofu/aggregatepay/contracts` | Ambiguous create failure requires query/close recovery design. |
| Combined payment | Not present in current BaoCaiTong aggregate transaction-interface tree | N/A | Current BCT aggregate docs list `unified_order`, `share_after_pay`, `order_refund`, and `order_close`, but no合单/合并支付 request contract. | N/A | `locallife/logic/combined_payment_service.go` fail-closes Baofoo main-business combined payment; `GET /v1/payments/capabilities` exposes `split_checkout_required=true`; Mini Program cart/order-confirm split by merchant. | Baofoo public acquiring docs have a separate "合并支付" product, but it is not the current BaoCaiTong aggregate contract path. Keep split checkout unless Baofoo opens a BCT aggregate combined interface under this contract. |

## 8. Nested Structures That Need Dedicated DTOs

| Parent | Nested object/array | Required local treatment |
| --- | --- | --- |
| Account open | `accInfo` | Use separate DTOs for personal four-factor, personal two-factor, and business/self-employed. Runtime first version rejects two-factor even though DTO fields are retained for drift detection. |
| Merchant report | `reportInfo` | Do not flatten. Keep WeChat and Alipay structures separate. First version only posts `WECHAT`. |
| Merchant report WeChat | `address_info` | Required fields are `province_code`, `city_code`, `district_code`, `address`; optional `longitude`, `latitude`, `type`. Do not use `province/city/district/locationPoint`. |
| Merchant report WeChat | `bankcard_info` | Required `card_no`, `card_name`; optional `bank_branch_name`. Do not use `account_no/account_name/bank_name`. |
| Merchant report WeChat | `service_codes` | For LocalLife mini program payment, send both `JSAPI` and `APPLET`; then call `bind_sub_config(APPLET)`. |
| Unified order | `payExtend` | For `WECHAT_JSAPI`, require `sub_appid`, `sub_openid`, `body`. Do not let openid or appid leak into share/refund DTOs. |
| Unified order | `riskInfo` | For WeChat/Alipay payCode, require `clientIp`; optional `locationPoint`. |
| Unified order/refund | `mktInfo` / `mktRefundInfo` | First version does not use marketing flows; if added later, amount relationship tests must be added before production use. |
| Share after pay | `sharingDetails[]` | Require non-empty array; each item requires `sharingMerId` from BaoCaiTong account binding and positive `sharingAmt` in fen. |
| Refund | `sharingRefundInfo[]` | First version rejects this because LocalLife only supports pre-share refund. Add a new contract group before enabling post-share refund. |
| Aggregate response/callback | `chlRetParam` | Treat as method/payCode-specific nested data. For WeChat JSAPI, preserve `wc_pay_data` as JSON and tolerate documented/observed string-number scalar variance only at parser boundary. |

## 9. Enum / Status Ownership

| Enum group | Source page | First-version values in production path |
| --- | --- | --- |
| Account type | Account open/query/balance pages | `1` personal, `2` business/merchant. |
| Account open state | Account open/notification pages | `1` success, `0` failed, `-1` abnormal, `2` processing. |
| Withdrawal state | Withdrawal query/notify pages | `1` success, `0` failed, `2` processing, `3` returned. |
| Report type/state | Merchant-report appendix | `WECHAT`; `PROCESSING/SUCCESS/FAIL`. |
| Report auth type | Merchant-report appendix | `APPLET` for LocalLife mini program binding; `AUTH/JSAPI` reserved. |
| WeChat report service code | Merchant-report appendix | `JSAPI`, `APPLET`; `MICROPAY` reserved. |
| Aggregate sign type | Aggregate/report appendices | `RSA` and `SM2`; local production supports RSA and fails closed on unsupported SM2 paths. |
| Product/order/pay code | Aggregate pages/appendix | `prodType=SHARING`, `orderType=7`, `payCode=WECHAT_JSAPI`. |
| Notify type | Aggregate notify-type appendix | `PAYMENT`, `SHARING`, `REFUND`, `SIGN`; route-specific parser accepts only the matching first three. |
| Business result | Aggregate business pages | `SUCCESS`, `FAIL`; non-empty `errCode` with failure semantics must not be hidden by a communication success. |
| Payment/share/refund states | Aggregate order-status appendix and per-callback pages | Keep per-flow allowlists; unknown values fail closed or become explicit unknown, never success. |

## 10. Build Order For A 100% Accurate Contract Layer

1. Freeze source-page links per capability group in `baofu-contract-source-matrix.md` before changing DTOs.
2. Implement one official DTO per API body and one public envelope per transport family; do not let handler/logic build upstream JSON maps directly.
3. Add table-driven tests for every M/C field, enum, amount unit, ID source, and nested object JSON tag before touching runtime wiring.
4. Add response/callback parsers only after signature/envelope validation; signed `dataContent` is the trust boundary for aggregate/report flows.
5. Persist facts before applying business state so callbacks and query recovery are idempotent and auditable.
6. Add drift guards for known mistakes: account vs aggregate envelope mixup, `bizContent` vs `dataContent`, `SHARING` vs `SHARE`, `subMchId` vs `sharingMerId`, account yuan vs aggregate fen, production vs sandbox `subMchId` behavior.
7. Promote C3 to C4 only with masked evidence in `baofu-sandbox-evidence.md` or production-first-order checklist; endpoint reachability alone is not field-level proof.

## 11. Current Repo Anchors

| Purpose | Local file |
| --- | --- |
| Public aggregate/report envelopes | `locallife/baofu/envelope.go` |
| Account union-gw transport | `locallife/baofu/uniongw.go`, `locallife/baofu/account/client.go` |
| Account official DTOs | `locallife/baofu/account/contracts/official_open.go`, `locallife/baofu/account/contracts/official_query.go`, `locallife/baofu/account/contracts/official_balance.go`, `locallife/baofu/account/contracts/official_withdraw.go` |
| Account notifications | `locallife/baofu/account/notification/notification.go` |
| Merchant report DTOs/enums | `locallife/baofu/merchantreport/contracts/types.go`, `locallife/baofu/merchantreport/contracts/enums.go`, `locallife/baofu/merchantreport/contracts/categories_generated.go` |
| Aggregate payment DTOs | `locallife/baofu/aggregatepay/contracts/types.go` |
| Aggregate notifications | `locallife/baofu/aggregatepay/notification/notification.go` |
| Drift guard | `locallife/scripts/check_baofu_contract_drift.sh`, `locallife/Makefile` target `check-baofu-contract` |
| Field audit | `artifacts/baofu-payment/baofu-api-contract-coverage-audit.md` |
| Source matrix | `artifacts/baofu-payment/baofu-contract-source-matrix.md` |
| Sandbox evidence | `artifacts/baofu-payment/baofu-sandbox-evidence.md` |
