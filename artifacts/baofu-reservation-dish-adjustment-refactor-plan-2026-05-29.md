# Baofu Reservation Dish Adjustment Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor full-payment reservation dish changes so positive deltas become effective only after Baofu `reservation_addon` payment success, while zero and negative deltas remain atomic and auditable.

**Architecture:** Introduce durable `reservation_adjustments` target snapshots. Positive adjustments create pending snapshots plus independent Baofu add-on payment orders; payment success fact application atomically applies the snapshot. Negative adjustments apply the snapshot and guarded refund orders in one transaction. Existing Baofu provider calls remain outside DB transactions.

**Tech Stack:** Go, Gin, pgx/sqlc, PostgreSQL migrations, Asynq workers, Baofu aggregate payment/refund/share, WeChat Mini Program TypeScript.

**Implementation status, 2026-05-29:** implemented in the active working tree. Final validation and commit/push are still pending.

Additional implementation notes:

- Baofu JSAPI order creation uses the `SubMchID` returned by the same local transaction that created the payment order and persisted its `attach`, preventing split-brain merchant-channel reads.
- Payment timeout handling expires linked `reservation_addon` adjustments both when the payment is closed in the current run and when a retry sees the payment order already closed.
- Reservation inventory release treats a missing `daily_inventory` row as an idempotent no-op for unlimited-stock dishes, so pending adjustment holds can always be released.

---

Risk class: G3. This touches funds, provider callbacks/query facts, reservation state, inventory, refund guards, profit-sharing readiness, timeout recovery, and frontend payment workflow.

Source design: `artifacts/baofu-reservation-dish-adjustment-refactor-design-2026-05-29.md`

Do not start implementation before reopening:

- `.github/copilot-instructions.md`
- `.github/README.md`
- `locallife/AGENTS.md`
- `.github/instructions/backend-locallife.instructions.md`
- `.github/prompts/backend-payment-domain.prompt.md`
- `.github/standards/domains/baofu-payment/README.md`
- `artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md`

## File Map

Create or modify these files during implementation:

- Create migration: `locallife/db/migration/000240_add_reservation_adjustments.up.sql`
- Create migration: `locallife/db/migration/000240_add_reservation_adjustments.down.sql`
- Create queries: `locallife/db/query/reservation_adjustment.sql`
- Modify queries if needed: `locallife/db/query/table_reservation.sql`, `locallife/db/query/reservation_item.sql`, `locallife/db/query/payment_order.sql`
- Modify generated sqlc after regeneration: `locallife/db/sqlc/**`
- Modify transaction code: `locallife/db/sqlc/tx_reservation.go`, `locallife/db/sqlc/tx_payment_success.go`
- Modify payment creation: `locallife/logic/payment_order_service.go`, `locallife/logic/baofu_payment_order_route.go`, `locallife/logic/service_support.go`, `locallife/logic/interfaces.go`
- Refactor dish logic: `locallife/logic/reservation_dishes.go` and, if needed for file size, create `locallife/logic/reservation_adjustments.go`
- Modify fact failure handling: `locallife/logic/payment_fact_application_service.go`, `locallife/logic/baofu_payment_fact_application.go`
- Modify timeout handling: `locallife/worker/task_payment_timeout.go`
- Modify reservation actions: `locallife/logic/reservation.go`, `locallife/api/table_reservation_action_state.go`
- Modify API responses: `locallife/api/table_reservation.go`
- Regenerate Swagger if annotations/contracts change: `locallife/docs/**`
- Modify Mini Program API/types: `weapp/miniprogram/api/reservation.ts`
- Modify Mini Program page: `weapp/miniprogram/pages/reservation/modify/index.ts`
- Reuse Mini Program workflow: `weapp/miniprogram/services/payment-workflow.ts`
- Add or update tests under `locallife/db/sqlc`, `locallife/logic`, `locallife/api`, `locallife/worker`, and `weapp` if frontend test tooling exists for the touched page.

## Task 1: Lock The Existing Bug With Tests

- [ ] Add a logic test proving positive full-payment `ModifyReservationDishes` does not call `ReplaceReservationItemsTx` before add-on payment success.

Target file: `locallife/logic/reservation_dishes_test.go`

