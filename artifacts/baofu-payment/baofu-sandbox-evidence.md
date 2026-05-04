# 宝付宝财通 Sandbox Evidence

> 本文件只记录宝付测试地址联调证据索引。没有证据行的接口不得从 C3 提升到 C4。

## Evidence Rules

- 不保存身份证号、完整银行卡号、手机号、私钥、AES key、完整签名、完整数字信封、完整原始 payload。
- 只保存脱敏后的请求/响应摘要、接口名、endpoint、商户侧流水号、宝付交易/报备/提现号、观察到的状态、回调/查询恢复结果、测试时间和 commit SHA。
- `OutRequestNo`、`OutTradeNo`、`ReportNo`、`ShareOutTradeNo`、`RefundOutTradeNo` 可以保留业务前缀和末 6 位；`subMchId`、`sharingMerId`、`contractNo` 只保留前 4 后 4。
- 每次联调必须同时记录：请求是否命中测试地址、是否收到/解析回包、是否落本地 command/fact、查询是否能补偿回调缺失、前端/用户可见错误是否为安全中文语义。
- 联调失败也要记录一行，`Result`/`Observed Status` 标记失败类别，`Notes` 写下一步处理，不写 raw upstream message。

## Account Open `T-1001-013-01`

| Date | Env | Endpoint | OutRequestNo | Owner | Owner Type | Result | ContractNo Masked | SharingMerID Masked | Callback | Query | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Account Query `T-1001-013-03`

| Date | Env | Endpoint | Query Key | Owner | Result | ContractNo Masked | SharingMerID Masked | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-05-04 | sandbox | `https://vgw.baofoo.com/union-gw/api/T-1001-013-03/transReq.do` | smoke synthetic account query | platform config smoke | reached sandbox; parsed union-gw envelope; upstream returned `BF0005`/abnormal without `contractNo` | - | - | `002c86f6` | Synthetic query did not prove a successful account exists; it only proves account-query transport/decryption is reachable. Treat as negative smoke evidence; positive C4 requires a real opened test account or a known query key from Baofoo. |

## Account Balance `T-1001-013-06`

| Date | Env | Endpoint | ContractNo Masked | Available Fen | Pending Fen | Frozen Fen | Result | Commit | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

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
