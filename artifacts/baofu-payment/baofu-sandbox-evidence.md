# 宝付宝财通 Sandbox Evidence

> 本文件只记录宝付测试地址联调证据索引。没有证据行的接口不得从 C3 提升到 C4。

## Evidence Rules

- 不保存身份证号、完整银行卡号、手机号、私钥、AES key、完整签名、完整数字信封、完整原始 payload。
- 只保存脱敏后的请求/响应摘要、接口名、endpoint、商户侧流水号、宝付交易/报备/提现号、观察到的状态、回调/查询恢复结果、测试时间和 commit SHA。
- `OutRequestNo`、`OutTradeNo`、`ReportNo`、`ShareOutTradeNo`、`RefundOutTradeNo` 可以保留业务前缀和末 6 位；`subMchId`、`sharingMerId`、`contractNo` 只保留前 4 后 4。
- 每次联调必须同时记录：请求是否命中测试地址、是否收到/解析回包、是否落本地 command/fact、查询是否能补偿回调缺失、前端/用户可见错误是否为安全中文语义。
- 联调失败也要记录一行，`Result`/`Observed Status` 标记失败类别，`Notes` 写下一步处理，不写 raw upstream message。

## Ready For Next Sandbox Test

- [ ] 用安全测试身份资料完成一次 `open_personal` 或机构开户正向测试，并通过查询拿到 `contractNo`/`sharing_mer_id` 脱敏证据。
- [ ] 用已开户的宝付二级商户号完成 `merchant_report`、`merchant_report_query` 和 `bind_sub_config(authType=APPLET)` 正向测试。
- [ ] 用真实 `subMchId` 和本小程序下真实 `sub_openid` 完成 `unified_order`，拿到 `wc_pay_data`，并记录回调/查询补偿证据。
- [ ] 基于已支付订单完成 `share_after_pay`、`share_query` 和分账回调证据。
- [ ] 基于分账前订单完成 `order_refund`、`refund_query` 和退款回调证据。
- [ ] 用真实 `contractNo` 完成余额查询、提现、提现查询和提现回调证据。
- [ ] 为账户、报备、聚合支付各补一条参数/配置/处理中类错误样例，验证 API 安全文案不泄露上游原文。

## Account Open `T-1001-013-01`

| Date | Env | Endpoint | OutRequestNo | Owner | Owner Type | Result | ContractNo Masked | SharingMerID Masked | Callback | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-01/transReq.do` | `BAOFU_OPEN_PERSON_20260505071859` | masked personal identity | personal two-factor | reached sandbox; parsed union-gw response; upstream returned abnormal with `BF0001` | - | - | no | no | `bee8a3d2` | Negative open-account evidence only. The provider accepted the request shape but did not open an account; no `contractNo`/`sharing_mer_id` was returned, so balance query and merchant report cannot proceed from this run. |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-01/transReq.do` | generated `BAOFU_OPEN_4FACTOR_*` | masked personal identity | personal four-factor | reached sandbox; response parsed as abnormal with `BF0001`, but local result fields were blank before numeric `retCode/state` parser fix | - | - | no | no | `64e5fecf` | Negative four-factor evidence. The run exposed two account response details from the official doc/sample: business fields can be inside `result[]`, and `retCode`/`state` can be numeric. Local parser now accepts string/number scalars. Rerun required after deploy to capture the masked `transSerialNo` and failure details correctly. |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-01/transReq.do` | generated `BAOFU_OPEN_4FACTOR_*` | masked personal identity | personal four-factor | reached sandbox; local client returned `ProviderError` with upstream `BF0001` | - | - | no | no | `505da5f2` | Negative four-factor evidence after numeric parser fix. The request now fails closed as a provider business failure instead of a zero-value success; remaining issue is Baofoo-side identity/card validation or sandbox whitelist, not local response-shape drift. |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-01/transReq.do` | `BAOFU_OPEN_4FACTOR_202605050001` | masked personal identity | personal four-factor | accepted; upstream state `2` / processing | - | - | pending | pending | `fd495dc8` | Positive acceptance evidence for four-factor open-account request shape. No `contractNo`/`sharing_mer_id` yet; poll `T-1001-013-03` by `loginNo=BAOFU_OPEN_4FACTOR_202605050001` until active/failed, and do not proceed to balance/report before active. |

