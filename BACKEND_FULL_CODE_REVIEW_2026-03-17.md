# 后端全量代码审查报告（当前状态）

> 审查时间：2026-03-17  
> 审查范围：`locallife/` Go 后端服务当前代码  
> 审查方法：静态审查 + 关键路径人工复核 + 构建级校验 `go test ./... -run '^$'`

## 结论摘要

- 当前仓库已经完成过一轮较系统的后端治理，旧报告中的多项高优问题已被修复。
- 本轮复核后，仍然确认存在 **5 项值得进入生产整改排期的问题**。
- 问题主要集中在：**调度器生命周期与防重入不一致**、**关键资金状态更新错误被吞没**、**若干异步旁路缺少背压或可靠补偿**。

严重级别定义：

- P1：高优先级，建议尽快修复
- P2：中优先级，应纳入最近一次稳定性治理
- P3：低优先级，作为工程一致性优化

---

## P1-1 调度器体系不一致，多个 scheduler 仍缺少防重入与 panic 恢复

### 证据

- `locallife/session/scheduler.go`
- `locallife/autotag/scheduler.go`
- `locallife/weather/scheduler.go`
- `locallife/worker/payment_recovery_scheduler.go`
- `locallife/worker/refund_recovery_scheduler.go`
- `locallife/worker/claim_recovery_scheduler.go`
- `locallife/worker/profit_sharing_recovery_scheduler.go`
- `locallife/worker/merchant_withdraw_recovery_scheduler.go`
- `locallife/worker/bill_reconciliation_scheduler.go`

以上调度器都直接使用 `cron.New()`，而未像 `scheduler/order_timeout.go`、`scheduler/data_cleanup.go`、`scheduler/takeout_auto_complete.go` 那样统一启用：

- `cron.SkipIfStillRunning(...)`
- `cron.Recover(...)`

### 问题说明

当前代码里已经存在两套调度器标准：

1. 一套是新的、生产友好的实现，具备防重入和 panic 保护。
2. 另一套仍是裸 `cron.New()`，没有串行保护，也没有统一恢复链。

这不是单个文件问题，而是整个调度体系的行为不一致。

### 风险

- 某次执行时间超过调度周期时，下一轮会直接并发进入，导致重复扫描、重复入队、重复状态迁移。
- 恢复类任务涉及支付、退款、分账、提现轮询，重入会放大幂等压力和下游噪音。
- 任一 job panic 时虽然未必导致主进程退出，但至少缺少统一的恢复和日志语义，故障定位成本更高。

### 建议

- 为所有 scheduler 统一引入 `cron.WithChain(cron.SkipIfStillRunning(...), cron.Recover(...))`。
- 将调度器构造逻辑收敛到一个统一 helper，避免后续继续出现“新旧两套标准”。
- 对涉及资金、恢复、补偿的 job，再补一层实例级互斥设计，避免多实例部署下重复执行。

---

## P1-2 多个调度器的“启动即执行一次”逻辑使用脱离生命周期的 goroutine

### 证据

- `locallife/session/scheduler.go`
- `locallife/autotag/scheduler.go`
- `locallife/weather/scheduler.go`
- `locallife/worker/payment_recovery_scheduler.go`
- `locallife/worker/refund_recovery_scheduler.go`
- `locallife/worker/claim_recovery_scheduler.go`
- `locallife/worker/profit_sharing_recovery_scheduler.go`
- `locallife/worker/merchant_withdraw_recovery_scheduler.go`

这些 `Start()` 方法里都包含类似逻辑：

```go
go s.runOnce()
```

或：

```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), ...)
    defer cancel()
    ...
}()
```

### 问题说明

这些 goroutine：

- 没有绑定到 `scheduler.Manager.StartAll()` 的关闭流程；
- `Stop()` 只调用 `cron.Stop()`，不会等待这些立即执行任务结束；
- 统一使用 `context.Background()`，不会在服务 shutdown 时被主动取消。

### 风险

- 服务启动阶段容易出现“cron 触发一次 + startup goroutine 再跑一次”的重叠窗口。
- 进程关闭时，任务可能执行到一半被硬切断，尤其是恢复、补偿、统计刷新类任务。
- 该模式在多个 scheduler 中重复出现，会使停机行为不可预测。

### 建议

- 禁止在 `Start()` 中直接 `go s.runOnce()`。
- 改为由 `scheduler.Manager` 统一调度一次性 warm-up 任务，或把首次执行也纳入受控 errgroup。
- 所有立即执行任务都应接受外层 `ctx`，并在 shutdown 时可取消、可等待。

---

## P1-3 资金链路中的状态回写错误被大量静默忽略，失败后可能留下不可追踪的中间态

### 证据

- `locallife/logic/refund_service.go`
- `locallife/worker/task_process_payment.go`
- `locallife/logic/reservation.go`
- `locallife/logic/reservation_dishes.go`
- `locallife/logic/replace_order.go`

代表性模式：

```go
_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
_, _ = s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
_, _ = s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
_, _ = s.store.UpdateProfitSharingReturnToFailed(ctx, ...)
```

### 问题说明

这些语句出现在退款、分账回退、退款结果处理等关键链路中。当前实现将状态推进视为“尽力而为”，但实际上这些状态本身就是后续补偿、审计、恢复调度的依据。

一旦数据库更新失败：

