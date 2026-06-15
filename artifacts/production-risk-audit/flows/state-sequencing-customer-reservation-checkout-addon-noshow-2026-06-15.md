# State Sequencing Audit Card: Customer Reservation Checkout, Add-On, Refund, And No-Show

Date: 2026-06-15
Risk theme: state sequencing / idempotency and retry / transaction consistency / authorization boundaries
Risk class: G3 - reservation availability, deposit/full payment, add-on payment, refund, no-show attribution, table/session handoff
Status: source-audited; backend transaction and worker proof exists; dedicated customer-side Mini Program contract and real provider proof still pending

## Decision

Promote `CUSTOMER-RESERVATION` before reservation payment, add-on,
modify-dishes, no-show, refund, or dine-in handoff changes.

The current source audit does not identify a fresh production-code defect. The
active risk is proof and change-safety: reservation state spans customer
drafts, backend room availability, Baofu payment facts, add-on adjustment
transactions, refund recovery, merchant no-show actions, table occupancy, and
dine-in session handoff. Future changes must start from this card instead of
using the takeout checkout card as a proxy.

## Current Runtime Path

1. Customer reservation confirm builds a draft request from selected room,
   date, time, contact, guest count, and payment mode, then calls
   `POST /v1/reservations`.
2. Backend `CreateReservation` rejects stale time/table/capacity/payment-mode
   input, applies locked table minimum-spend/deposit pricing, checks final
   reservation conflicts after locking the table row, and creates reservation
   plus full-payment pre-order items inside `CreateReservationTx`.
3. Deposit reservations enter the shared payment workflow with
   `business_type=reservation`; full-payment reservations route into dine-in
   menu with the persisted reservation id.
4. Reservation payment success is provider-fact driven. `ProcessPaymentSuccessTx`
   creates `reservation_payments`, marks the reservation paid, and syncs
   reservation inventory. Replays first check existing reservation payment by
   `payment_order_id`.
5. Customer detail/list pages can restart reservation payment from persisted
   reservation state. The payment result page polls backend payment truth and
   routes reservation and reservation add-on results back to reservation detail.
6. Add-dishes/modify-dishes lock the reservation, require customer ownership
   on online reservations, block cooking-started or active-adjustment states,
   and produce one of three outcomes: applied, payment-required, or
   refund-initiated.
7. Positive full-payment adjustments create a `reservation_addon` Baofu payment
   order plus adjustment rows and inventory holds without replacing effective
   reservation items until payment is paid.
8. Paid add-on callbacks apply the adjustment once, replace effective items,
   convert inventory holds, and mark the payment processed.
9. Negative full-payment adjustments create refund orders in the same
   replace-items transaction and enqueue refund tasks when possible; if enqueue
   fails, refund recovery remains the fallback.
10. Cancel/complete/no-show all reject active adjustments before terminalizing
    reservation status. Cancel can create an async Baofu refund order after the
    cancellation transaction; no-show records behavior evidence and avoids
    attributing offline/phone reservations to the staff account.

## Evidence Anchors

- Customer reservation codegraph slice:
  `artifacts/codegraph/customer-state-flows/customer-reservation-lifecycle.slice.md`.
- Merchant reservation/table shared state slice:
  `artifacts/codegraph/merchant-state-flows/merchant-reservation-and-table.slice.md`.
- Customer confirm submit and deposit payment workflow:
  `weapp/miniprogram/pages/reservation/confirm/index.ts:229` through `:303`.
- Customer modify-dishes add-on payment handoff:
  `weapp/miniprogram/pages/reservation/modify/index.ts:336` through `:362`.
- Shared payment result re-entry and reservation redirect:
  `weapp/miniprogram/pages/payment/result/index.ts:77` through `:158` and
  `:208` through `:220`.
- Backend reservation create validation and locked transaction entry:
  `locallife/logic/reservation.go:197` through `:358`.
- Backend cancel/refund/no-show/complete guards:
  `locallife/logic/reservation.go:523` through `:705` and
  `locallife/logic/reservation.go:708` through `:735`.
