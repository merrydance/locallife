# Baofu Reservation Completed-Share And Refund Transaction Design

Risk class: G3 - Baofu reservation money movement, pre-share refund, profit-sharing command start, scheduler recovery, SQL/sqlc, and refund transaction boundaries.

Related finding: `BE-AUDIT-2026-05-29-01`

Related prior slice: `artifacts/baofu-refund-slice-fix-plan.md`

## Background

The Baofu refund slice first found and fixed a pure technical bypass: `ReplaceReservationOrderWithBaofu` created refund orders through plain `CreateRefundOrder`, so it skipped the locked refund guard in `CreateRefundOrderTx`. That part is a technical vulnerability because the code bypassed an existing shared invariant.

The follow-up audit found a broader timing conflict. Baofu reservation and `reservation_addon` payment orders currently require profit sharing, and the recovery scheduler scans paid Baofu aggregate payment orders every five minutes. For reservations, the current readiness policy treats `paid`, `confirmed`, `checked_in`, and `completed` as ready for profit sharing. At the same time, reservation cancellation refunds, dish-reduction refunds, and replacement-order refunds are still supported before reservation completion.

This means the race is not only in refund creation code:

- If refund creation wins first, the active refund order blocks later profit sharing.
- If the scheduler creates and starts profit sharing first, later pre-share refund is correctly blocked by the Baofu refund guard.
- The guard is technically correct after sharing starts, but the business result is wrong if the UI still offers refund-capable reservation actions.

## Business Decisions

The durable product rule is:

- Merchant `completed` is a required operation for reservations.
- Reservation profit sharing starts only after the merchant completes the reservation.
- `completed_at` is the only readiness anchor for reservation and `reservation_addon` Baofu profit sharing.
- There is no `checked_in_at + 4h` automatic fallback.
- There is no separate 4-hour reminder or alert. Incomplete checked-in tables already have a different UI state, and there is no guaranteed immediate reminder channel that would make a timer alert operationally reliable.

Classification:

- The old direct `CreateRefundOrder` path was a pure technical vulnerability and has already been fixed.
- The early reservation profit-sharing policy is a business-rule mismatch with money-path consequences. Now that the business cutoff is explicitly `completed`, changing readiness to `completed_at` is a required G3 business-rule correction.
- The dish-change and replacement-order transaction gaps are technical consistency defects once the business operation is defined as "change state and reserve refund amount together". They should be fixed because a user-visible successful dish/order mutation must not survive if the required refund-order creation is rejected by the Baofu guard.

## Target Invariants

After this design is implemented:

- Baofu reservation and `reservation_addon` profit sharing can be created only when the reservation is `completed` and `completed_at` is present.
- `paid`, `confirmed`, and `checked_in` reservations are never profit-sharing-ready, regardless of age.
- Active refund orders in `pending` or `processing` block profit sharing.
- Successful partial refunds on `reservation` and `reservation_addon` payment orders do not permanently block later profit sharing; the local bill must be created or refreshed against `payment_order.amount - successful_refunds`.
- If the net amount is zero or negative, no Baofu profit sharing is created.
- The successful-refund net-share rule is scoped to reservation payment orders only. Ordinary `order` payment orders keep the existing behavior where successful refunds block automatic Baofu profit-sharing recovery/triggering.
- `reservation_addon` is an independent payment order created by dish add/positive dish-change delta; it receives its own independent Baofu profit-sharing bill rather than being merged into the original reservation payment.
- A started Baofu profit-sharing command still blocks new pre-share refunds.
- Dish replacement with a negative price delta atomically replaces reservation items and creates all required refund orders in one DB transaction.
- Replacement-order negative delta atomically creates the replacement order, marks the old order replaced, links billing groups, and creates all required refund orders in one DB transaction.
- External Baofu provider calls are never made inside the DB transaction.
- If provider submission fails after commit, the committed refund order remains durable and recoverable through the existing pending/processing/recovery paths.

## Profit-Sharing Readiness Design

Change `locallife/db/query/profit_sharing_order.sql` query `ListBaofuOrdersReadyForProfitSharing` for the reservation branch.

Current shape:

```sql
po.business_type IN ('reservation', 'reservation_addon')
AND r.status IN ('paid', 'confirmed', 'checked_in', 'completed')
AND COALESCE(r.paid_at, r.updated_at, po.paid_at, po.created_at) <= sqlc.arg(refund_closed_before)
```

