# TASK-PAY-003F-4b 预订换菜补款单笔支付命令记录任务卡

日期：2026-04-25

## 1. 目标

把预订换菜差额为正时发起的收付通单笔 JSAPI 支付下单同步返回记录为 command 结果，避免 `prepay_id` 或调起支付参数被误读为换菜补款已支付。

本任务是 TASK-PAY-003F 的第九个独立域切片，只覆盖 `ReplaceReservationOrder` 中 `createReplaceOrderEcommercePayment` 的补款支付 create，不覆盖 `PaymentOrderService` 常规订单/预订下单、合单支付、退款、支付 callback/query fact、timeout 查单或业务终态推进。

## 2. 范围

本段落地范围：

- `ReplaceReservationOrder` 在 `delta > 0` 时创建的新补款 payment order。
- `createReplaceOrderEcommercePayment` 调用 `CreatePartnerJSAPIOrder` 的成功和同步失败分支。
- 微信同步下单成功且本地 `prepay_id` 持久化成功后记录 `external_payment_commands.accepted`。
- 微信同步下单失败且本地 payment order 已关闭后记录 `external_payment_commands.rejected`。

不在本段处理：

- 常规订单/预订支付下单。
- 合单 `CreateCombineOrder`。
- 换菜退款、预订取消退款、通用退款服务或 worker 退款任务。
- 支付 callback/query fact 写入和 fact application 消费。
- payment success 业务状态推进。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 `prepay_id`、`pay_params` 或微信 create 成功解释为补款已支付。
- command 记录失败只影响审计日志，不影响补款下单主流程。
- response snapshot 只允许保存 `out_trade_no`、`prepay_id`、稳定错误码和错误摘要；不得保存 `paySign`、密钥、证件号、银行卡号或完整微信原始 payload。

## 4. 同步失败处理约束

本路径在微信 create API 返回错误时会关闭本地 payment order，然后返回错误给调用方；后续重试会重新创建新的 payment order 和新的 `out_trade_no`。因此只有本地关闭成功后才写 `rejected` command。

如果关闭本地 payment order 失败，则不写 `rejected`，避免同一外部键仍处于不明确状态时被 command 表首次状态锁死。

`prepay_id` 本地持久化失败时，本段不写 `accepted`；该分支会尝试关闭微信订单并把本地 payment order 标记 failed，属于本地接管失败，后续需另按恢复设计处理。

## 5. 验收

- 换菜补款收付通单笔下单成功后记录 provider `wechat`、channel `ecommerce`、capability `partner_jsapi_payment`、command type `create_payment`、business owner `reservation`、external object type `payment`、external object key `out_trade_no`。
- 成功记录使用 `prepay_id` 作为 secondary key，状态为 `accepted`。
- command 的 business object 指向本地 `payment_order`。
- 微信 create API 失败且本地关闭成功后记录 `rejected`；关闭失败时不新增 command。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run 'TestReplaceReservationOrder_.*PaymentCommand|TestReplaceReservationOrder_DeltaPositive|TestOrderServiceReplaceOrderSchedulesPaymentTimeout' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把支付 create 返回当成业务终态，并且失败记录只发生在本地关闭成功之后。