Test name: `TestModifyReservationDishesPositiveDeltaCreatesPendingAdjustmentWithoutReplacingItems`

Expected failure before implementation: current code calls `ReplaceReservationItemsTx` and then tries to create payment.

- [ ] Add a logic test proving `AddReservationDishes` does not call `CreateReservationItem` before add-on payment success.

Target file: `locallife/logic/reservation_dishes_test.go`

Test name: `TestAddReservationDishesPositiveDeltaCreatesPendingAdjustmentWithoutAppendingItems`

Expected failure before implementation: current code calls `CreateReservationItem`.

- [ ] Add an API test proving `modify-dishes` passes a Baofu-capable payment facade and returns a typed `payment_required` response for positive delta.

Target file: `locallife/api/table_reservation_test.go`

Expected failure before implementation: handler does not pass `PaymentFacade`, and response type is not the new contract.

## Task 2: Add Adjustment Schema And Queries

- [ ] Add migration `000240_add_reservation_adjustments.up.sql`.

Include:

- `reservation_adjustments`
- `reservation_adjustment_items`
- `reservation_adjustment_inventory_holds`
- status/direction check constraints
- one-active-positive partial unique index
- `payment_order_id` unique nullable foreign key
- indexes by reservation, payment order, status, and expiry

- [ ] Add migration `000240_add_reservation_adjustments.down.sql`.

Drop indexes and tables in reverse dependency order.

- [ ] Add `locallife/db/query/reservation_adjustment.sql`.

Required queries:

- `CreateReservationAdjustment`
- `CreateReservationAdjustmentItem`
- `CreateReservationAdjustmentInventoryHold`
- `GetReservationAdjustmentForUpdate`
- `GetReservationAdjustmentByPaymentOrderForUpdate`
- `ListReservationAdjustmentItems`
- `ListReservationAdjustmentInventoryHoldsForUpdate`
- `ListActiveReservationAdjustments`
- `MarkReservationAdjustmentPendingPayment`
- `MarkReservationAdjustmentApplying`
- `MarkReservationAdjustmentApplied`
- `MarkReservationAdjustmentClosed`
- `MarkReservationAdjustmentFailed`
- `MarkReservationAdjustmentExpired`
- `MarkReservationAdjustmentHoldConverted`
- `MarkReservationAdjustmentHoldReleased`
- `GetActiveReservationAdjustmentByReservation`

- [ ] Run sqlc generation.

Command:

```bash
cd locallife
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make sqlc
```

Expected: generated query files and interfaces update cleanly.

## Task 3: Add DB Transactions For Positive Adjustment Creation

- [ ] Add a transaction in `locallife/db/sqlc/tx_reservation.go`:

Suggested name: `CreateReservationPositiveAdjustmentPaymentTx`.

Responsibilities:

- lock `table_reservations`
- reject non-owner, wrong merchant, wrong status, cooking started, or active positive adjustment
- verify expected current total
- insert `reservation_adjustments(status='creating_payment', direction='positive')`
- insert `reservation_adjustment_items`
- reserve incremental dish inventory into `daily_inventory.reserved_quantity`
- insert `reservation_adjustment_inventory_holds(status='held')`
- create local `payment_orders(business_type='reservation_addon', payment_channel='baofu_aggregate', requires_profit_sharing=true)`
- set `reservation_adjustments.payment_order_id`
- return payment order, adjustment, items, holds, and merchant `sub_mch_id`

- [ ] Add DB tests:

Target file: `locallife/db/sqlc/tx_reservation_adjustment_test.go`

Tests:

- `TestCreateReservationPositiveAdjustmentPaymentTxDoesNotReplaceEffectiveItems`
- `TestCreateReservationPositiveAdjustmentPaymentTxRejectsActiveAdjustment`
- `TestCreateReservationPositiveAdjustmentPaymentTxRejectsStaleCurrentTotal`
- `TestCreateReservationPositiveAdjustmentPaymentTxHoldsIncrementalInventory`
- `TestCreateReservationPositiveAdjustmentPaymentTxRollsBackPaymentWhenInventoryUnavailable`

## Task 4: Add DB Transaction For Positive Payment Success Application

- [ ] Add a transaction in `locallife/db/sqlc/tx_payment_success.go` or a focused new tx file:

Suggested name: `ApplyPaidReservationAdjustmentTx`.

Responsibilities:

- lock payment order
- lock adjustment by `payment_order_id`
- idempotently return if payment processed and adjustment applied
- mark adjustment `applying`
- lock reservation
- require status `paid`, `confirmed`, or `checked_in`
- require `cooking_started_at` is null
- replace effective `reservation_items` from adjustment items
- convert held inventory into `reservation_inventory` without double-reserving daily inventory
- create `reservation_payments(type='addon')`
- add prepaid amount
- mark adjustment `applied`
- mark payment processed

- [ ] Wire `ProcessPaymentSuccessTx` for `reservation_addon`:

If linked adjustment exists, call the new application path. If no linked adjustment exists, preserve current legacy behavior for old add-on payments until the old path is fully removed.

- [ ] Add DB tests:

Target file: `locallife/db/sqlc/tx_payment_success_reservation_adjustment_test.go`

Tests:

- `TestProcessPaymentSuccessTxReservationAddonAppliesPendingAdjustmentOnce`
- `TestProcessPaymentSuccessTxReservationAddonDuplicateCallbackDoesNotDuplicateItems`
- `TestProcessPaymentSuccessTxReservationAddonAddsPrepaidOnlyOnce`
- `TestProcessPaymentSuccessTxReservationAddonConvertsInventoryHold`
- `TestProcessPaymentSuccessTxReservationAddonFailsWhenReservationCookingStarted`

## Task 5: Add Close, Failure, And Timeout Transactions

- [ ] Add `CloseReservationAdjustmentForPaymentTx`.

Responsibilities:

- lock payment and adjustment
- no-op if adjustment is already `applied`
- release held inventory
- mark adjustment `closed`, `failed`, or `expired`
- keep effective reservation items unchanged

- [ ] Wire Baofu terminal failure application:

Files:

- `locallife/logic/payment_fact_application_service.go`
- `locallife/logic/baofu_payment_fact_application.go`

When a `reservation_addon` payment terminal fact is `closed` or `failed`, mark both payment order and linked adjustment terminal.

- [ ] Wire local timeout:

File: `locallife/worker/task_payment_timeout.go`

After local pending payment close succeeds for a `reservation_addon` linked to an adjustment, expire the adjustment and release holds.

- [ ] Wire user close:

File: `locallife/logic/payment_order_service.go`

When closing a pending linked `reservation_addon` payment, close the adjustment and release holds.

- [ ] Add tests:

Files:

- `locallife/db/sqlc/tx_reservation_adjustment_close_test.go`
- `locallife/logic/payment_fact_application_service_test.go`
- `locallife/worker/task_payment_timeout_test.go`

Tests:

- close releases holds and preserves effective items
- failed fact marks adjustment failed
- timeout marks adjustment expired
- late close after applied logs/conflicts but does not undo applied items

## Task 6: Refactor Reservation Dish Logic To Target-Snapshot Adjustment

- [ ] Replace the core `ModifyReservationDishes` positive path.

File: `locallife/logic/reservation_dishes.go`

New behavior:

- validate target items
- compute `current_total`, `target_total`, and `delta`
- if `delta > 0`, call the dedicated positive adjustment payment flow and return `Payment`
- do not call `ReplaceReservationItemsTx`
- do not mutate `reservation_items`
- do not add prepaid amount

- [ ] Route `AddReservationDishes` through the same target-snapshot service.

It should load current effective items, merge new requested items, then call the same adjustment code.

- [ ] Preserve and reuse existing negative-delta transaction behavior from the completed-share design:

Current useful code:

- `ReplaceReservationItemsWithRefundOrdersTx`
- `createRefundOrderWithGuard`
- `buildReservationRefundAllocations`

Refine it only as needed to record `reservation_adjustments`.

- [ ] Keep provider refund submission post-commit.

Do not move Baofu refund HTTP calls into DB transactions.

- [ ] Add logic tests:

Target file: `locallife/logic/reservation_dishes_test.go`

Tests:

- positive delta returns payment and no effective item mutation
- zero delta applies immediately
- negative delta records adjustment, applies items, creates refund orders, and schedules refunds
- cooking-started reservation rejects all modifications
- active pending positive adjustment rejects new modification
- incomplete refund allocation rejects and leaves state unchanged

