# TASK-PAY-003F-2 追偿直连支付下单命令记录任务卡

日期：2026-04-25

## 1. 目标

把商户追偿和骑手追偿的直连 JSAPI 支付下单同步返回记录为 command 结果，避免 create 返回被误读为追偿已支付或业务终态。

本任务是 TASK-PAY-003F 的第二个独立域切片，只覆盖追偿 direct payment 下单，不覆盖订单支付、押金充值、退款、分账、进件、提现或转账。

## 2. 范围

本段落地范围：

- `CreateMerchantClaimRecoveryPayment`。
- `CreateRiderClaimRecoveryPayment` 共用的 `createClaimRecoveryPayment` 新建支付单分支。
- 微信同步下单成功记录 `external_payment_commands.accepted`。
- 微信同步下单失败且本地 payment order 已关闭后记录 `external_payment_commands.rejected`。

不在本段处理：

- 复用已有 pending payment order 的 `GenerateJSAPIPayParams` 分支。
- 过期 pending 单关闭命令记录。
- 支付 callback/query fact application。
- 追偿业务状态推进。

## 3. 边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 `prepay_id` 或调起支付参数解释为追偿支付成功。
- 不改变追偿事件写入、支付回调、查询恢复、状态推进或已有 pending 单复用语义。
- command 记录失败只影响审计日志，不影响支付下单主流程。
- response snapshot 只允许保存 `out_trade_no`、`prepay_id`、稳定错误码和错误摘要；不得保存 `paySign`、密钥、证件号、银行卡号或完整微信原始 payload。

## 4. 验收

- 新建追偿支付单下单成功记录 provider `wechat`、channel `direct`、capability `direct_jsapi_payment`、command type `create_payment`、business owner `claim_recovery`、external object type `payment`、external object key `out_trade_no`。
- 成功记录使用 `prepay_id` 作为 secondary key，状态为 `accepted`。
- 同步拒绝记录状态为 `rejected`，并保留稳定错误码和摘要。
- 复用已有 pending 单不新增 command。

## 5. 验证

- `go test ./logic -run 'TestCreateMerchantClaimRecoveryPayment|TestCreateRiderClaimRecoveryPayment' -count=1`
- Review 确认本段没有新增 fact/application 写入，也没有新增业务终态推进。