## Account Query `T-1001-013-03`

| Date | Env | Endpoint | Query Key | Owner | Result | ContractNo Masked | SharingMerID Masked | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-05-04 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-03/transReq.do` | smoke synthetic account query | platform config smoke | reached sandbox; parsed union-gw envelope; upstream returned `BF0005`/abnormal without `contractNo` | - | - | `002c86f6` | Synthetic query did not prove a successful account exists; it only proves account-query transport/decryption is reachable. Treat as negative smoke evidence; positive C4 requires a real opened test account or a known query key from Baofoo. |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-03/transReq.do` | `loginNo:BAOF***0843` | platform config smoke | reached sandbox; parsed union-gw envelope; upstream returned `BF0005`/abnormal without `contractNo` | - | - | `773ac598` | Rerun after public-envelope/dataContent fixes still proves union-gw account-query transport/decryption only. Positive account C4 still requires Baofoo-accepted test identity material or a known successful query key. |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-03/transReq.do` | `loginNo:BAOF***0001` | personal four-factor open query | reached sandbox; provider returned `BF0003` before personal `accType` query fix | - | - | `b8a3bb10` | Negative query evidence. The open request was personal `accType=1`, but the local query adapter still defaulted `QueryAccount` to business `accType=2`; code now carries `AccountType` into query/balance requests. Rerun with `AccountType=personal`. |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-03/transReq.do` | `loginNo:BAOF***0001` | personal four-factor open query | reached sandbox; returned `contractNo` with no explicit state and top-level success code | `CP61***2938` | `CP61***2938` | `e3f525e2` | Positive account-query evidence. Official query response does not document a `state` field; local parser now treats a successful query with `contractNo` as active and clears success-like `fail_code`. Use this `contractNo` for balance query and this Baofoo secondary merchant ID as `bctMerId` for merchant report unless Baofoo later returns a distinct secondary merchant field. |

## Account Balance `T-1001-013-06`

