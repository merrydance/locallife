# Idempotency Card: Merchant Order Action Concurrent Validation

Date: 2026-06-15
Risk theme: idempotency and retry / state sequencing
Risk class: G3/G2 - merchant order status, reject refund boundary, print side effects
Status: backend validation implemented

## Decision

Do not add request idempotency keys to merchant status actions by default. The
current product contract relies on conditional state transitions plus backend
readback, and Flutter duplicate accept/print coalescing is already covered.

Backend validation now covers the multi-client race boundary: conditional
transactions produce a single durable winner, and stale transaction losers are
reported to API callers as `409 Conflict` so clients can refresh backend truth.

## Current Runtime Path

1. Merchant Mini Program list/detail and kitchen views call accept/reject/ready
   wrappers, set local in-flight state, then reload backend truth.
2. Flutter order list/detail/alert paths use the same backend endpoints. Manual
   and alert-driven accept share `OrderAcceptanceCoordinator`, which coalesces
   duplicate accept/print attempts by order id.
3. Backend merchant and kitchen handlers resolve merchant ownership.
4. `logic.AcceptMerchantOrder`, `RejectMerchantOrder`, and
   `MarkMerchantOrderReady` lock/read the order, check ownership and expected
   state, then call status transactions.
5. SQL status updates use expected-status conditions. Repeated status actions
   after the first successful transition fail and are mapped by logic/API to
   `409 Conflict`; clients must resolve the loser path by readback.

## Evidence Anchors

- Merchant order slice:
  `artifacts/codegraph/merchant-state-flows/merchant-order-operations.slice.md`.
- Mini Program action/reload flow:
  `weapp/miniprogram/pages/merchant/orders/list/index.ts:566` through `:573`.
- Flutter coalescing evidence:
  `merchant_app/test/order_acceptance_coordinator_test.dart` and
  `merchant_app/test/order_alert_coordinator_test.dart`.
- Backend merchant order logic:
  `locallife/logic/merchant_order.go:41` through `:183`.
- Conditional status transaction:
  `locallife/db/sqlc/tx_order_status.go:33` through `:77`.
- Expected status SQL:
  `locallife/db/query/order.sql:200` through `:208`.
- Existing focused tests:
  `locallife/logic/merchant_order_test.go`,
  `locallife/api/order_test.go`, and
  `locallife/db/sqlc/tx_order_status_test.go`.
- Backend implementation commits:
  `a69a2fb1 test: add merchant order action concurrency coverage` and
  `93e1b3cd fix: map merchant order action races to conflict`.

## Source-Audit Questions

| Question | Current answer |
| --- | --- |
| If two clients accept the same paid order concurrently, is exactly one transition/log/delivery-pool side effect produced? | Covered by `TestAcceptTakeoutOrderTx_ConcurrentDuplicateAcceptHasSingleWinner`; one success, one stale-state conflict, one accept log, one delivery, one pool item. |
| If accept and reject race on the same paid order, is there one durable winner and clear readback for the loser? | Covered by `TestTakeoutOrderTx_ConcurrentAcceptAndRejectHasSingleDurableWinner` plus logic/API `ConflictAfterRead` cases; exactly one durable winner and the loser maps to `409 Conflict`. |
| If ready is repeated from kitchen and merchant detail, is status/log behavior deterministic? | Covered by `TestMarkTakeoutOrderReadyTx_ConcurrentDuplicateReadyHasSingleWinner`; one ready transition/log and one stale-state conflict. |
| Does reject refund truth stay visible when reject wins but refund submission is pending/manual? | Existing reject success API response still returns `refund_submission`; this phase did not change refund submission processing. |
| Does Flutter local BLE print remain single-fire for a coalesced accept? | Existing Flutter tests cover local coalescing; Flutter was not changed in this phase. |

## Focused Validation To Add Or Run

From `locallife/`:

```bash
go test ./logic -run 'TestAcceptMerchantOrder|TestRejectMerchantOrder|TestMarkMerchantOrderReady' -count=1
go test ./api -run 'TestAcceptOrderAPI|TestRejectOrderAPI|TestMarkOrderReadyAPI' -count=1
go test ./db/sqlc -run 'TestUpdateOrderStatusTx|TestAcceptTakeoutOrderTx|TestMarkTakeoutOrderReadyTx|TestOrderLifecycle_MerchantReject|TestTakeoutOrderTx_ConcurrentAcceptAndRejectHasSingleDurableWinner' -count=1
```

From `merchant_app/`:

```bash
flutter test test/order_acceptance_coordinator_test.dart test/order_alert_coordinator_test.dart
```

Rerun these before changing status-action contracts.

## Validation Recorded

- `go test ./db/sqlc -run 'TestAcceptTakeoutOrderTx_ConcurrentDuplicateAcceptHasSingleWinner|TestTakeoutOrderTx_ConcurrentAcceptAndRejectHasSingleDurableWinner|TestMarkTakeoutOrderReadyTx_ConcurrentDuplicateReadyHasSingleWinner' -count=10`
- `go test ./logic -run 'TestAcceptMerchantOrder|TestRejectMerchantOrder|TestMarkMerchantOrderReady' -count=1`
- `go test ./api -run 'TestAcceptOrderAPI|TestRejectOrderAPI|TestMarkOrderReadyAPI' -count=1`
- `go test ./db/sqlc -run 'TestUpdateOrderStatusTx|TestAcceptTakeoutOrderTx|TestMarkTakeoutOrderReadyTx|TestOrderLifecycle_MerchantReject|TestTakeoutOrderTx_ConcurrentAcceptAndRejectHasSingleDurableWinner' -count=1`

## Residual Risk

No request-level idempotency key was added by design. Clients still need to
refresh backend truth after a `409 Conflict`. This phase did not rerun Flutter
coalescing tests or Mini Program UI scripts because no frontend code changed.
