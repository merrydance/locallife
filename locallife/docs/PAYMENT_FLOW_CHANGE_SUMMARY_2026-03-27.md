# Payment Flow 变更说明

日期：2026-03-27

关联文档：
- locallife/docs/PAYMENT_FLOW_REMEDIATION_PLAN_2026-03-26.md
- locallife/docs/PAYMENT_FLOW_IMPLEMENTATION_TASK_BREAKDOWN_2026-03-27.md

## 目标

本说明用于对齐本轮支付整改的实际落地结果，逐条对应审查发现、代码改动、自动化验证，以及仍需手工回归的边界项。

## 问题与落地结果

### 1. 回调归属校验缺失

已完成：
- 单笔支付回调增加直连 mchid/appid 校验。
- 单笔退款回调增加直连 mchid 校验。
- 收付通退款回调增加 sp_mchid 校验。
- 合单回调增加 combine_mchid/combine_appid 校验。
- 分账回调增加服务商 mchid 校验，并补齐 sub_mchid 与本地 merchant_payment_config 的一致性校验。
- 所有 ownership mismatch 分支统一返回 FAIL，并发送 critical 系统告警。

主要文件：
- locallife/api/payment_callback.go
- locallife/api/payment_callback_test.go
- locallife/wechat/payment.go
- locallife/wechat/interface.go

### 2. 通知认领后 release 失败会导致回调被吞

已完成：
- duplicate claim 不再一律直接返回 SUCCESS。
- 已 processed 的重复通知返回 SUCCESS。
- 未 processed 且仍在处理中返回 FAIL。
- stale claim 尝试释放占位并返回 FAIL。
- release 失败统一发送 critical 告警。
- 成功处理出口统一写入 processed_at。
- 新增 stale notification recovery scheduler，定时释放超时未 processed 的通知占位，兜底恢复后续微信重试。

主要文件：
- locallife/api/payment_callback.go
- locallife/db/sqlc/tx_notification.go
- locallife/db/query/wechat_notification.sql
- locallife/worker/wechat_notification_recovery_scheduler.go
- locallife/main.go

### 3. 本地查不到支付单却返回 SUCCESS

已完成：
- 单笔支付回调 not found 改为 FAIL + release + critical 告警。
- 合单子单 not found 改为 FAIL，并告警。
- 合单主单 not found 改为 FAIL，并告警。

主要文件：
- locallife/api/payment_callback.go
- locallife/api/payment_callback_test.go

### 4. 合单异常子单仍可能推动主单 paid

已完成：
- closed/failed 子单不再计入主合单成功数。
- 金额异常子单不再计入主合单成功数。
- 仅当所有子单都是真正成功时才更新主合单 paid。
- 当存在异常子单时，主合单保持不推进 paid，并输出告警信号。

主要文件：
- locallife/api/payment_callback.go
- locallife/api/payment_callback_test.go

### 5. 金额异常与恢复调度误推进

已完成：
- 金额异常回调先创建或复用 refund_order，再记录 paid，再尝试自动退款。
- payment recovery scheduler 在 SQL 和运行时两层排除已有 refund activity 的支付单。
- refund_order 创建保持幂等复用，避免回调重试或任务重试产生重复退款申请。

主要文件：
- locallife/api/payment_callback.go
- locallife/db/query/payment_order.sql
- locallife/worker/payment_recovery_scheduler.go
- locallife/worker/payment_recovery_scheduler_test.go

### 6. 无 OrderID 业务无法自动退款闭环

已完成：
- membership_recharge 金额异常退款已接入收付通退款链路。
- rider_deposit 金额异常退款已接入直连退款链路。
- 退款请求 total_amount 统一使用 max(payment_amount, refund_amount)，避免 overpay 场景被微信拒绝。

主要文件：
- locallife/worker/task_process_payment.go
- locallife/worker/task_process_payment_mismatch_test.go

## 自动化验证

已执行的聚焦验证包括：
- go test ./api -run 'TestHandle(PaymentNotify(Idempotency|FullFlow)|RefundNotify(Idempotency|OwnershipMismatch)|CombinePaymentNotify(Idempotency|_ClosedOrderEnqueuesAnomalyRefund|_OwnershipMismatchReturnsFail|_SubOrderNotFoundReturnsFail|_AmountMismatchEnqueuesRefund|_MainOrderNotFoundReturnsFail)|EcommerceRefundNotify(Idempotency|_OwnershipMismatchReturnsFail)|ApplymentStateNotifyIdempotency|ProfitSharingNotifyIdempotency)$'
- go test ./worker -run 'TestWechatNotificationRecoverySchedulerRunOnce'
- worker 金额异常退款与 payment recovery 相关测试
- db/sqlc wechat notification 相关测试

## 已确认需要的生成步骤

本轮已执行：
- make sqlc
- make mock

本轮不需要：
- make swagger

## 仍需手工回归

以下属于计划中的发布前手工检查，当前未由自动化完全覆盖：
- 正常支付成功。
- closed 或 failed 后迟到回调。
- 金额异常自动退款。
- 回调找不到本地支付单。
- 错租户 combine_mchid/sub_mchid/sp_mchid 回调不得推进任何本地状态。
- release 失败后重复回调，应可再次进入处理或被补偿任务恢复。

## 范围说明

本轮没有扩展到以下范围外事项：
- 普通用户主动退款的大规模重构。
- 支付表结构新增业务状态。
- 非支付领域的前端或运营后台改造。