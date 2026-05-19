# 宝付超时关单修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复系统超时取消链路，使宝付聚合支付单在超时场景下必须先走宝付查询与关单，失败不允许本地降级为已关闭或已取消，并向前端返回语义明确的稳定提示。

**Architecture:** 超时关闭分成两条链路：`payment_order:timeout` 负责支付单级别的远端查询、关单、本地关闭和业务订单取消；`order:payment_timeout` 只负责普通订单历史兜底，不能绕过支付单关单。宝付聚合支付要复用 `locallife/logic/baofu_payment_service.go` 现有的 `CloseOrder` 能力，但 worker 必须先查询远端状态，再决定是否允许本地状态推进；任何远端错误都要返回任务错误并落结构化日志，不能吞错或本地降级。

**Tech Stack:** Go, asynq workers, Gin API, PostgreSQL/sqlc, zerolog structured logging, existing Baofoo/Baofu aggregate payment client and contract tests.

---

## 0. Scope, Boundary, And Non-Negotiables

- 风险等级：G3。原因：这是支付与订单状态机耦合的超时关单链路，涉及外部支付单关闭、内部订单取消、幂等重试和前端可见状态。
- 只修这三个边界：
  1. 宝付聚合支付单超时关闭链路。
  2. 旧 `order:payment_timeout` 绕行入口。
  3. 前端可见的支付状态提示。
- 明确不做的事：
  - 不把宝付查询失败/关单失败降级成本地关闭成功。
  - 不把宝付 `ORDER_NOT_EXIST`、`SYSTEM_BUSY` 之类吞掉当成成功。
  - 不改退款、分账、提现、开户、报备、回调协议。
- 不变量：
  - 本地 `UpdatePaymentOrderToClosed` 之前必须有远端宝付查询或关单证据。
  - 业务订单 `CancelOrderTx` 之前必须确认支付单已经被宝付受理关单，或远端状态已经是明确终态。
  - 任何远端错误都必须可观测：worker 返回 error，asynq 统一记录任务错误，日志必须带 `payment_order_id`、`out_trade_no`、`operation`。
  - 前端只接稳定中文提示，不接宝付原始错误文本。

---

## 1. Current-State Reading Checklist

在动代码前，执行者必须先看完这些文件并确认当前行为：

- `locallife/worker/task_payment_timeout.go`
- `locallife/worker/task_order_timeout.go`
- `locallife/logic/baofu_payment_service.go`
- `locallife/logic/baofu_payment_service_test.go`
- `locallife/worker/baofu_payment_recovery_scheduler.go`
- `locallife/worker/baofu_payment_recovery_scheduler_test.go`
- `locallife/api/payment_order.go`
- `locallife/api/server.go`
- `.github/standards/domains/baofu-payment/README.md`
- `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md`
- `.github/standards/backend/ERROR_HANDLING.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`

当前已确认的事实：

- `locallife/logic/baofu_payment_service.go:276-312` 已有宝付 `CloseOrder` 封装，会记录 `close_payment` 外部命令并调用 `aggregatepay.Client.CloseOrder`。
- `locallife/worker/task_payment_timeout.go:157-405` 目前只把 `ecommerce`、`ordinary_service_provider`、`direct` 纳入远端查询/关单；宝付聚合支付没有分支，会落到 `required=false`，随后直接本地关闭支付单并取消业务订单。
- `locallife/worker/task_order_timeout.go:94-103` 旧订单超时任务会直接 `CancelOrderTx`，没有先检查是否存在宝付聚合支付单。
- `locallife/api/payment_order.go:450-456` 新支付单超时任务已经按支付单维度调度，说明正确入口是 `payment_order:timeout`，不是旧订单超时任务。
- `locallife/api/server.go:1797-1817` 已有 `internalError` / `loggedServerError`，可以满足“所有错误都落日志，前端只看稳定提示”的要求。

---

## 2. File Map