- Add-on/refund logic:
  `locallife/logic/reservation_dishes.go:42` through `:130` and
  `locallife/logic/reservation_dishes.go:201` through `:430`.
- Reservation create/cancel/no-show/confirm/complete transactions:
  `locallife/db/sqlc/tx_reservation.go:35` through `:111`,
  `:132` through `:188`, `:205` through `:230`, `:247` through `:260`,
  and `:946` through `:1011`.
- Active-adjustment guard:
  `locallife/db/sqlc/tx_reservation.go:594` through `:604`.
- Positive add-on payment transaction and paid adjustment application:
  `locallife/db/sqlc/tx_reservation_adjustment.go:58` through `:188` and
  `:208` through `:260`.
- Reservation and add-on payment success:
  `locallife/db/sqlc/tx_payment_success.go:141` through `:208`.
- Reservation timeout and no-show alert workers:
  `locallife/worker/task_reservation_timeout.go:17` through `:180`.
- Reservation refund recovery:
  `locallife/worker/refund_recovery_scheduler.go:189` through `:275`.

## Source-Audit Questions

| Question | Current answer |
| --- | --- |
| Can a stale room/date/time/minimum-spend draft create an invalid reservation? | Backend prechecks and `CreateReservationTx` lock the table row, revalidate table truth, reapply locked pricing, and rerun conflict checks before insert. |
| Can payment success be replayed without breaking reservation state? | `ProcessPaymentSuccessTx` checks `reservation_payments.payment_order_id` before insert and skips already-applied reservation/add-on payments. |
| Can two positive add-on attempts replace effective items before payment? | Active adjustment guards block another add/modify/cancel/complete/no-show while one adjustment is active; positive add-on payment creates adjustment rows and holds, not effective item replacement. |
| Can a paid add-on replay apply twice? | `ApplyPaidReservationAdjustmentTx` returns idempotently when payment is processed and adjustment is already applied. |
| Can a negative modify over-refund after funds changed? | Logic computes allocations from paid reservation payment orders and rejects incomplete refund allocation; `ReplaceReservationItemsWithRefundOrdersTx` has current-total and refund guard tests. |
| Can no-show punish the staff account for phone/walk-in reservations? | `MarkNoShowTx` leaves `behavior_decisions.user_id` null for offline/phone sources and records offline identity in the fact snapshot. |
| Can merchant terminal actions bypass active add-on/refund uncertainty? | Complete, cancel, no-show, cooking-start, and item replacement all check active adjustments. |
| Does the customer have a re-entry path after unknown reservation payment result? | Payment result polling and reservation detail/list payment restart exist, but there is no dedicated customer reservation contract script equivalent to takeout checkout. |
| Does real provider callback/recovery evidence exist for reservation deposit/add-on/refund? | Not from this local audit; this remains part of the Baofu evidence gap. |

## Human-Centered UI Check

- Role and primary task: a customer is reserving a room, paying a deposit or
  full amount, later changing dishes, and recovering from payment/退款 states
  without needing to understand payment infrastructure.
- High-frequency path: reservation detail and user-center reservations must be
  able to resume payment or show the latest backend reservation status after
  WeChat/Baofu result uncertainty.
- First-screen priority: reservation status, payment state, refund/add-on
  outcome, allowed next action, reservation time, room, merchant, and amount
  should be visible before secondary merchant/table metadata.
- State to preserve: selected room/date/time/guest/contact draft before submit;
  returned reservation id after create; payment order id for pending
  confirmation; modify-dishes selection until backend outcome is known.
- Failure/recovery paths: payment-create failure after reservation create must
  route to persisted reservation truth; add-on payment failure must keep the
  adjustment/payment order recoverable; refund-initiated copy must lead to a
  readback path rather than treating refund as instant success.
- Non-goals: do not add request idempotency keys or optimistic local terminal
  states without backend transaction/fact ownership.

## Focused Validation To Run Before Reservation Changes

From `locallife/`:

