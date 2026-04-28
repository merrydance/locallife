# TASK-PAY-003F-3d 通用退款服务收付通退款命令记录任务卡

日期：2026-04-25

## 1. 目标

把 `RefundService.CreateRefundOrder` 中收付通退款 create 的同步返回记录为 command 结果，避免退款 create 受理或同步拒绝被误读为业务退款终态。

本任务是 TASK-PAY-003F 的第六个独立域切片，只覆盖通用退款服务里最终调用 `CreateEcommerceRefund` 的退款 create，不覆盖分账回退 create/finish、worker 退款任务、退款查询、callback 或业务终态推进。

## 2. 范围

本段落地范围：

- `RefundService.CreateRefundOrder` 创建的本地 refund order。
- `processProfitSharingRefund` 内分账回退前置处理完成后发起的收付通退款 create。
- 微信同步受理后，在本地 refund order 标记 processing 后记录 `external_payment_commands.accepted`。
- 微信同步拒绝且本地 refund order 已标记 failed 后记录 `external_payment_commands.rejected`。

不在本段处理：

- `CreateProfitSharingReturn` / 分账回退命令记录。
- `RefundRecoveryScheduler` 或 worker 重试路径。
- 商户拒单、预订取消、换菜退款已独立覆盖的路径。
- refund callback/query fact 写入和 fact application 消费。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 create 返回解释为退款成功、订单退款完成或业务终态。
- 不改变现有 refund order processing/failed 更新语义。
- command 记录失败只影响审计日志，不影响退款主流程。
- response snapshot 只允许保存 `out_refund_no`、`refund_id`、稳定错误码和错误摘要；不得保存完整微信原始 payload、证件号、银行卡号、密钥或签名。

## 4. 同步失败处理约束

本路径在微信 create API 返回错误时会把本地 refund order 标记 failed，并将错误返回调用方。因此可以记录 `rejected` command；后续不会期望同一个 pending `out_refund_no` 被 recovery 原样重试为 accepted。

如果后续改成 pending + recovery 语义，必须同步修改本任务边界，避免 `external_payment_commands` 首次 rejected 记录锁死后续 accepted 审计。

## 5. 验收

- 收付通退款 create 成功后记录 provider `wechat`、channel `ecommerce`、capability `ecommerce_refund`、command type `create_refund`、external object type `refund`、external object key `out_refund_no`。
- 成功记录使用微信 `refund_id` 作为 secondary key，状态为 `accepted`。
- 同步拒绝记录状态为 `rejected`，并保留稳定错误码和摘要。
- command 的 business object 指向本地 `refund_order`。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run TestCreateRefundOrder -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把退款 create 返回当成业务终态。