### 2.1 Core Worker Path

- Modify: `locallife/worker/task_payment_timeout.go`
- Test: `locallife/worker/task_payment_timeout_test.go`

### 2.2 Legacy Bypass Path

- Modify: `locallife/worker/task_order_timeout.go`
- Test: `locallife/worker/task_order_timeout_test.go`

### 2.3 Baofu Service Boundary

- Modify: `locallife/logic/baofu_payment_service.go`
- Test: `locallife/logic/baofu_payment_service_test.go`
- Optional test support: `locallife/worker/baofu_payment_recovery_scheduler_test.go` if query-state reuse needs a shared fixture.

### 2.4 API Message Surface

- Modify: `locallife/api/payment_order.go`
- Modify: `locallife/api/server.go` only if a new helper is needed for a stable message field or response shape.
- Test: `locallife/api/payment_order_test.go`

---

## 3. Task 1: Add Failing Tests For Baofu Timeout Behavior

**Files:**
- Modify: `locallife/worker/task_payment_timeout_test.go`
- Modify: `locallife/logic/baofu_payment_service_test.go`
- Modify: `locallife/worker/task_order_timeout_test.go`
- Modify: `locallife/api/payment_order_test.go`

- [ ] **Step 1: Write the failing worker test for Baofu `WAIT_PAYING`**

Add a test named `TestProcessTaskPaymentOrderTimeout_BaofuWaitPayingClosesRemoteBeforeLocalCancel` in `locallife/worker/task_payment_timeout_test.go`.

Use a `baofuRecoveryAggregateClient`-style fake or a dedicated fake implementing `aggregatepay.Client` with fields that capture both `QueryPayment` and `CloseOrder` calls.

Expected setup and assertions:

```go
paymentOrder := db.PaymentOrder{
    ID:             9010,
    OutTradeNo:     "BF_TIMEOUT_WAIT_1",
    Amount:         12345,
    Status:         "pending",
    PaymentChannel: db.PaymentChannelBaofuAggregate,
    BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
    OrderID:        pgtype.Int8{Int64: 91010, Valid: true},
    TransactionID:  pgtype.Text{String: "BFTX_9010", Valid: true},
    Attach:         pgtype.Text{String: "sub_mchid:1900000112", Valid: true},
    ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
}
```

The fake must return a query response equivalent to `WAIT_PAYING` with matching amount. The test must expect, in order:

1. `QueryPayment(ctx, req)` with `MerchantID == "COLLECT_MER"`, `TerminalID == "COLLECT_TER"`, and `TradeNo == "BFTX_9010"`.
2. `CloseOrder(ctx, req)` with the same merchant/terminal and `OutTradeNo == "BF_TIMEOUT_WAIT_1"`.
3. `UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)`.
4. `GetOrderForUpdate(ctx, 91010)` and `CancelOrderTx(...)`.

The test must fail before code change because the current worker never routes Baofu to a closeable remote branch.

- [ ] **Step 2: Run the test and confirm the failure mode**

Run:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskPaymentOrderTimeout_BaofuWaitPayingClosesRemoteBeforeLocalCancel' -v
```

Expected: fail because `QueryPayment` / `CloseOrder` are never invoked or because the worker exits through the wrong branch.

- [ ] **Step 3: Write the failing worker test for Baofu `SUCCESS`**

Add `TestProcessTaskPaymentOrderTimeout_BaofuSuccessRecordsFactInsteadOfClosing`.

Use a query result with `TxnState == aggregatecontracts.PaymentStateSuccess`, matching amount, and a non-empty `TradeNo`.

Expected assertions:

1. `QueryPayment` is called.
2. `CloseOrder` is not called.
3. `UpdatePaymentOrderToPaid` is called.
4. `CreateExternalPaymentFact` is called with provider `baofu`, channel `baofu_aggregate`, capability `baofu_payment`, source `query` or manual reconciliation depending on the final implementation path.
5. `CancelOrderTx` is not called.

- [ ] **Step 4: Run the test and confirm the failure mode**

Run:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskPaymentOrderTimeout_BaofuSuccessRecordsFactInsteadOfClosing' -v
```

