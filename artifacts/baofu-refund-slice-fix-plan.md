# Baofu Refund Slice Fix Plan

Risk class: G3 - Baofu refund, profit-sharing exclusion, callback/query recovery, and money-state terminalization.

## Background

The Baofu refund slice audit found two runtime risks and one cleanup item:

- `ReplaceReservationOrderWithBaofu` creates refund orders with plain `CreateRefundOrder`, bypassing `CreateRefundOrderTx` payment-order locking, cumulative refund checks, Baofu profit-sharing-started guard, and optional idempotency binding.
- Refund recovery records Baofu refund query facts for `order` and `reservation`, but not `reservation_addon`; callback and application layers already support `reservation_addon`.
- `merchant_reject_refund.go` keeps unused legacy command helper code that records provider `wechat` while using Baofu refund capability.

The core invariant is: for a Baofu aggregate payment order, refund creation and profit-sharing command start must be mutually exclusive, and all terminal refund facts must be applied through the modeled payment fact application path.

## Current Status

Implemented and verified:

- `ReplaceReservationOrderWithBaofu` now creates refund orders through `CreateRefundOrderTx`, so replacement-order refunds use the same payment-order lock, cumulative refund amount check, Baofu profit-sharing-started guard, and pending refund amount reservation as other refund entrypoints.
- Refund recovery now records Baofu query facts for `reservation_addon` and creates a `reservation_domain` application for terminal query results.
- The unused merchant-reject refund command helper chain was removed; the active merchant-reject path already uses `recordBaofuRefundCommand`.

Verification already run from `locallife/`:

```bash
/usr/local/go/bin/go test ./logic -run 'TestReplace|TestProcessReplace|TestMerchantReject|TestProcessMerchantReject' -count=1
/usr/local/go/bin/go test ./worker -run 'TestRefundRecoverySchedulerRunOnce' -count=1
/usr/local/go/bin/go test ./logic ./worker
PATH="/usr/local/go/bin:$PATH" make check-baofu-contract
PATH="/usr/local/go/bin:$PATH" make test-safety
git diff --check
```

No `make sqlc`, `make swagger`, or `make mock` was required for the implemented fix because no SQL, route/Swagger annotation, or interface signature changed.

## Refund Vs Profit-Sharing Timing Finding

Follow-up analysis found that this slice cannot be judged only from the refund creation side.

Current Baofu reservation payment creation sets `RequiresProfitSharing: true` for reservation and `reservation_addon` payments. The Baofu payment recovery scheduler runs every five minutes and scans for paid Baofu aggregate payment orders ready for profit sharing. For reservations, `ListBaofuOrdersReadyForProfitSharing` currently treats the following reservation statuses as ready:

- `paid`
- `confirmed`
- `checked_in`
- `completed`

The query excludes payment orders that already have refund orders in `pending`, `processing`, or `success`, and excludes payment orders that already have a profit-sharing order. This means:

- If a refund order is created first, pending/processing/success refund state blocks later profit sharing.
- If the scheduler creates and starts a profit-sharing command first, later refund creation is correctly blocked by the Baofu refund guard.

The already-implemented fix closes the technical bypass where replacement-order refunds skipped `CreateRefundOrderTx`. It does not change the broader business timing: reservation profit sharing can start shortly after payment success, before the diner has completed all allowed post-payment reservation actions such as changing dishes or cancelling within policy.

Classification:

- The old direct `CreateRefundOrder` call in `ReplaceReservationOrderWithBaofu` was a pure technical bug because it bypassed the shared Baofu refund guard.
- The remaining replacement-order transaction composition question is a business consistency design issue unless product requires replace-order success and refund-order creation to be atomic.
- The larger unresolved issue is Baofu reservation profit-sharing timing. If post-payment reservation refunds are a supported business flow until completion or another explicit cutoff, treating `paid`, `confirmed`, and `checked_in` reservations as profit-sharing-ready is likely too early.

Recommended next audit slice:

- Review Baofu reservation profit-sharing timing across `reservation` and `reservation_addon`.
- Decide the business cutoff for reservation refunds and dish changes.
- Align `ListBaofuOrdersReadyForProfitSharing`, `baofuReservationReadyForProfitSharing`, recovery scheduler behavior, and tests with that cutoff.
- Check whether active findings should be written to `.github/review/open-findings.md` after the audit.

Follow-up design:

- The business cutoff is now chosen: reservation and `reservation_addon` Baofu profit sharing should start only after the merchant-required `completed` action writes `completed_at`.
- There is no `checked_in_at + 4h` auto-share fallback and no separate 4-hour reminder.
- The combined design for completed-triggered sharing plus dish-change/replacement-order refund transaction refactors is in `artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md`.

Audit result:

- Finding confirmed. Current Baofu reservation profit-sharing timing overlaps with supported post-payment reservation actions.
- `locallife/db/query/profit_sharing_order.sql` treats `reservation` and `reservation_addon` payment orders as ready for profit sharing when reservation status is `paid`, `confirmed`, `checked_in`, or `completed`, provided there is no active refund order and no existing profit-sharing order.
- `locallife/worker/baofu_payment_recovery_scheduler.go` repeats the same readiness policy in `baofuReservationReadyForProfitSharing`.
- `locallife/logic/reservation.go` still allows cancel/refund from `paid` and `confirmed` reservations when refund policy permits it.
- `locallife/logic/reservation_dishes.go` still allows dish modification from `paid`, `confirmed`, and `checked_in`, and creates refund orders for negative deltas.
- `locallife/logic/replace_order.go` still allows full-payment reservation order replacement from `paid`, `confirmed`, and `checked_in`, and creates refund orders for negative deltas.
- `locallife/db/sqlc/profit_sharing_order_recovery_test.go` currently locks the early policy with `TestListBaofuOrdersReadyForProfitSharing_IncludesPaidReservations`.

Impact:

- If a refund action creates a refund order first, the pending/processing/success refund order blocks later sharing as intended.
- If the five-minute Baofu recovery scheduler creates and starts a profit-sharing order first, later cancellation, dish reduction, or replacement-order refund is blocked by the Baofu refund guard. That block is technically correct for pre-share refund safety, but it contradicts the still-available reservation business actions.
- Because LocalLife first-version scope supports pre-share refund and does not model post-share refund, early sharing can turn an otherwise supported automatic refund into a failed or manual-support path.

Durable tracking:

- Record this as a high-severity G3 open finding under `.github/review/open-findings.md`.
- Add an audit-log entry under `.github/review/audit-log.md`.
- Do not implement the timing change until the business cutoff is chosen. The likely fix is to restrict reservation profit-sharing readiness to `completed`, or to another explicit state/time cutoff that also closes cancel, dish modification, and replacement-order refund windows.

Key files for next slice:

- `locallife/db/query/profit_sharing_order.sql`
- `locallife/worker/baofu_payment_recovery_scheduler.go`
- `locallife/worker/task_baofu_profit_sharing.go`
- `locallife/logic/baofu_profit_sharing_trigger.go`
- `locallife/logic/reservation.go`
- `locallife/logic/reservation_dishes.go`
- `locallife/logic/replace_order.go`
- `locallife/logic/payment_fact_application_service.go`

## Files

- Modify `locallife/logic/replace_order.go`
- Modify `locallife/logic/replace_order_test.go`
- Modify `locallife/worker/refund_recovery_scheduler.go`
- Modify `locallife/worker/refund_recovery_scheduler_test.go`
- Modify `locallife/logic/merchant_reject_refund.go`

## Task 1: Route replacement-order refund creation through `CreateRefundOrderTx`

Goal: ensure replacement refunds use the same locked Baofu refund guard as merchant/API/reservation refunds.

Steps:

- Add a failing test showing `ReplaceReservationOrderWithBaofu` calls `CreateRefundOrderTx` and does not call plain `CreateRefundOrder`.
- Update `ReplaceReservationOrderWithBaofu` to create refund orders with `CreateRefundOrderTx`.
- Preserve provider call behavior after the refund order is created.
- Run focused logic tests:

```bash
cd locallife
go test ./logic -run 'TestReplace|TestProcessReplace'
```

Residual risk to check after implementation: replacement order DB state and refund order DB state still happen in separate transactions. If `CreateRefundOrderTx` fails after `ReplaceOrderTx`, the order replacement may already be persisted. This plan fixes the audited Baofu guard bypass first; a separate transaction-composition refactor may still be warranted.

## Task 2: Add `reservation_addon` to Baofu refund query recovery

Goal: callback-loss recovery should terminalize `reservation_addon` refund orders the same way callback does.

Steps:

- Add a failing worker test where a stuck Baofu `reservation_addon` refund query returns terminal success and should create a `reservation_domain` refund application.
- Update `recordBaofuRefundQueryFact` to map `reservation_addon` to reservation owner, `refund_order` business object, and `reservation_domain` consumer.
- Run focused worker tests:

```bash
cd locallife
go test ./worker -run 'TestRefundRecoverySchedulerRunOnce'
```

## Task 3: Remove stale merchant-reject refund command helper

Goal: remove unused code that advertises inconsistent provider/capability metadata.

Steps:

- Confirm `recordMerchantRejectRefundCommandAccepted` and `dbMerchantRejectRefundCommandInput` are not referenced outside their own helper chain.
- Delete the unused helper and snapshot function if it becomes unused.
- Run focused logic tests:

```bash
cd locallife
go test ./logic -run 'TestMerchantReject|TestProcessMerchantReject'
```

## Final Validation

After all focused tests pass, run a broader backend validation only if time and environment permit:

```bash
cd locallife
go test ./logic ./worker
```

No SQL, sqlc, Swagger, or mock regeneration is expected unless function signatures on generated/mock-backed interfaces change.