Target shape:

```sql
po.business_type IN ('reservation', 'reservation_addon')
AND r.status = 'completed'
AND r.completed_at IS NOT NULL
AND r.completed_at <= sqlc.arg(refund_closed_before)
```

Keep these exclusions and net-amount rules:

- No active refund order in `pending` or `processing`.
- For ordinary `order` payment orders, no successful refund order.
- For `reservation` and `reservation_addon` payment orders, successful refunds are subtracted from the payment amount and the query returns `net_amount`; rows with net amount `<= 0` are skipped.
- No existing `profit_sharing_orders` row for the same payment order in `processing` or `finished`. Existing Baofu `pending` or `failed` bills may be refreshed to the latest reservation net amount before command start.
- Payment order must be paid, Baofu aggregate, and `requires_profit_sharing = true`.

Update ordering to use `r.completed_at` for reservation rows, not `paid_at` or `updated_at`, so the oldest completed reservations are picked first.

Update `locallife/worker/baofu_payment_recovery_scheduler.go`:

- Replace `baofuReservationReadyForProfitSharing(status string) bool` with a check that requires both `status == "completed"` and `completed_at.Valid`.
- Add the same defensive check inside `createBaofuReservationProfitSharingOrder`, even though the SQL scan should already filter it. This protects direct call sites and future scheduler changes.
- Do not use `checked_in_at` in readiness.

## Immediate Trigger After Completion

The recovery scheduler remains the fallback, but completion should also enqueue profit sharing immediately.

Current order flow already has the pattern:

- Resolve an existing Baofu profit-sharing bill for a completed order.
- Schedule `TaskProcessBaofuProfitSharing` through `TaskScheduler.ScheduleProfitSharing`.

Reservation completion should follow the same style:

- After `CompleteReservationTx` commits and returns a `completed` reservation with `completed_at`, resolve or create the Baofu reservation profit-sharing bill for each paid Baofu payment order tied to the reservation.
- Dish-change add-payments use the existing `reservation_addon` business type. Completion-triggered sharing treats each paid Baofu `reservation` or `reservation_addon` payment order independently and passes that payment order's successful refunded amount into bill creation.
- Resolve the active `profit_sharing_configs` row for `order_source = reservation` before creating the bill, using the merchant and region scope. The immediate completion trigger and the recovery scheduler must use the same rate source, so reservation bills do not silently fall back to hardcoded 2% / 3% rates.
- Enqueue `ScheduleProfitSharing` for the created or existing pending/failed bill.
- Log and continue if enqueue fails; the recovery scheduler remains the durable fallback.

Implementation can add a focused helper such as `ScheduleBaofuProfitSharingForCompletedReservation`, or keep it in `CompleteReservation` if the existing logic style favors a smaller change. The helper should not create any provider command directly. It should only create or resolve the local bill and enqueue the existing worker task.

## Refund Transaction Boundary Design

The current reusable refund guard is inside `CreateRefundOrderTx`. To reuse it inside larger transactions, extract the guarded body into an internal helper that accepts transaction-scoped `*Queries`.

Suggested shape:

```go
func createRefundOrderWithGuard(
    ctx context.Context,
    q *Queries,
    arg CreateRefundOrderTxParams,
) (CreateRefundOrderTxResult, error)
```

`CreateRefundOrderTx` then becomes a thin wrapper around `store.execTx` plus this helper. The helper must keep the existing behavior:

- Lock payment order with `GetPaymentOrderForUpdate`.
- Validate optional idempotency metadata and replay behavior.
- Reject non-paid payment orders.
- For Baofu aggregate orders, lock and check `GetBaofuPaymentOrderRefundGuardForUpdate`.
- Count occupied refund amount with `GetTotalRefundedByPaymentOrder`, including `pending`, `processing`, and `success`.
- Create refund order in `pending` status.
- Create idempotency binding when metadata is present.

Dish-change transaction:

- Add a DB transaction such as `ReplaceReservationItemsWithRefundOrdersTx`.
- It replaces reservation items and creates all required guarded refund orders in the same transaction.
- The logic layer prepares validated items, refund allocations, and `out_refund_no` values before entering the transaction.
- On guarded refund failure, item replacement rolls back.
- After commit, the logic layer schedules `ProcessRefund` tasks for each returned refund order.