Expected: fail because Baofu has no dedicated query branch yet.

- [ ] **Step 5: Write the failing worker test for Baofu remote error**

Add `TestProcessTaskPaymentOrderTimeout_BaofuQueryErrorStopsLocalClose`.

Fake `QueryPayment` should return a provider error or plain error such as `errors.New("baofu payment query failed")`.

Expected assertions:

1. No `UpdatePaymentOrderToClosed`.
2. No `CancelOrderTx`.
3. Worker logs the query failure with `payment_order_id`, `out_trade_no`, and operation context.
4. Worker returns error.
5. Error message wraps context such as `query baofu payment order before timeout close`.

- [ ] **Step 6: Run the test and confirm it fails before implementation**

Run:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskPaymentOrderTimeout_BaofuQueryErrorStopsLocalClose' -v
```

Expected: fail because current code does not branch to Baofu-specific remote handling.

- [ ] **Step 7: Write the failing legacy bypass test**

Add `TestProcessTaskOrderPaymentTimeout_DelegatesPendingBaofuPaymentOrder` in `locallife/worker/task_order_timeout_test.go`.

The test must set up an order with a linked `pending` Baofu aggregate payment order and assert this single allowed outcome:

- `order:payment_timeout` must delegate to the payment-order timeout path for that payment order.

Required negative assertions:

- It must not directly call `CancelOrderTx` on the business order while the linked Baofu payment order is still pending.
- It must not return `nil` without either delegating or processing the payment-order timeout branch.

- [ ] **Step 8: Run the test and confirm failure mode**

Run:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskOrderPaymentTimeout_DelegatesPendingBaofuPaymentOrder' -v
```

Expected: fail because the legacy task still directly cancels orders.

- [ ] **Step 9: Add the API-side regression for message shape**

Add or extend a payment-order query or close API test in `locallife/api/payment_order_test.go` so that a Baofu-related provider error is surfaced as a stable Chinese message and not raw provider text.

Expected assertions:

- response status is `502` or `503` depending on the final mapping,
- `error` field contains a stable Chinese sentence such as `宝付支付状态查询失败，请稍后刷新支付状态` or `支付状态暂不可确认，请稍后重试`,
- raw `baofu` upstream text is not present.

---

## 4. Task 2: Implement Baofu Timeout Query And Close Branch

**Files:**
- Modify: `locallife/worker/task_payment_timeout.go`
- Modify: `locallife/logic/baofu_payment_service.go`
- Test: `locallife/worker/task_payment_timeout_test.go`
- Test: `locallife/logic/baofu_payment_service_test.go`

- [ ] **Step 1: Add a Baofu remote-close discriminator to the worker state**

Extend `paymentOrderTimeoutRemoteClose` in `locallife/worker/task_payment_timeout.go` with a `baofu bool` flag.

Keep the existing flags for `direct` and `ordinary`; do not merge all providers into one generic helper. The final shape should still make the remote side effect obvious at call sites.

- [ ] **Step 2: Add `prepareBaofuPaymentOrderTimeoutClose`**

Implement a new helper that mirrors the existing ecommerce/direct/ordinary helpers:

```go
func (p *RedisTaskProcessor) prepareBaofuPaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error)
```

Required behavior:

1. Fail fast if `p.baofuAggregateClient == nil`.
2. Fail fast if `p.baofuProfitSharingConfig.CollectMerchantID` or `CollectTerminalID` is blank.
3. Resolve the query reference using the same rule as recovery scheduler and service layer:
   - use `TransactionID` as `tradeNo` when present,
   - otherwise use `OutTradeNo`.
