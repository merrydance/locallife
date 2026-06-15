# Idempotency Card: Merchant Order Action Concurrent Validation

Date: 2026-06-15
Risk theme: idempotency and retry / state sequencing
Risk class: G3/G2 - merchant order status, reject refund boundary, print side effects
Status: execution card, documentation-only

## Decision

Do not add request idempotency keys to merchant status actions by default. The
current product contract relies on conditional state transitions plus backend
readback, and Flutter duplicate accept/print coalescing is already covered.

The remaining work is API-level concurrent multi-client validation: Mini Program,
Flutter, kitchen, websocket/push/polling, and manual refresh can all point at
the same backend action endpoints.

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
   after the first successful transition fail and must be resolved by readback.

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

## Source-Audit Questions

| Question | Required answer before code changes |
| --- | --- |
| If two clients accept the same paid order concurrently, is exactly one transition/log/delivery-pool side effect produced? | Add API or db-backed concurrent test; do not rely only on single-call unit tests. |
| If accept and reject race on the same paid order, is there one durable winner and clear readback for the loser? | Add cross-action concurrent test. |
| If ready is repeated from kitchen and merchant detail, is status/log behavior deterministic? | Add concurrent ready/readback coverage. |
| Does reject refund truth stay visible when reject wins but refund submission is pending/manual? | Preserve `refund_submission` response truth in concurrency tests. |
| Does Flutter local BLE print remain single-fire for a coalesced accept? | Existing Flutter tests cover local coalescing; keep them in pre-change validation. |

## Focused Validation To Add Or Run

From `locallife/`:

```bash
go test ./logic -run 'TestAcceptMerchantOrder|TestRejectMerchantOrder|TestMarkMerchantOrderReady' -count=1
go test ./api -run 'TestAcceptOrderAPI|TestRejectOrderAPI|TestMarkOrderReadyAPI' -count=1
go test ./db/sqlc -run 'TestUpdateOrderStatusTx|TestAcceptTakeoutOrderTx|TestMarkTakeoutOrderReadyTx|TestOrderLifecycle_MerchantReject' -count=1
```

From `merchant_app/`:

```bash
flutter test test/order_acceptance_coordinator_test.dart test/order_alert_coordinator_test.dart
```

Add missing db/API concurrency tests before changing status-action contracts.

## Remaining Real Issue

The backend has conditional transitions, and Flutter coalescing is covered, but
there is still no dedicated proof for cross-client/API-level concurrent
accept/reject/ready races. That proof should be added before modifying merchant
action APIs or status transactions.