Replacement-order transaction:

- Add a separate transaction such as `ReplaceOrderWithRefundOrdersTx` rather than changing every `ReplaceOrderTx` caller.
- It should reuse the current `ReplaceOrderTx` behavior: create new order, create items, create status log, mark old order replaced, and copy billing group links.
- It then creates all required guarded refund orders in the same transaction.
- On guarded refund failure, the new order and old-order replacement state roll back.
- After commit, the logic layer submits or schedules the provider refund. Provider calls stay post-commit.

Do not put Baofu HTTP calls in these transactions. Database atomicity should cover local business state and local refund-order reservation only.

## API Semantics

For dish-change or replacement-order negative deltas:

- If the guarded refund order cannot be created, return a conflict or business error and leave the old business state unchanged.
- If the DB transaction commits, return success for the business operation and mark refund initiation according to the returned refund orders.
- If task enqueue or provider submission fails after commit, do not report the whole business operation as rolled back. Log the failure and rely on the durable refund order plus recovery path.

This avoids the current bad user-visible state where the business mutation can be persisted while the required refund-order creation fails afterward.

## Existing Financial Records

The first implementation prevents new early reservation profit-sharing orders. It does not automatically mutate historical profit-sharing records.

Current rollout note: no historical data remediation is required for this implementation because the issue was found before affected historical records existed.

Before rollout, run an audit query for existing Baofu reservation or `reservation_addon` profit-sharing orders whose reservation is not completed:

- `pending` or `failed`: review whether they can be cancelled or left unretried until completion.
- `processing` or `finished`: do not modify automatically. These need operational review because provider-side money movement may already have started or completed.

If production data contains early-started reservation sharing, post-share refund is outside the first-version supported scope and should remain a manual-support/operations path unless a separate post-share refund design is approved.

## Tests

DB/sqlc tests:

- `paid` reservation payment order is not returned by `ListBaofuOrdersReadyForProfitSharing`.
- `confirmed` reservation payment order is not returned.
- `checked_in` reservation payment order is not returned.
- `completed` reservation without `completed_at` is not returned.
- `completed` reservation with `completed_at` is returned.
- Completed reservation with active refund order is still blocked.
- `completed` reservation with successful partial refund is returned with `net_amount`.
- Ordinary completed `order` payment with successful refund is not returned by this reservation fix.
- Completed reservation with existing `processing` or `finished` profit-sharing order is still blocked.
- Completed reservation with existing Baofu `pending` or `failed` profit-sharing order can be refreshed to the current net amount before command start.
- Dish-change transaction rolls back item replacement when guarded refund creation rejects.
- Replacement-order transaction rolls back new order and old-order replacement state when guarded refund creation rejects.

Worker tests:

- Recovery scheduler creates and enqueues Baofu reservation profit sharing only for `completed` + `completed_at`.
- Recovery scheduler rejects direct reservation profit-sharing creation for non-completed reservations.
- Recovery scheduler rejects completed reservations missing `completed_at`.

Logic/API tests:

- Merchant completes a full-payment Baofu reservation and the logic schedules profit sharing after commit.
- Merchant completes a Baofu reservation after successful partial refunds and the logic schedules independent net-amount sharing for both the primary `reservation` payment and any `reservation_addon` payment.
- Completion succeeds even if scheduling fails, with a warning log and recovery fallback.
- Modify reservation dishes with negative delta uses the combined transaction and schedules refund tasks after commit.
- Modify reservation dishes leaves old items unchanged when the refund guard rejects.
- Replace reservation order with negative delta uses the combined transaction.
- Replace reservation order leaves the old order unreplaced when the refund guard rejects.

## Validation

