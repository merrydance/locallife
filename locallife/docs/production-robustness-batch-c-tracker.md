# LocalLife 批次 C 修复执行跟踪

## 说明

- 本文用于跟踪批次 C 的实际推进，后续每一次进展都继续追加到本文。
- 批次 C 的来源文档：
  - [production-robustness-review-report.md](./production-robustness-review-report.md)
  - [production-robustness-p0-remediation-review.md](./production-robustness-p0-remediation-review.md)
- 本批次覆盖问题 10、11。

## 状态总览

| 任务 | 对应问题 | 当前状态 | 下一步 |
| --- | --- | --- | --- |
| C-1 | 问题 10：预订改菜退款错误压到单一原始支付单 | 已完成 | 进入后续批次 |
| C-2 | 问题 11：预订改菜退款缺少完整异步闭环 | 已完成 | 进入后续批次 |

## 固定实施顺序

1. C-1
2. C-2

说明：问题 11 的异步闭环必须建立在问题 10 的退款对象正确之上，因此不拆开推进。

## 进展记录

### 2026-04-02 第 1 次推进

完成内容：

- 已重新核对问题 10、11 的真实代码路径，确认当前缺口集中在 `logic/reservation_dishes.go`、`worker/task_process_payment.go`、`worker/refund_recovery_scheduler.go`。
- 已确认预订取消退款和主退款链已经具备可复用的模式：`CreateRefundOrderTx` 负责退款额度占用与建单，`ProcessTaskRefundResult` 负责最终状态落账，恢复调度负责把 pending 退款重新送回执行路径。
- 已确认预订改菜退款之所以失真，不是单点 bug，而是两层问题叠加：
  - 退款对象仍只取单一 `business_type=reservation` 支付单，忽略 `reservation_addon`。
  - 退款发起和预付余额回写仍停留在同步分支，没有真正接入统一异步收敛。

当前决策：

- 退款归因采用“按真实已支付交易集合分摊”的方案，不再把总预付余额直接压回原始预订支付单。
- 分摊顺序按当前支付单创建顺序逆序回退，优先冲销较新的 `reservation_addon` 入账，再回退到原始 `reservation` 入账，避免把 addon 余额继续错误映射到首单。
- 改菜退款改为先建退款单、再走统一退款任务；同步接口不再把微信退款成功作为唯一完成条件。
- 预订预付余额的扣减收口到“退款成功落账”阶段，避免只在同步 success 分支单点修改 `prepaid_amount`。

原因：

- 这样可以一次性关闭“退款对象错误”和“异步结果不回写余额”两类缺口，并保持与现有退款状态机一致。

### 2026-04-02 第 2 次推进

完成内容：

- 已在 `logic/reservation_dishes.go` 引入“按真实支付交易集合分摊退款”的实现，退款规划不再强制只取单一 `business_type=reservation` 支付单。
- 当前实现会按预订支付单创建顺序逆序分摊退款额度：优先冲销较新的 `reservation_addon` 入账，再回退到原始 `reservation` 入账；每笔支付单还会扣除已占用的 `pending/processing/success` 退款额度。
- 改菜退款不再在接口层直接调用微信退款并同步修改 `prepaid_amount`；现在会先创建对应的退款单，再把每笔退款计划送入统一 `ProcessRefund` 任务入口。
- 已扩展退款任务载荷，支持显式透传 `reservation_id` 与 `out_refund_no`，让改菜退款可以复用已创建的退款单号进入 worker 幂等闭环，而不再被旧的“`payment_order_id + reservation_id` 唯一退款单号”模型限制。
- 已在 `worker/task_process_payment.go` 为预订类支付成功退款新增统一成功收敛 helper：只有退款单从 `pending/processing` 真正推进到 `success` 时，才会扣减 `reservation.prepaid_amount` 并按累计退款额决定是否把支付单标记为 `refunded`。

当前判断：

- 问题 10 的根因已经关闭：退款请求对象改为真实支付单集合，不再把 addon 入账错误映射到原始预订首单。
- 问题 11 的主闭环也已建立：改菜退款的发起、成功落账、预付余额回写现在共享同一条异步状态机。

### 2026-04-02 第 3 次推进

完成内容：

- 已在 `db/query/refund_order.sql` 增加 pending 预订退款恢复查询，用于捞起 `reservation` 与 `reservation_addon` 两类仍停留在 `pending` 的退款单。
- 已在 `worker/refund_recovery_scheduler.go` 新增对应恢复分支：当预订退款单在创建后未被及时发起或首次发起失败时，调度器会按原始 `out_refund_no` 重新投递 `ProcessRefund` 任务，而不是让 pending 退款单长期沉底。
- 已保持旧的预订取消退款路径兼容，新恢复逻辑不会要求改动既有 API 契约；如果同步入队失败，系统仍能依靠 pending 退款恢复分支继续推进。

验证结果：

- 已执行 `make sqlc`，新的退款恢复查询、Store 接口和 mock 已同步更新。
- 已新增 `logic/reservation_dishes_test.go`，验证改菜退款会优先冲销 `reservation_addon` 支付，并在 addon 不足时继续回退到原始预订支付单。
- 已新增 `worker/task_process_payment_reservation_refund_test.go`，覆盖：
  - 改菜退款 worker 会复用调用方提供的 `out_refund_no` 发起微信退款；
  - 退款成功回调会把预订 `prepaid_amount` 按成功退款额扣减，并在累计退款额达到支付金额时把支付单标记为 `refunded`。
- 已执行以下定向验证，结果通过：
  - `go test ./logic -run 'TestBuildReservationRefundAllocations_SplitsAcrossReservationPayments'`
  - `go test ./worker -run 'TestProcessTask(InitiateRefund_ReservationAddonRefund_UsesProvidedOutRefundNo|RefundResult_ReservationRefundSuccess_UpdatesPrepaidAmount)'`
  - `go test ./api -run 'TestCreateReservationAPI'`
  - `go test ./worker -run 'TestDoesNotExist'`（用于编译整个 worker 包）

当前结论：

- 问题 10、11 可以视为已完成。
- 批次 C 已实现“多支付单正确归因 + 统一异步发起 + 成功后余额回写 + pending 恢复补偿”的完整闭环，不再依赖同步 success 单点路径维持预订账务一致性。