4. Call `p.baofuAggregateClient.QueryPayment(ctx, req)`.
5. If query returns nil, return error.
6. Pass the result into a dedicated `handleBaofuPaymentTimeoutQueryResult`.
7. Return `paymentOrderTimeoutRemoteClose{required: true, baofu: true}` only when the remote state allows proceeding to close.

Do not invent a new provider abstraction. Keep the helper local to the payment timeout worker.

- [ ] **Step 3: Add `handleBaofuPaymentTimeoutQueryResult`**

This helper must use `aggregatecontracts.NormalizePaymentTerminalStatus` and follow Baofoo contract semantics.

Required branches:

```go
switch tradeState {
case aggregatecontracts.PaymentStateSuccess:
    // if amount mismatch: log alert, stop automatic close
    // if amount matches: UpdatePaymentOrderToPaid, record fact, enqueue application, return stop=true
case aggregatecontracts.PaymentStateWaitPaying, aggregatecontracts.PaymentStateClosed, aggregatecontracts.PaymentStatePayError:
    return false, nil
case aggregatecontracts.PaymentStateAbnormal:
    publish alert, return true, nil
default:
    publish alert, return true, nil
}
```

Important:

- `SUCCESS` must not fall through to local close.
- `WAIT_PAYING` is the only clear candidate for remote close path.
- `CLOSED` should be treated as a terminal remote state; it may skip close, but it must not trigger local close without query evidence.
- Amount mismatch must publish the existing critical alert pattern, naming `payment_order_no`, `remote_state`, `expected_amount`, `actual_amount`.

Use `baofuPaymentFactFromQueryResult` or a Baofu-specific equivalent to build the fact payload; preserve `tradeNo` in `ExternalSecondaryKey` and `RawResource` so the application layer has traceable evidence.

- [ ] **Step 4: Route `PaymentChannelBaofuAggregate` in `preparePaymentOrderTimeoutClose`**

Update the top-level switch:

```go
if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
    return p.prepareBaofuPaymentOrderTimeoutClose(ctx, paymentOrder)
}
```

Keep it above the generic direct/ecommerce/ordinary checks only if that preserves the actual ownership boundary in this repo. The important invariant is that Baofu must not fall through to `required=false`.

- [ ] **Step 5: Route Baofu in `closeRemotePaymentOrderForTimeout`**

Extend the remote close helper so `remoteClose.baofu` calls:

```go
_, err := p.baofuAggregateClient.CloseOrder(ctx, aggregatecontracts.OrderCloseRequest{
    MerchantID: p.baofuProfitSharingConfig.CollectMerchantID,
    TerminalID: p.baofuProfitSharingConfig.CollectTerminalID,
    OutTradeNo: paymentOrder.OutTradeNo,
})
```

Required error behavior:

- return a wrapped error such as `close baofu payment order before local timeout close: %w` on any unexpected provider error;
- do not swallow `ORDER_NOT_EXIST` or `SYSTEM_BUSY` into success;
- do not update local payment order or cancel business order if remote close failed.

- [ ] **Step 6: Keep logging inside the worker decision boundary**

The worker must log with structured fields before returning errors for the following cases:

- remote query failure,
- remote close failure,
- amount mismatch,
- abnormal/unknown state.

Use `log.Error()` or `log.Warn()` with fields:

- `payment_order_id`
- `payment_order_no`
- `out_trade_no`
- `remote_state`
- `remote_amount` / `expected_amount` where relevant

Do not add `fmt.Println`. Do not convert these errors into `nil` or `skip`.

- [ ] **Step 7: Make `UpdatePaymentOrderToClosed` and `CancelOrderTx` happen only after the Baofu branch authorizes it**

The final flow for `pending` Baofu orders must be:

1. query remote state,
2. decide if remote close is required,
3. call remote close,
4. update local payment order to closed,
5. cancel the linked business order.

Any earlier failure must stop the chain.

- [ ] **Step 8: Re-run the Baofu worker tests**