| Date | Env | Endpoint | ContractNo Masked | Available Fen | Pending Fen | Frozen Fen | Result | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-05-05 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-06/transReq.do` | `CP61***2938` | - | - | - | reached sandbox; local client returned provider_error with system code `S_0000` before parsing business balance | `7133b966` | Negative balance evidence. Root cause is local contract drift: balance query page requires `version=4.0.0` and official examples return numeric balance fields, while deployed code sent `4.1.0` and only accepted string balances. Fixed locally; redeploy and rerun required before C4. |

## Withdrawal `T-1001-013-14`

| Date | Env | Endpoint | TransSerialNo | Owner | Amount Fen | Result | Baofu WithdrawNo Masked | Callback | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Withdrawal Query `T-1001-013-15`

| Date | Env | Endpoint | TransSerialNo | TradeTime | Result | Local Recovery | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Merchant Report `merchant_report`

| Date | Env | Endpoint | ReportNo | Merchant | BCTMerID Masked | Result | subMchId Masked | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Merchant Report Query `merchant_report_query`

| Date | Env | Endpoint | ReportNo | Result | subMchId Masked | Local Sync | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Bind Sub Config `bind_sub_config` / `APPLET`

| Date | Env | Endpoint | subMchId Masked | AuthType | AuthContent Masked | Result | Payment Readiness | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Unified Order `unified_order`

| Date | Env | Endpoint | OutTradeNo | subMchId Masked | Amount Fen | wc_pay_data | Callback | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Payment Query `order_query`

| Date | Env | Endpoint | OutTradeNo/TradeNo | Result | Local Fact | Local Application | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; public envelope returned `FAIL` | no | no | `640a0d1b` | Exposed local diagnostics gap: envelope `returnMsg` was not carried into `ProviderError.UpstreamMessage`. Fixed in next commit; rerun required to classify whether this is expected missing-order response or config/signing issue. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; upstream `FAIL` message indicates `merId` missing from `bizContent` | no | no | `002c86f6` | Smoke request omitted query-level `merId/terId`; local contract now validates `PaymentQueryRequest` before POST so this does not reach Baofoo as a malformed request. Rerun with collect merchant/terminal in request body. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; upstream still reported `商户号不能为null` after DTO carried `merId/terId` | no | no | `6bec7b1f` | Root cause narrowed to public envelope wire format: official docs and Baofoo Java demo define `bizContent` as JSON string (`S`) and `signStr` as SHA256withRSA hex over that string; local client was sending `bizContent` as JSON object and base64 signature. Fixed in next commit; rerun required. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; upstream still reported `商户号不能为null` after JSON-string `bizContent` and hex signature | no | no | `d1b8dc55` | Rechecked Baofoo demo transport: public-envelope request is `application/x-www-form-urlencoded`, not JSON body. Fixed in next commit; rerun required. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; upstream `FAIL` message changed to `时间戳传入错误` | no | no | `c67d7fdf` | Form transport fixed the prior null-merchant parse error. Remaining issue matched public-envelope timestamp: docs require sender timestamp within 10 minutes; Java demo uses local `new Date()` formatted as `yyyyMMddHHmmss`, while Go used UTC. Fixed in next commit to Asia/Shanghai timestamp. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; upstream `FAIL` message changed to `签名证书序号字符长度超限（10）` | no | no | `5f4c3313` | Public-envelope parse/timestamp passed. Remaining config issue: `signSn/ncrptnSn` are S(10) public-envelope certificate indexes, not the 16-char certificate serials derived from PEM. Baofoo Java demo defaults both to `1`; local config validation now rejects values longer than 10 before startup/smoke. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | reached sandbox; public envelope returned `SUCCESS/OK` with signed `dataContent` and no `bizContent` | no | no | `b4961434` | This proves the request-side public envelope now passes Baofoo parsing/signature/timestamp/serial checks. Local client still expected response `bizContent`; fix parses official response `dataContent` while retaining legacy `bizContent` fallback for local fixtures. Rerun after deploy should classify the business payload, likely as order-not-found/manual-review for the synthetic order. |
| 2026-05-04 | sandbox | `https://mch-juhe.baofoo.com/api` | smoke synthetic order query | parsed official `dataContent`; client returned success with `resultCode=SUCCESS` and `txnState=ABNORMAL` | no | no | `b4961434` | Public envelope request/response and `dataContent` parsing are now proven against sandbox for `order_query`. The synthetic order still has no `outTradeNo/tradeNo`; treat this as transport/contract evidence only, not a real paid-order query or local fact-application proof. |
| 2026-05-05 | sandbox | `https://mch-juhe.baofoo.com/api` | `BAOFU_SMOKE_ORDER_20260505070843` | parsed official `dataContent`; client returned success with `resultCode=SUCCESS` and `txnState=ABNORMAL` | no | no | `773ac598` | Rerun against deployed sandbox config confirms the previous fix: form public envelope, local Shanghai timestamp, S(10) serial indexes, response `dataContent` parsing. Synthetic order has no local fact/application and is not a paid-order C4 proof. |

## Payment Callback

| Date | Env | Callback URL | OutTradeNo | TradeNo Masked | Observed Status | Fact Persisted | Application Enqueued | ACK | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Profit Sharing `share_after_pay`

| Date | Env | Endpoint | ShareOutTradeNo | Origin TradeNo Masked | Receiver Count | Amount Fen | Result | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Profit Sharing Query `share_query`

| Date | Env | Endpoint | ShareOutTradeNo/TradeNo | Result | Local Fact | Local Application | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Profit Sharing Callback

| Date | Env | Callback URL | ShareOutTradeNo | TradeNo Masked | Observed Status | Fact Persisted | Application Enqueued | ACK | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Refund Before Share `order_refund`

| Date | Env | Endpoint | RefundOutTradeNo | Origin OutTradeNo/TradeNo | Amount Fen | Result | Callback | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Refund Query `refund_query`

| Date | Env | Endpoint | RefundOutTradeNo/TradeNo | Result | Local Recovery | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |

## Refund Callback

| Date | Env | Callback URL | RefundOutTradeNo | Observed Status | Fact Persisted | Application Enqueued | ACK | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Order Close `order_close`

| Date | Env | Endpoint | OutTradeNo/TradeNo | Trigger | Result | Local State | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