## Task 7: Add Payment Facade Support For Adjustment Payments

- [ ] Add a narrow method to `logic.PaymentFacade`.

Suggested method:

```go
CreateReservationAdjustmentPaymentOrder(ctx context.Context, input CreateReservationAdjustmentPaymentInput) (CreatePaymentOrderResult, error)
```

The input should include user ID, reservation ID, target items, current total, target total, delta amount, client IP, and now/expires-at data.

- [ ] Implement it in `DefaultPaymentFacade` via `PaymentOrderService`.

File: `locallife/logic/service_support.go`

- [ ] Add `PaymentOrderService.CreateReservationAdjustmentPayment`.

Files:

- `locallife/logic/payment_order_service.go`
- `locallife/logic/baofu_payment_order_route.go`

The method should:

- validate Baofu payment service configured
- validate merchant Baofu readiness
- validate user openid
- call `CreateReservationPositiveAdjustmentPaymentTx`
- call Baofu unified order outside the transaction
- mark adjustment `pending_payment` on success
- close payment/release adjustment on unified-order or pay-param parse failure

- [ ] Update test fakes and mocks.

Run:

```bash
cd locallife
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make mock
```

Run only if mock-backed interfaces changed.

## Task 8: Guard Reservation Operational Actions

- [ ] Add DB query:

`GetActiveReservationAdjustmentByReservation`

- [ ] Guard `CompleteReservation`.

File: `locallife/logic/reservation.go`

If active positive adjustment exists, return 409 with a clear message and pending payment reference.

- [ ] Guard start-cooking.

File: `locallife/logic/reservation.go`

Reject while active positive adjustment exists.

- [ ] Guard cancellation.

File: `locallife/logic/reservation.go`

Reject while active positive adjustment exists. Do not silently cancel locally while Baofu add-on payment can still succeed remotely.

- [ ] Update action-state response.

File: `locallife/api/table_reservation_action_state.go`

Expose disabled action reasons for pending adjustment/payment.

- [ ] Add tests:

Files:

- `locallife/logic/reservation_complete_profit_sharing_test.go`
- `locallife/logic/reservation_action_state_test.go`
- `locallife/logic/reservation_cancel_test.go`

Tests:

- completion rejects with active pending adjustment
- start-cooking rejects with active pending adjustment
- cancel rejects with active pending adjustment
- normal completed-at profit-sharing behavior remains unchanged after no active adjustment

## Task 9: Update API Contract And Swagger

- [ ] Replace response structs in `locallife/api/table_reservation.go`.

New response should include:

- `outcome`
- `adjustment_id`
- `delta_amount`
- optional `payment`
- optional `refunds`
- optional user-facing `message`

- [ ] Pass `server.buildPaymentFacade()` or `server.paymentFacade` into add/modify dish logic.

Current missing locations:

- `locallife/api/table_reservation.go:1532-1538`
- `locallife/api/table_reservation.go:1626-1633`

- [ ] Schedule timeout only after a payment response exists.

Keep using `server.scheduleTimeoutForPaymentOrder`.

- [ ] Update Swagger annotations.

Run:

```bash
cd locallife
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make swagger
```

Run because route response contracts change.

- [ ] Add API tests:

Target file: `locallife/api/table_reservation_test.go`

Tests:

- positive delta returns `payment_required`
- negative delta returns `refund_initiated`
- zero delta returns `applied`
- missing Baofu config returns safe payment setup message and does not imply item mutation
- unauthorized user cannot create adjustment

## Task 10: Update Mini Program Reservation Modify Flow

- [ ] Type the backend response.

File: `weapp/miniprogram/api/reservation.ts`

Add a `ReservationDishAdjustmentResponse` union or interface with `outcome`, `adjustment_id`, `delta_amount`, `payment`, and `refunds`.

- [ ] Update `ReservationService.modifyDishes` and `addDishes` return types.

Current return type is `Promise<unknown>`.

- [ ] Update modify page submit flow.

File: `weapp/miniprogram/pages/reservation/modify/index.ts`

Behavior:

