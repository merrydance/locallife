# TASK-PAY-003F-3b 预订换菜收付通退款命令记录任务卡

日期：2026-04-25

## 1. 目标

把预订换菜产生的收付通退款 create 返回记录为 command 结果，避免同步受理被误读为预订退款已完成。

本任务是 TASK-PAY-003F 的第四个独立域切片，只覆盖 `ReplaceReservationOrder` 中的换菜退款，不覆盖预订取消、商户拒单、通用退款服务、worker 退款任务、退款查询、callback 或业务终态推进。

## 2. 范围

本段落地范围：

- `ReplaceReservationOrder` 计算出差额为负数时创建的退款单。
- `processReplaceOrderRefund` 调用微信收付通退款 create 的成功和同步失败分支。
- 微信同步受理后，在本地 refund order 标记 processing 后记录 `external_payment_commands.accepted`。
- 微信同步拒绝且本地 refund order 已标记 failed 后记录 `external_payment_commands.rejected`。

不在本段处理：

- 预订取消退款。
- 商户拒单退款。
- 通用 `RefundService` 退款。
- worker/recovery 重试路径。
- refund callback/query fact 写入和 fact application 消费。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 create 返回解释为退款成功、预订完成退款或业务终态。
- 不改变现有 refund order processing/failed 更新语义。
- command 记录失败只影响审计日志，不影响换菜主流程。
- response snapshot 只允许保存 `out_refund_no`、`refund_id`、稳定错误码和错误摘要；不得保存完整微信原始 payload、证件号、银行卡号、密钥或签名。

## 4. 验收

- 预订换菜收付通退款 create 成功后记录 provider `wechat`、channel `ecommerce`、capability `ecommerce_refund`、command type `create_refund`、business owner `reservation`、external object type `refund`、external object key `out_refund_no`。
- 成功记录使用微信 `refund_id` 作为 secondary key，状态为 `accepted`。
- 同步拒绝记录状态为 `rejected`，并保留稳定错误码和摘要。
- command 的 business object 指向本地 `refund_order`。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 5. 验证

- `go test ./logic -run TestReplaceReservationOrder -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把退款 create 返回当成业务终态。