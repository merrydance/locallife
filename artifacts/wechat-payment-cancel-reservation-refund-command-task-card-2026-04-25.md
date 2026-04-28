# TASK-PAY-003F-3c 预订取消收付通退款命令记录任务卡

日期：2026-04-25

## 1. 目标

把预订取消触发的收付通退款 create 返回记录为 command 结果，避免同步受理被误读为预订退款已完成。

本任务是 TASK-PAY-003F 的第五个独立域切片，只覆盖 `CancelReservation` 中的预订取消退款，不覆盖换菜退款、商户拒单退款、通用退款服务、worker 退款任务、退款查询、callback 或业务终态推进。

## 2. 范围

本段落地范围：

- `CancelReservation` 在 paid/confirmed 预订取消后创建的退款单。
- 微信收付通退款 create 成功受理后，在本地 refund order 标记 processing 后记录 `external_payment_commands.accepted`。

不在本段处理：

- 微信 create API 同步失败但本地 refund order 保持 pending 的分支。
- `RefundRecoveryScheduler` 或 worker 重试路径。
- 换菜退款、商户拒单退款、通用退款服务。
- refund callback/query fact 写入和 fact application 消费。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 create 返回解释为退款成功、预订完成退款或业务终态。
- 不改变现有 pending 退款恢复行为。
- command 记录失败只影响审计日志，不影响取消预订主流程。
- response snapshot 只允许保存 `out_refund_no`、`refund_id` 等稳定且脱敏字段；不得保存完整微信原始 payload、证件号、银行卡号、密钥或签名。

## 4. 同步失败处理约束

当前 `external_payment_commands` 以外部对象键去重，重复写入不会把第一条状态从 `rejected` 或 `unknown` 更新为 `accepted`。

预订取消退款在微信 create API 失败时会保留本地 refund order 为 pending，并交给恢复调度器后续使用同一个 `out_refund_no` 重试。因此本段不在该失败分支写入 `rejected` 或 `unknown` command，避免后续重试受理后无法记录 accepted command。

## 5. 验收

- 预订取消收付通退款 create 成功后记录 provider `wechat`、channel `ecommerce`、capability `ecommerce_refund`、command type `create_refund`、business owner `reservation`、external object type `refund`、external object key `out_refund_no`。
- 成功记录使用微信 `refund_id` 作为 secondary key，状态为 `accepted`。
- command 的 business object 指向本地 `refund_order`。
- 微信 API 失败分支不新增 command，不改变现有 pending + recovery 语义。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run TestCancelReservation -count=1`
- Review 确认本段没有新增 fact/application 写入，没有终态推进，并且失败分支未锁死可重试 command 状态。