- 主流程可能已经返回失败或成功；
- 但内部状态未同步落库；
- 且没有明确日志、没有错误上抛、没有补偿记录。

### 风险

- 退款单、支付单、分账回退单可能卡在旧状态，后续恢复调度基于脏状态重复执行。
- 故障排查时只能看到“外部调用失败/成功”，却看不到本地状态推进是否完成。
- 这类问题直接影响资金链路的可追踪性和补偿准确性。

### 建议

- 对关键状态推进禁止使用 `_, _ = ...` 静默吞错。
- 至少记录结构化错误日志，携带 `refund_order_id`、`payment_order_id`、`return_id` 等键。
- 更理想的做法是将外部调用结果与本地状态回写放入统一补偿模型，确保“外部结果已知但本地状态未更新”可被扫描恢复。

---

## P2-1 搜索关键词记录仍采用每请求直接起 goroutine，缺少统一背压与关闭控制

### 证据

- `locallife/api/search.go`

当前实现：

```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    ...
}()
```

### 问题说明

搜索记录属于旁路统计，不阻塞主请求是合理的；但当前方案是每次请求直接起 goroutine，并在其中执行两次数据库写入。

该模式没有：

- 并发上限；
- 队列长度；
- 服务关闭时的 drain 机制。

### 风险

- 高并发搜索流量下会形成 goroutine 突刺。
- 数据库抖动时，后台 goroutine 会在 3 秒窗口内堆积。
- 进程退出时未完成的热词/历史写入直接丢失。

### 建议

- 迁移到有界 worker 池或复用现有任务分发体系。
- 若仍保留本地异步，至少引入带缓冲队列和 shutdown drain。
- 为该链路增加 dropped / failed 指标，避免“静默少写”。

---

## P2-2 图片删除队列满时直接丢弃任务，长期运行会积累脏文件

### 证据

- `locallife/api/upload_url.go`

当前逻辑：

```go
select {
case w.jobs <- path:
default:
    log.Warn().Str("path", path).Msg("image delete queue full, dropping delete job")
}
```

### 问题说明

这比旧实现“无限制开 goroutine”要好得多，但当前降级策略仍然是直接丢弃删除任务。对于上传频繁的生产系统，这意味着清理失败会永久转化为磁盘垃圾。

### 风险

- 长期积累无主图片文件，抬高磁盘占用。
- 问题只记 warn 日志，没有补偿队列，靠人工几乎无法回收。
- 一旦某段时间突发上传/替换操作，清理丢失会批量发生。

### 建议

- 队列满时不要直接丢弃，可落本地补偿表或延迟重试。
- 至少增加删除失败/丢弃数量的监控指标。
- 若磁盘空间是刚性资源，建议引入周期性 orphan file sweep 作为二次兜底。

---

## P2-3 审计写入在队列满时直接丢弃，和“审计日志”语义不完全匹配

### 证据

- `locallife/api/audit_writer.go`

当前逻辑：

```go
select {
case w.jobs <- auditJob{...}:
default:
    log.Warn().Str("action", input.Action).Msg("audit log queue full, dropping write")
}
```

### 问题说明

当前实现已经优于旧版的“每次请求起 goroutine”，但审计日志与普通埋点不同，它往往承担操作追踪、风控取证、合规留痕的角色。队列满即丢弃虽然保护了主链路，却削弱了审计的完整性。

### 风险

- 高峰期关键操作日志丢失，事后无法完整还原事件序列。
- 对外声称有审计能力，但在系统压力最大时恰好最容易缺日志。
- 仅日志告警而无计数与补偿，不利于运维感知。

### 建议

- 明确区分“业务埋点”和“审计日志”的可靠性级别。
- 对高价值审计事件考虑同步写库、事务 outbox 或单独高优先级队列。
- 至少为 dropped audit logs 增加指标和报警阈值。

---

## 已确认的正向结论

以下几项旧高风险问题，本轮复核确认已修复或已明显缓解：

- 生产环境 CORS 已改为显式白名单，并在启动阶段禁止空白与 `*`。
- 生产环境已要求 Redis 可用，资金流任务不再允许无队列降级。
- `order-timeout`、`data-cleanup`、`takeout-auto-complete` 三类核心 scheduler 已具备防重入与 panic 恢复。
- 审计写入已从“每请求 goroutine”升级为有界队列 + worker 池。
- 发货信息上报已迁移为 asynq 任务，而非直接在 handler 里裸异步执行。
- 运营实时统计参数解析已从宽松解析改为严格 `strconv.ParseInt`。
- 连接池参数已配置化，不再完全硬编码。

---

## 修复优先级建议

### 第一批（建议 1 周内处理）

1. 统一所有 scheduler 的构造方式，补齐 `SkipIfStillRunning` 和 `Recover`。
2. 去掉 `Start()` 内脱离生命周期的立即执行 goroutine，纳入统一 shutdown 管理。
3. 清理资金链路里所有关键状态推进的 `_, _ = ...`，至少做到可观测。

### 第二批（建议随后的稳定性冲刺处理）

1. 将搜索关键词记录改为有界异步模型。
2. 为图片删除和审计写入补齐 dropped 指标与补偿策略。

---

## 校验记录

- 已执行：`cd locallife && go test ./... -run '^$'`
- 结果：当前后端代码可编译通过。
