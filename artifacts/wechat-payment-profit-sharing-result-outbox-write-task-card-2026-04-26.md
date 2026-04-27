# TASK-PAY-007H profit sharing result outbox 双写任务卡

日期：2026-04-26

状态：历史阶段。已由 TASK-PAY-007M 直接切换到 outbox 执行面；旧结果任务不再作为生产 fallback。

## 1. 目标

在 payment fact application 成功应用分账终态后，持久化写入 `profit_sharing_result_ready` payment domain outbox，为后续把分账结果通知迁移到 outbox dispatcher 做准备。

本段只做 durable outbox 双写，不让 dispatcher 消费真实事件，也不替换旧 `payment:process_profit_sharing_result` 任务。

## 2. 范围

- 为 `payment_domain_outbox` 增加 `(event_type, aggregate_type, aggregate_id)` 唯一索引，明确同一业务聚合的同类事件只写一条。
- 新增 `CreatePaymentDomainOutboxOnce` sqlc 查询，重复写入时返回既有 outbox，不覆盖既有 payload 或状态。
- 在 `ApplyExternalPaymentFactApplication` 应用 `profit_sharing_domain/profit_sharing_order` 终态后，写入 `profit_sharing_result_ready` outbox。
- outbox payload 复用旧结果任务需要的核心字段：分账单 ID、商户分账单号、结果、失败原因、商户 ID，并附带 fact/application ID 便于追踪。
- outbox 写入失败时，fact application 标记 failed 并按既有 retry 机制重试。

## 3. 不在本段处理

- 不让 `PaymentDomainOutboxScheduler` 扫描 `profit_sharing_result_ready`。
- 不让 dispatcher 发送商户通知或运营告警。
- 不删除 callback/query 当前旧 `DistributeTaskProcessProfitSharingResult` 入队。
- 不改变 `ProcessTaskProfitSharingResult` 的通知、告警和重试逻辑。

## 4. 验收

- 分账 success fact application 成功应用后会创建 pending `profit_sharing_result_ready` outbox。
- 分账 failed/closed fact application 成功应用后会创建 pending `profit_sharing_result_ready` outbox，并保留失败原因。
- outbox 写入失败会使 application 进入 failed retry，而不是把 application 标记 applied。
- 重复写同一 event/aggregate 时返回原 outbox，不重复创建、不覆盖 payload/status。

## 5. 验证

- `make -C /home/sam/locallife/locallife sqlc`
- `go -C /home/sam/locallife/locallife test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication' -count=1`
- `go -C /home/sam/locallife/locallife test ./db/sqlc -run 'TestPaymentDomainOutbox|TestExternalPaymentFactApplication|TestCreatePaymentDomainOutboxOnce' -count=1`

## 6. Review 结论

风险等级：G3。原因是该 outbox 后续会承接分账结果通知发布链路。本段仍保持旁路双写，不消费真实事件、不切换旧通知路径，因此不会改变当前用户可见通知行为。幂等边界由唯一索引和 `CreatePaymentDomainOutboxOnce` 支撑，避免 application 重试导致重复 outbox。