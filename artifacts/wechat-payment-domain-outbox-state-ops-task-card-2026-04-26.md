# TASK-PAY-007F payment domain outbox 状态操作任务卡

日期：2026-04-26

## 1. 目标

补齐 `payment_domain_outbox` 的最小发布状态机持久化操作，为后续把分账结果通知从旧 callback/query 触发迁移到 application/outbox 驱动做准备。

本段只补持久层能力，不接 runtime dispatcher，不发布通知，不替换旧分账结果任务。

## 2. 范围

- 新增 `ClaimPaymentDomainOutbox`，只 claim retryable 的 `pending/failed` outbox，claim 后进入 `processing` 并递增 `attempt_count`。
- 新增 `MarkPaymentDomainOutboxPublished`，只允许 `processing -> published`。
- 新增 `MarkPaymentDomainOutboxFailed`，只允许 `processing -> failed`，记录 `last_error` 和 `next_retry_at`。
- 补充 sqlc 持久层生命周期测试。

## 3. 不在本段处理

- 不新增 outbox scheduler/worker。
- 不把 `ProcessTaskProfitSharingResult` 改为 outbox dispatcher。
- 不改 callback/query 当前旧状态写入和旧结果通知链路。
- 不新增 migration；本段复用 000217 已存在的 `payment_domain_outbox` 表和索引。

## 4. 验收

- pending outbox 可以被 claim，重复 claim processing 记录会失败。
- failed outbox 未到 `next_retry_at` 前不能被 claim，到期后可以重新 claim。
- processing outbox 可以标记 published，且会清空 retry/error 字段。
- published outbox 不能再标记 failed。

## 5. 验证

- `make -C /home/sam/locallife/locallife sqlc`
- `go -C /home/sam/locallife/locallife test ./db/sqlc -run 'TestPaymentDomainOutbox' -count=1`

## 6. Review 结论

风险等级：G3。原因是该 outbox 后续会承载支付/分账通知迁移的恢复语义。本段只增加持久化状态操作和测试，没有接入生产调度或通知发布，因此运行时行为不变。