# TASK-PAY-007E 分账 fact application 即时入队任务卡

日期：2026-04-26

## 1. 目标

在 callback/query 已经写入 terminal payment fact application 后，立即 best-effort 入队 `payment:process_fact_application`，让事实应用不只依赖 007D 的分钟级 scheduler。

本段保持旧 callback/query 状态更新和 `ProcessTaskProfitSharingResult` 通知链路不变，避免在通知持久化边界尚未完成前引入丢通知或重复通知风险。

## 2. 范围

- `recordProfitSharingCallbackFact` 返回创建或复用的 `ExternalPaymentFactApplication`。
- `recordProfitSharingQueryFact` 返回创建或复用的 `ExternalPaymentFactApplication`。
- callback 在 fact/application 写入成功且旧本地状态写入完成后，尝试通过 `PaymentFactApplicationTaskDistributor` 入队 application task，避免 application worker 抢先推进状态导致 callback 条件更新失败。
- query recovery 在 fact/application 写入成功且旧本地状态写入完成后，尝试通过 `PaymentFactApplicationTaskDistributor` 入队 application task。
- 入队失败只记录 warn；007D scheduler 仍会扫描 pending/failed application 作为恢复兜底。
- 入队使用 `QueueCritical`、`MaxRetry(5)` 和短窗口 `Unique`。

## 3. 不在本段处理

- 不把商户到账通知或分账失败告警搬进 `ProcessTaskPaymentFactApplication`。
- 不移除旧 callback/query 对 `profit_sharing_orders` 的同步状态写入。
- 不移除旧 `DistributeTaskProcessProfitSharingResult` 入队。
- 不新增 outbox dispatcher 或通知 exactly-once 机制。
- 不扩大 `TaskDistributor` 主接口；仍通过 `PaymentFactApplicationTaskDistributor` 窄接口探测能力。

## 4. 验收

- terminal callback fact 写入 application 且旧状态写入成功后，会在 Redis distributor 支持窄接口时立即入队 application task。
- terminal query fact 写入 application 且旧状态写入成功后，会在 Redis distributor 支持窄接口时立即入队 application task。
- mock/noop distributor 不支持窄接口时，旧状态写入和旧结果通知链路保持原行为。
- application task 入队失败不影响微信回调旧闭环，由 scheduler 后续补偿。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./api ./worker -run 'TestHandleProfitSharingNotify|TestProcessTaskProfitSharing|TestPaymentFactApplicationScheduler|TestProcessTaskPaymentFactApplication' -count=1`
- `go -C /home/sam/locallife/locallife test ./logic ./worker -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication|TestProcessTaskPaymentFactApplication|TestPaymentFactApplicationScheduler' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及分账 callback/query、payment fact application 入队、异步恢复和旧通知链路共存。

本段是迁移前置步骤，而不是最终单源状态/通知切换。下一段如要让 application consumer 接管结果通知，必须先落地持久化通知或 outbox publish 标记，避免 application 已 applied 但结果任务入队失败后无法重试的窗口。