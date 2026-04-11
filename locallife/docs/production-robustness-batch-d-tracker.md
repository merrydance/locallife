# LocalLife 批次 D 修复执行跟踪

## 说明

- 本文用于跟踪批次 D 的实际推进，后续每一次进展都继续追加到本文。
- 批次 D 的来源文档：
  - [production-robustness-review-report.md](./production-robustness-review-report.md)
  - [production-robustness-p0-remediation-review.md](./production-robustness-p0-remediation-review.md)
- 本批次覆盖问题 13、14。

## 状态总览

| 任务 | 对应问题 | 当前状态 | 下一步 |
| --- | --- | --- | --- |
| D-1 | 问题 13：预订与预订加菜分账缺少恢复入口 | 已完成 | 进入后续批次 |
| D-2 | 问题 14：processing 分账恢复后立即短路 | 已完成 | 进入后续批次 |

## 固定实施顺序

1. D-1
2. D-2

说明：恢复入口和 processing 收敛必须一起修，否则把预订分账重新送回执行路径后仍会在 processing 上空转。

## 进展记录

### 2026-04-02 第 1 次推进

完成内容：

- 已重新核对批次 D 的真实代码路径，确认两个问题都集中在 `worker/profit_sharing_recovery_scheduler.go` 与 `worker/task_process_payment.go`。
- 已确认预订和 `reservation_addon` 支付成功时，生产路径本来就会按 `ReservationID` 入队分账任务，因此问题 13 不是“没有预订分账”，而是“恢复入口和失败重试仍只会回到 order_id 模型”。
- 已确认问题 14 的根因也仍然原样存在：恢复器会把 `processing` 分账单重新投回 `ProcessTaskProfitSharing`，但该处理器读到已有分账单状态是 `processing` 后会直接 `skip`，没有任何查询微信结果的动作。

当前决策：

- 统一引入“从 payment_order 推导分账恢复 payload”的通用逻辑，恢复器和失败重试都不再手写 `order_id` 假设，而是根据 `payment_order.order_id` / `payment_order.reservation_id` 自动构造正确 payload。
- 不新增任务类型，仍复用 `ProcessTaskProfitSharing`；但当已有分账单状态是 `processing` 时，处理器必须改为查询微信分账结果，再决定维持 processing、收敛 finished，或转为 failed。
- 查询结果落账后，继续复用现有 `ProcessTaskProfitSharingResult` 发送通知/告警，避免新造一套分账结果副作用路径。

原因：

- 这样可以用最小改动同时关闭“预订分账无法恢复”和“processing 恢复空转”两类缺口，并保持现有任务模型与告警模型不分叉。

### 2026-04-02 第 2 次推进

完成内容：

- 已在 `worker/task_process_payment.go` 引入统一的 `buildProfitSharingPayloadFromPaymentOrder`，当前失败重试与恢复调度都会按 `payment_order.order_id` / `payment_order.reservation_id` 自动构造 payload，不再把预订分账硬编码到 `order_id` 模型。
- 已在 `worker/profit_sharing_recovery_scheduler.go` 接通预订与 `reservation_addon` 的恢复入队路径；只要 `payment_order` 能定位到 `reservation_id`，恢复器就会重新投递正确的分账任务。
- 已在 `ProcessTaskProfitSharingResult` 的失败重试路径复用同一 payload 构造逻辑，因此预订分账回调失败后也不再因为缺少 `order_id` 而直接丢失重试入口。
- 已把 `ProcessTaskProfitSharing` 中已有分账单状态为 `processing` 的分支改成真实调用微信 `QueryProfitSharing`，不再直接 `skip`。
- 查询结果当前会按接收方结果与顶层状态收敛成 `SUCCESS` / `FAILED` / `PROCESSING` 三类：
  - 仍在处理中时保持原状态并退出；
  - 已完成时把本地分账单推进到 `finished`；
  - 已失败时把本地分账单推进到 `failed`。
- 查询收敛后仍复用现有 `ProcessTaskProfitSharingResult` 结果任务发送商户通知与失败告警，未引入第二套分账结果副作用链。

验证结果：

- 已执行 `gofmt -w worker/task_process_payment.go worker/profit_sharing_recovery_scheduler.go worker/profit_sharing_recovery_scheduler_test.go worker/task_process_payment_test.go`。
- 已执行 `go test ./worker -run 'TestProfitSharingRecoverySchedulerRunOnceEnqueuesReservationProfitSharingRetry|TestProcessTaskProfitSharing_ProcessingOrderQueriesAndFinishes|TestProcessTaskProfitSharingResult_FailedReenqueuesReservationPayload'`，通过。
- 已完成文件级错误检查，本轮涉及的 worker 代码与测试文件未发现新的编译或静态错误。

当前结论：

- 问题 13、14 可以视为已完成。
- 批次 D 已实现“预订分账可恢复 + processing 分账可查询收敛 + 失败后仍沿统一告警/通知链闭环”的最小生产修复面，不再存在原先“恢复器表面入队但预订链没有入口、processing 链只会空转”的结构性缺口。