Expected validation after implementation:

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" make sqlc
/usr/local/go/bin/go test ./db/sqlc -run 'TestListBaofuOrdersReadyForProfitSharing|TestReplaceReservationItems.*Refund|TestReplaceOrder.*Refund' -count=1 -p 1
/usr/local/go/bin/go test ./logic -run 'TestCompleteReservation|TestModifyReservationDishes|TestReplaceReservationOrder' -count=1
/usr/local/go/bin/go test ./worker -run 'TestBaofuPaymentRecoveryScheduler|TestProcessTaskBaofuProfitSharing' -count=1
/usr/local/go/bin/go test ./logic ./worker
PATH="/usr/local/go/bin:$PATH" make check-baofu-contract
PATH="/usr/local/go/bin:$PATH" make test-safety
PATH="/usr/local/go/bin:$PATH" make check-generated
git diff --check
```

Run narrower commands first while implementing. Broader validation is required before marking `BE-AUDIT-2026-05-29-01` resolved.

## Implementation Order

1. Add failing readiness tests that prove `paid`, `confirmed`, and `checked_in` reservations are no longer share-ready.
2. Change SQL readiness to `completed` + `completed_at`, run `make sqlc`, and update generated code.
3. Update worker defensive readiness and scheduler tests.
4. Add reservation-completion immediate scheduling through the existing `TaskScheduler.ScheduleProfitSharing` path.
5. Extract `createRefundOrderWithGuard` from `CreateRefundOrderTx` without changing behavior.
6. Add dish-change combined transaction and logic tests.
7. Add replacement-order combined transaction and logic tests.
8. Run full validation.
9. Update `.github/review/open-findings.md` and `.github/review/audit-log.md` to close `BE-AUDIT-2026-05-29-01` only after implementation and validation pass.

## Implementation Closure

Implemented on 2026-05-29:

- Reservation Baofu profit-sharing readiness now requires `r.status = 'completed'` and `r.completed_at IS NOT NULL`.
- The worker also defensively rejects reservation profit sharing without `completed_at`.
- Merchant completion triggers local Baofu profit-sharing bill creation/enqueue after `CompleteReservationTx` commits, with the scheduler retained as fallback.
- Reservation immediate-trigger and recovery-created bills now both resolve active `profit_sharing_configs` for `order_source = reservation`, and tests assert non-default rates propagate into the persisted Baofu bill.
- `CreateRefundOrderTx` now exposes its guarded body for transaction-scoped reuse without changing the public transaction behavior.
- Dish-change negative deltas use `ReplaceReservationItemsWithRefundOrdersTx`, so item replacement rolls back when guarded refund-order creation is rejected.
- Dish replacement transactions lock the reservation and re-check the caller's observed current item total before replacing items, so concurrent modify requests fail closed instead of applying stale refund/payment deltas.
- Replacement-order negative deltas use `ReplaceOrderWithRefundOrdersTx`, so the new order and old-order replacement marker roll back when guarded refund-order creation is rejected.
- Replacement-order transactions lock the old order and require it to still be paid and unreplaced before creating the replacement order.
- Idempotent Baofu profit-sharing bill creation now also rejects payment orders with occupied refund amount, so completion/recovery cannot leave a local bill that will later be permanently blocked at command start.
- Successful partial refunds on `reservation` and `reservation_addon` payment orders now participate in net-amount sharing. Pending/processing refunds still block sharing, zero-net payments are skipped/rejected, and existing Baofu `pending`/`failed` bills are refreshed to the net amount before command execution.
- The net-share follow-up is deliberately scoped to reservation payment orders. Ordinary `order` payment orders with successful refunds are still excluded from automatic Baofu sharing by the recovery scan and completion trigger.
- Dish add/positive dish-change payments remain independent `reservation_addon` payment orders with independent Baofu share bills; they are not merged into the original reservation payment order.
- Baofu provider calls remain post-commit and outside DB transactions.

Validation run:

```bash
/usr/local/go/bin/go test ./db/sqlc -run 'TestReplaceReservationItemsWithRefundOrdersTx|TestReplaceOrderWithRefundOrdersTx|TestCreateRefundOrderTx' -count=1 -p 1
/usr/local/go/bin/go test ./logic -run 'TestModifyReservationDishes|TestReplaceReservationOrder|TestCompleteReservation' -count=1
/usr/local/go/bin/go test ./worker -run 'TestBaofuPaymentRecoverySchedulerRunOnce' -count=1
/usr/local/go/bin/go test ./api -run TestCompleteReservationAPI -count=1
/usr/local/go/bin/go test ./logic ./worker
PATH="/usr/local/go/bin:$PATH" make check-baofu-contract
PATH="/usr/local/go/bin:$PATH" make test-safety
PATH="/usr/local/go/bin:$PATH" make check-generated
git diff --check
```

## Non-Goals

- No `checked_in_at + 4h` auto-share fallback.
- No 4-hour reminder feature.
- No post-share refund support in this change.
- No automatic mutation of historical profit-sharing orders.
- No provider HTTP call inside a DB transaction.