Run:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskPaymentOrderTimeout_Baofu|TestProcessTaskOrderPaymentTimeout_DoesNotBypassBaofuPaymentOrder' -v
```

Expected: the tests from Task 1 now pass.

---

## 5. Task 3: Block The Legacy Order Timeout Bypass

**Files:**
- Modify: `locallife/worker/task_order_timeout.go`
- Test: `locallife/worker/task_order_timeout_test.go`
- Optional: `locallife/api/logic_adapters.go` only if the chosen fix is to reschedule the correct task instead of erroring.

- [ ] **Step 1: Implement the single allowed behavior for the legacy path**

Production decision: `order:payment_timeout` must delegate to the payment-order timeout path when the order has a linked `pending` Baofu aggregate payment order.

Rationale:

- Returning an error from the legacy task would stop the unsafe local cancel, but it would also leave an already-timed-out order waiting for a separate operator or retry path.
- Delegation preserves the invariant that Baofu payment orders are closed only through the payment-order timeout flow while still making timeout progress automatically.
- This avoids duplicate cancellation logic: the payment-order timeout path remains the single owner for remote Baofu query, remote Baofu close, local payment close, and business order cancel.

This is not a silent fallback. Delegation must log a structured info record with `order_id`, `order_no`, `payment_order_id`, `payment_order_no`, `payment_channel`, and `operation=delegate_order_timeout_to_payment_order_timeout`.

- [ ] **Step 2: Implement the legacy guard**

In `ProcessTaskOrderPaymentTimeout`, after loading the order but before `CancelOrderTx`, look up the latest linked payment order.

If a linked pending Baofu aggregate payment order exists:

- do not call `CancelOrderTx`,
- do not silently continue,
- call the payment-order timeout path for the linked payment order in the same worker execution or enqueue `payment_order:timeout` when the distributor boundary is required by existing architecture,
- return the delegate result; if delegation fails, log and return the error so asynq can retry.

If the legacy task still needs to support older non-Baofu orders, the guard must be channel-specific, not a blanket ban.

- [ ] **Step 3: Add a test that proves the bypass is closed**

The test must assert that the legacy task does not directly cancel a pending business order when the Baofu payment order is still pending.

Assert the delegate call or same-process handoff to `ProcessTaskPaymentOrderTimeout`, depending on the final local wiring. Also assert the business order is not cancelled by the legacy path itself.

- [ ] **Step 4: Run the focused test**

Run:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskOrderPaymentTimeout_DelegatesPendingBaofuPaymentOrder' -v
```

Expected: PASS.

---

## 6. Task 4: Expose Stable, Semantic Frontend Messaging

**Files:**
- Modify: `locallife/api/payment_order.go`
- Modify: `locallife/api/server.go` only if a helper is needed for the response shape.
- Test: `locallife/api/payment_order_test.go`

- [ ] **Step 1: Keep the API response shape stable**

Production decision: do not add `status_hint` or any new frontend field for this fix.

Rationale:

- The existing `paymentOrderQueryResponse` already exposes the durable payment status fields the frontend needs.
- The production defect is an unsafe backend state transition and provider-error exposure risk, not a missing UI data field.
- Adding a new optional response field would create an unnecessary contract branch and frontend interpretation surface.

The fix must only improve the error path: provider failures are logged with full backend context and returned to the frontend as a stable Chinese error message.

- [ ] **Step 2: Map provider failures to stable Chinese text**

When a Baofu provider path fails in the API layer, do not expose raw provider text. Use the existing `loggedServerError` / `internalError` / `writeLogicRequestError` patterns so that:

- logs keep the wrapped provider error,
- response only contains a stable Chinese sentence such as `宝付支付状态暂不可确认，请稍后刷新支付状态；如持续失败请联系平台处理`,
- the sentence indicates what the front-end should do next, such as refresh status or contact platform.