```bash
PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestCancelReservation|TestConfirmReservation|TestMarkReservationNoShow|TestCompleteReservation|Test.*ReservationDishes|Test.*Reservation.*ProfitSharing' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestCreateReservationTx|TestConfirmReservationTx|TestCompleteReservationTx|TestCancelReservationTx|TestMarkNoShowTx|TestReservationCompleteFlow|TestReservationCancelFlow|TestReservationTableConsistency|Test.*ReservationAdjustment|Test.*ReservationInventory|Test.*ReservationRefund' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestCreateReservationAPI|TestGetReservationAPI|TestListUserReservationsAPI|TestConfirmReservationAPI|TestCancelReservationAPI|TestCompleteReservationAPI|TestMarkNoShowAPI' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'Test.*Reservation.*|TestRefundRecoverySchedulerRunOnceProcessesPendingReservationRefundsWithoutOrderRefunds|TestRefundRecoverySchedulerRunOnceRecordsBaofuReservationAddonRefundQueryFact' -count=1
```

From `weapp/`:

```bash
npm run check:merchant-reservation-table-status-contract
npm run check:merchant-reservation-actions-contract
npm run check:payment-refund-terminal-flow
npm run check:payment-workflow-boundary
```

Before any customer reservation checkout/add-on UI change, add or run a
dedicated customer reservation contract script that proves:

- create -> deposit payment-create failure routes to persisted reservation
  detail/list truth;
- pending payment result with `paymentOrderId` polls backend status and routes
  back to reservation detail;
- modify-dishes `payment_required` preserves the add-on payment order and can
  recover after unknown result;
- modify-dishes `refund_initiated` renders async refund truth and does not
  imply instant refund success;
- copied reservation/payment wrappers stay aligned for create/detail/list,
  modify/add-dishes, payment create/query/detail, and refund detail.

## Validation Recorded 2026-06-15

Commands run:

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestCancelReservation|TestConfirmReservation|TestMarkReservationNoShow|TestCompleteReservation|Test.*ReservationDishes|Test.*Reservation.*ProfitSharing' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run 'TestCreateReservationTx|TestConfirmReservationTx|TestCompleteReservationTx|TestCancelReservationTx|TestMarkNoShowTx|TestReservationCompleteFlow|TestReservationCancelFlow|TestReservationTableConsistency|Test.*ReservationAdjustment|Test.*ReservationInventory|Test.*ReservationRefund' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestCreateReservationAPI|TestGetReservationAPI|TestListUserReservationsAPI|TestConfirmReservationAPI|TestCancelReservationAPI|TestCompleteReservationAPI|TestMarkNoShowAPI' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'Test.*Reservation.*|TestRefundRecoverySchedulerRunOnceProcessesPendingReservationRefundsWithoutOrderRefunds|TestRefundRecoverySchedulerRunOnceRecordsBaofuReservationAddonRefundQueryFact' -count=1

cd ../weapp
PATH="$HOME/.local/bin:$PATH" npm run check:merchant-reservation-table-status-contract
PATH="$HOME/.local/bin:$PATH" npm run check:merchant-reservation-actions-contract
PATH="$HOME/.local/bin:$PATH" npm run check:payment-refund-terminal-flow
PATH="$HOME/.local/bin:$PATH" npm run gate:payment-workflow-boundary
```

Observed result: all commands returned `ok` or printed their contract-passed
message. No production code, SQL, generated files, or Mini Program files were
changed in this step.

## Remaining Real Issue

Reservation backend state has strong transaction and focused-test coverage for
locked room creation, terminal status transitions, active adjustment blocking,
positive add-on payment application, negative modify refund creation,
inventory release/sync, offline no-show attribution, and refund recovery.

The remaining issue is evidence: customer reservation checkout/add-on does not
yet have a dedicated Mini Program contract script equivalent to takeout
checkout, and real Baofu callback/query/funds evidence for reservation
deposit, add-on, and refund paths is still external-provider evidence, not
local unit-test proof. Do not treat reservation payment/add-on/no-show changes
as release-ready until those checks are added or explicitly run with masked
target evidence.
