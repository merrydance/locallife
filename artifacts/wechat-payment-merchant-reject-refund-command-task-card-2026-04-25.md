# TASK-PAY-003F-3a 商户拒单收付通退款命令记录任务卡

日期：2026-04-25

## 1. 目标

把商户拒单触发的订单收付通退款 create 返回记录为 command 结果，避免微信同步受理被误读为订单退款已完成。

本任务是 TASK-PAY-003F 的第三个独立域切片，只覆盖商户拒单订单退款，不覆盖预订取消、换单退款、通用退款服务、worker 退款任务、退款查询、callback 或业务终态推进。

## 2. 范围

本段落地范围：

- `ProcessMerchantRejectRefund` 创建的订单退款单。
- `processMerchantRejectEcommerceRefund` 调用微信收付通退款 create 的成功分支。
- 微信同步受理后，在本地 refund order 标记 processing 后记录 `external_payment_commands.accepted`。

不在本段处理：

- 微信 create API 同步失败但本地 refund order 保持 pending 的分支。
- `RefundRecoveryScheduler` 或 worker 重试路径。
- 预订取消、换单退款、异常退款、分账回退退款。
- refund callback/query fact 写入和 fact application 消费。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 create 返回解释为退款成功、订单完成退款或业务终态。
- 不改变现有 pending 退款恢复行为。
- command 记录失败只影响审计日志，不影响商户拒单主流程。
- response snapshot 只允许保存 `out_refund_no`、`refund_id` 等稳定且脱敏字段；不得保存完整微信原始 payload、证件号、银行卡号、密钥或签名。

## 4. 同步失败处理约束

当前 `external_payment_commands` 以外部对象键去重，重复写入不会把第一条状态从 `rejected` 或 `unknown` 更新为 `accepted`。

商户拒单退款在微信 create API 失败时会保留本地 refund order 为 pending，并交给恢复调度器后续使用同一个 `out_refund_no` 重试。因此本段不在该失败分支写入 `rejected` 或 `unknown` command，避免后续重试受理后无法记录 accepted command。

如果后续需要记录每次重试尝试，应先新增 attempt-level command 或可安全状态推进语义，再迁移 worker/recovery 路径。

## 5. 验收

- 商户拒单收付通退款 create 成功后记录 provider `wechat`、channel `ecommerce`、capability `ecommerce_refund`、command type `create_refund`、business owner `order`、external object type `refund`、external object key `out_refund_no`。
- 成功记录使用微信 `refund_id` 作为 secondary key，状态为 `accepted`。
- command 的 business object 指向本地 `refund_order`。
- 微信 API 失败分支不新增 command，不改变现有 pending + recovery 语义。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run TestProcessMerchantRejectRefund -count=1`
- Review 确认本段没有新增 fact/application 写入，没有终态推进，并且失败分支未锁死可重试 command 状态。