- call `ReservationService.modifyDishes`
- if `payment_required`, call `completePaymentWorkflow`
- navigate back only when workflow result is `paid`
- show a non-terminal state if payment is `cancelled`, `closed`, `failed`, `pending_confirmation`, or pay params are missing
- if `refund_initiated` or `applied`, navigate back or reload as existing UX expects

- [ ] Keep frontend scope narrow.

Do not touch unrelated dirty Mini Program files. The current worktree has many unrelated `weapp/` changes.

- [ ] Run the smallest relevant Mini Program validation if frontend code changes.

Potential commands from `weapp/`:

```bash
PATH="$HOME/.local/bin:$PATH" npm run compile
PATH="$HOME/.local/bin:$PATH" npm run lint
```

If dependency/tooling is unavailable, state that explicitly with the exact failure.

## Task 11: Deprecate Or Fence The Old Replace-Order Reservation Path

- [ ] Inspect current callers of `/v1/orders/{id}/replace`.

Files:

- `locallife/api/order.go`
- `locallife/logic/replace_order.go`
- Mini Program order/reservation callers

- [ ] Add a guard or test so reservation dish-change UI does not use replacement order.

The primary reservation dish-change route must be `modify-dishes`.

- [ ] If `ReplaceReservationOrderWithBaofu` still supports reservation add-money replacement, add a follow-up finding or route it through `reservation_adjustments`.

Do not broaden this task into ordinary takeout partial-refund behavior.

## Task 12: Profit Sharing And Refund Regression Coverage

- [ ] Keep completed-at share readiness from the prior design unchanged.

Regression tests should still pass:

- paid reservation not ready for sharing
- confirmed reservation not ready
- checked-in reservation not ready
- completed reservation with `completed_at` ready
- active refund blocks sharing
- successful partial reservation/add-on refunds share by net amount

- [ ] Add add-on adjustment sharing test.

After positive adjustment payment success and reservation completion, a paid `reservation_addon` payment order should produce an independent Baofu share bill based on its own net amount.

Target files:

- `locallife/db/sqlc/profit_sharing_order_recovery_test.go`
- `locallife/logic/reservation_complete_profit_sharing_test.go`

## Task 13: Full Validation Gate

Run focused tests first while implementing. Before calling the refactor complete, run:

```bash
cd locallife
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make sqlc
/usr/local/go/bin/go test ./db/sqlc -run 'Test.*ReservationAdjustment|TestProcessPaymentSuccessTxReservationAddon|TestReplaceReservationItemsWithRefundOrdersTx|TestListBaofuOrdersReadyForProfitSharing' -count=1 -p 1
/usr/local/go/bin/go test ./logic -run 'TestModifyReservationDishes|TestAddReservationDishes|TestCompleteReservation|TestCancelReservation|TestReservationActionState' -count=1
/usr/local/go/bin/go test ./api -run 'Test.*Reservation.*Dishes|TestCompleteReservationAPI|TestCancelReservationAPI' -count=1
/usr/local/go/bin/go test ./worker -run 'Test.*Payment.*Timeout|TestBaofuPaymentRecoveryScheduler' -count=1
/usr/local/go/bin/go test ./logic ./worker
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-baofu-contract
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make test-safety
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-generated
git diff --check
```

If API annotations changed, include:

```bash
cd locallife
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make swagger
```

If mocks changed, include:

```bash
cd locallife
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make mock
```

If Mini Program code changed, include the smallest relevant command from `weapp/`:

```bash
cd weapp
PATH="$HOME/.local/bin:$PATH" npm run compile
```

## Completion Criteria

- Positive full-payment reservation dish changes do not mutate `reservation_items`, `reservation_inventory`, `reservation_payments`, or `prepaid_amount` before Baofu payment success fact application.
- Payment success applies the linked target snapshot exactly once.
- Payment close/fail/timeout leaves effective items unchanged and releases pending inventory holds.
- Negative dish changes apply target items and create guarded refund orders atomically.
- Add-dishes and modify-dishes use the same target-snapshot adjustment model.
- Mini Program payment-required response is handled through `completePaymentWorkflow`.
- Completion/start-cooking/cancellation reject while a positive adjustment payment is pending.
- Reservation and `reservation_addon` share bills remain independent and net successful reservation refunds correctly.
- Provider calls remain outside DB transactions.
- No ordinary takeout partial-refund semantics are changed.
