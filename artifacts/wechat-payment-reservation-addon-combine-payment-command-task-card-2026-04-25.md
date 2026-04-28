# TASK-PAY-003F-4d 预订加菜合单支付下单命令记录任务卡

日期：2026-04-25

## 1. 目标

把预订加菜/改菜补差价路径中的收付通 `CreateCombineOrder` 同步返回记录为 command 结果，避免 `prepay_id` 或调起支付参数被误读为加菜款已支付。

本任务是 TASK-PAY-003F 的独立域切片，只覆盖 `createReservationAddonPaymentOrder` 中的合单 create，不覆盖通用合单支付、单笔 JSAPI 支付、退款、合单 query、timeout、callback/fact、支付成功业务推进或预订菜品状态流转。

## 2. 范围

本段落地范围：

- `AddReservationDishes` 和 `ModifyReservationDishes` 正差额复用的 `createReservationAddonPaymentOrder`。
- 微信同步下单成功且本地 `payment_order.prepay_id` 持久化成功后记录 `external_payment_commands.accepted`。
- 微信同步下单失败且本地加菜支付单和对应合单都关闭成功后记录 `external_payment_commands.rejected`。

不在本段处理：

- 预订菜品负差额退款任务和 worker refund command。
- 合单查询 `QueryCombineOrder`。
- 合单关闭 `CloseCombineOrder`。
- callback fact 写入、fact application 消费、支付成功业务状态推进。
- `UpdateCombinedPaymentOrderPrepay` 当前 best-effort 行为的语义重构。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 `prepay_id`、`pay_params` 或微信 create 成功解释为加菜款已支付。
- command 记录失败只影响审计日志，不影响预订加菜下单主流程。
- response snapshot 只保存 `combine_out_trade_no`、子单 `out_trade_no`、`prepay_id`、稳定错误码和错误摘要；不得保存 `paySign`、密钥、证件号、银行卡号或完整微信原始 payload。

## 4. 同步失败处理约束

本路径在微信 create API 返回错误或空 `prepay_id` 时会尝试关闭本地加菜支付单和对应合单。

只有本地支付单和合单都关闭成功后才写 `rejected` command。如果任一关闭动作失败，则不写 `rejected`，避免同一外部键仍处于不明确状态时被 command 表首次状态锁死。

`payment_order.prepay_id` 本地持久化失败时，本段不写 `accepted`；该分支沿用现有本地 failed 标记和错误返回。

## 5. 验收

- 加菜合单下单成功后记录 provider `wechat`、channel `ecommerce`、capability `combine_payment`、command type `create_payment`、business owner `reservation`、external object type `combined_payment`、external object key `combine_out_trade_no`。
- command 的 business object 指向本地 `payment_order`，便于预订加菜补款按支付单追踪。
- 成功记录使用 `prepay_id` 作为 secondary key，状态为 `accepted`。
- 微信 create API 失败或空 `prepay_id` 且本地支付单和合单都关闭成功后记录 `rejected`；任一关闭失败时不新增 command。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run 'TestCreateReservationAddonPaymentOrder' -count=1`
- `go test ./logic -run 'TestCreateCombinedPaymentOrder|TestCreateReservationAddonPaymentOrder' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把加菜合单 create 返回当成业务终态，并且失败记录只发生在本地关闭成功之后。