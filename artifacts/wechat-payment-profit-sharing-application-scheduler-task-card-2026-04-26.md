# TASK-PAY-007D 分账 fact application scheduler 任务卡

日期：2026-04-26

状态：历史阶段。后续切换完成后，该 scheduler 成为分账 fact application 的默认恢复入口。

## 1. 目标

把 007C 的显式 `payment:process_fact_application` 任务接入后台恢复扫描，让 `external_payment_fact_applications` 中 retryable 的分账 application 可以自动入队处理。

本段只做 application task 入队，不直接在 scheduler 中应用业务状态，也不替换 callback/query 的现有同步写路径。

## 2. 范围

- 新增 `PaymentFactApplicationTaskDistributor` 窄接口，避免扩大现有 `TaskDistributor` 和 mock 生成面。
- `RedisTaskDistributor` 支持 `DistributeTaskProcessPaymentFactApplication`。
- 新增 `PaymentFactApplicationScheduler`，每分钟扫描 retryable application。
- 新增 sqlc 查询 `ListRetryableExternalPaymentFactApplicationsByTarget`，从数据库层只扫描 `consumer=profit_sharing_domain` 且 `business_object_type=profit_sharing_order` 的 application，避免跨 consumer 扫描后内存过滤导致分账 application 饥饿。
- 入队使用 `QueueCritical`、`MaxRetry(5)`、短窗口 `Unique` 去重。
- 在 `main.go` 中仅当 task distributor 支持该窄接口时注册 scheduler。

## 3. 不在本段处理

- 不迁移旧 callback/query 对 `profit_sharing_orders` 的同步状态更新。
- 不迁移 `ProcessTaskProfitSharingResult` 通知链路。
- 不处理 rider deposit、direct refund、profit sharing return 或 subsidy 的 application。
- 不改 `ListRetryableExternalPaymentFactApplications` 的跨 consumer 查询语义；本段只新增分账 scheduler 专用 target 过滤查询。

## 4. 验收

- scheduler 能扫描 retryable application 并只入队分账 application。
- 非分账 consumer 不会被 scheduler 查询到，也不会送入分账 consumer core 后标记失败。
- Redis distributor 支持显式 application task 入队。
- Redis 不可用或 distributor 不支持窄接口时，主程序不会注册该 scheduler。
- 已运行 `make sqlc` 生成 sqlc 代码和 mocks。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./worker -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication|TestProcessTaskPaymentFactApplication|TestPaymentFactApplicationScheduler' -count=1`
- `go -C /home/sam/locallife/locallife test ./logic ./worker -count=1`
- `make -C /home/sam/locallife/locallife sqlc`

## 6. Review 结论

风险等级：G3。原因是本段让 payment fact application 从纯手工入口进入后台恢复链路，触及支付事实消费、worker 入队、重复执行和恢复节奏。

当前 scheduler 继续只负责入队；但旧 callback/query 的同步状态更新已移除，`profit_sharing_orders` 由 fact application consumer 单源推进。结果通知和失败告警由后续 outbox 执行面负责。