- [ ] **Step 3: Add an API regression test**

Add a test proving the JSON response or error body contains the stable frontend wording and not the raw upstream text.

- [ ] **Step 4: Re-run the API test**

Run:

```bash
cd locallife
go test ./api -run 'Test.*PaymentOrder.*(Query|Close|Timeout|Baofu)' -v
```

Expected: PASS.

---

## 7. Task 5: Reuse Existing Baofoo Contract Semantics, Do Not Drift

**Files:**
- Modify: `locallife/logic/baofu_payment_service.go`
- Modify: `locallife/logic/baofu_payment_service_test.go`
- Optional: `locallife/worker/baofu_payment_recovery_scheduler.go` only if you discover the timeout worker needs a shared helper extracted from recovery.

- [ ] **Step 1: Keep `CloseOrder` as the only Baofu close writer**

Do not duplicate the close request construction in multiple places unless the same helper is not extractable without widening the file too much.

If the worker needs a shared helper, extract a tiny helper in `baofu_payment_service.go` or a closely related file, for example:

```go
func buildBaofuOrderCloseRequest(cfg BaofuPaymentServiceConfig, paymentOrder db.PaymentOrder) aggregatecontracts.OrderCloseRequest
```

Only extract if it reduces duplication in both service and worker. Otherwise keep the duplication local and explicit.

- [ ] **Step 2: Extend service tests for close request correctness**

Add/extend a test asserting the close request uses:

- `MerchantID == CollectMerchantID`
- `TerminalID == CollectTerminalID`
- `OutTradeNo == paymentOrder.OutTradeNo`

The test must also assert that the `external_payment_commands` record is written before the client call.

- [ ] **Step 3: Make sure the recovery scheduler continues to use the same query reference rule**

The timeout worker must match the recovery scheduler rule from `locallife/worker/baofu_payment_recovery_scheduler.go`:

- prefer `TransactionID` as `tradeNo` when present,
- otherwise use `OutTradeNo`.

If the logic diverges, factor the reference rule into a small local helper and reuse it from both places.

- [ ] **Step 4: Re-run service tests**

Run:

```bash
cd locallife
go test ./logic -run 'TestBaofuPaymentService.*CloseOrder|TestBaofuPaymentService.*CreateWechatJSAPIOrder' -v
```

Expected: PASS.

---

## 8. Validation Matrix

Run these in order after implementation:

```bash
cd locallife
go test ./worker -run 'TestProcessTaskPaymentOrderTimeout_Baofu|TestProcessTaskOrderPaymentTimeout_DoesNotBypassBaofuPaymentOrder' -v
go test ./logic -run 'TestBaofuPaymentService.*CloseOrder' -v
go test ./api -run 'Test.*PaymentOrder.*(Query|Close|Timeout|Baofu)' -v
make check-baofu-contract
make test-safety
```

Expected outcomes:

- worker tests pass,
- logic tests pass,
- API test passes,
- Baofu contract check passes,
- safety suite passes or the failure is tied to a pre-existing unrelated issue that is explicitly documented.

If `make test-safety` is too broad for the final iteration, the minimum acceptable fallback is the focused worker/logic/API tests plus `make check-baofu-contract`, but the residual risk must be stated plainly.

---

## 9. Self-Review Before Execution

Before handing this to an implementer, verify the plan itself against the spec:

1. Spec coverage: the plan includes Baofu query, Baofu close, local state update, business order cancel, legacy bypass, and frontend message semantics.
2. Placeholder scan: no `TBD`, no vague “add validation”, no “write tests for the above” without explicit test names.
3. Type consistency: the plan uses only existing names already present in the codebase, such as `PaymentChannelBaofuAggregate`, `CloseOrder`, `payment_order:timeout`, `TaskOrderPaymentTimeout`, and `loggedServerError`.
4. Non-negotiable rule check: there is no path that says “if Baofu close fails, still close locally”.
