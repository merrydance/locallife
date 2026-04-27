# TASK-PAY-007G payment domain outbox dispatcher 骨架任务卡

日期：2026-04-26

## 1. 目标

建立 `payment_domain_outbox` 的最小运行时 dispatcher 骨架，验证 outbox 可以从 scheduler 入队、worker claim、处理成功后标记 published、处理失败后标记 failed 并等待持久化重试。

本段只处理 probe event，不接入分账结果通知事件。

## 2. 范围

- 新增 `payment:process_domain_outbox` asynq task。
- 新增 `PaymentDomainOutboxTaskDistributor` 窄接口和 Redis 实现。
- 新增 `PaymentDomainOutboxScheduler`，每分钟只扫描 `payment_domain_outbox_dispatcher_probe` event。
- 新增 sqlc 查询 `ListPendingPaymentDomainOutboxByEventType`，在 SQL 层按 event type 过滤后再 LIMIT，避免共享 outbox 固定批次扫描饥饿。
- worker claim outbox 后，只支持 probe event；probe 成功后标记 `published`。
- unsupported event 会标记 `failed + next_retry_at`，但 scheduler 不会主动扫描这些事件。
- 在 `main.go` 中仅当 distributor 支持窄接口时注册 scheduler。

## 3. 不在本段处理

- 不写入 `profit_sharing_result_ready` outbox。
- 不让 dispatcher 发送商户通知或运营告警。
- 不替换 `DistributeTaskProcessProfitSharingResult`。
- 不扫描所有 outbox event；本段仅扫描 probe event。
- 不新增 schema migration。

## 4. 验收

- probe outbox task 可以 claim 后标记 published。
- unsupported event task 会标记 failed，并设置 retry 时间。
- scheduler 只查询 probe event type，并只入队返回的 probe outbox ID。
- missing/不可 claim outbox 不会触发重试风暴。

## 5. 验证

- `make -C /home/sam/locallife/locallife sqlc`
- `go -C /home/sam/locallife/locallife test ./worker -run 'TestProcessTaskPaymentDomainOutbox|TestPaymentDomainOutboxScheduler' -count=1`
- `go -C /home/sam/locallife/locallife test ./db/sqlc -run 'TestPaymentDomainOutbox' -count=1`

## 6. Review 结论

风险等级：G3。原因是 outbox dispatcher 后续会承载支付/分账通知迁移。本段故意限制为 probe-only，不消费真实业务事件，因此不会改变现有分账通知运行时行为。