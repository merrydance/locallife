# TASK-PAY-003F-3e Worker 收付通退款命令记录任务卡

日期：2026-04-25

## 1. 目标

把 worker 中发起的收付通退款 create 同步返回记录为 command 结果，避免任务触发的退款 create 受理被误读为业务退款终态。

本任务是 TASK-PAY-003F 的第七个独立域切片，只覆盖 worker 里的 `CreateEcommerceRefund` 提交结果记录，不覆盖退款 callback/query fact、fact application 消费、分账回退命令、直连退款命令或业务终态推进。

## 2. 范围

本段落地范围：

- `ProcessTaskInitiateRefund` 中普通订单收付通退款 create。
- `processReservationRefund` 中预订/预订补差收付通退款 create。
- `ProcessTaskAnomalyRefund` 中已关闭/失败支付单异常到账收付通退款 create。
- 微信同步受理后，在本地 refund order 标记 processing 成功后记录 `external_payment_commands.accepted`。

不在本段处理：

- `CreateProfitSharingReturn` / 分账回退命令记录。
- 直连退款 create 命令记录。
- `RefundRecoveryScheduler` 查询路径。
- refund callback/query fact 写入和 fact application 消费。
- 任何退款业务终态推进。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 create 返回解释为退款成功、订单退款完成或业务终态。
- command 记录失败只影响审计日志，不影响 worker 主流程。
- response snapshot 只允许保存 `out_refund_no`、`refund_id` 等稳定且脱敏字段；不得保存完整微信原始 payload、证件号、银行卡号、密钥或签名。

## 4. 同步失败处理约束

worker 退款任务失败后通常会由 asynq 使用同一个 payload 和同一个 `out_refund_no` 重试；即使本地 refund order 被标记为 `failed`，后续任务仍可能复用该退款单号继续 create。

由于 `external_payment_commands` 以外部对象键去重且不会把第一条 `rejected` 更新为后续 `accepted`，本段不在 worker create 失败分支写入 `rejected` 或 `unknown` command。只有当微信 create 受理且本地 refund order 已标记 `processing` 后，才写入 `accepted`。

## 5. 验收

- worker 收付通退款 create 成功后记录 provider `wechat`、channel `ecommerce`、capability `ecommerce_refund`、command type `create_refund`、external object type `refund`、external object key `out_refund_no`。
- 成功记录使用微信 `refund_id` 作为 secondary key，状态为 `accepted`。
- command 的 business object 指向本地 `refund_order`。
- 普通订单路径 business owner 为 `order`，预订路径为 `reservation`。
- 微信 create API 失败分支不新增 command，不锁死后续 worker retry 的 accepted 审计。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./worker -run 'TestProcessTaskInitiateRefund_.*EcommerceRefund.*Command|TestProcessTaskAnomalyRefund_.*EcommerceRefund.*Command' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把退款 create 返回当成业务终态，并且失败分支未锁死可重试 command 状态。