# State Sequencing Audit Card: Customer Dine-In Checkout Convergence

Date: 2026-06-15
Risk theme: state sequencing / transaction consistency / release configuration
Risk class: G3 - table session, billing group, order payment, post-paid session checkout
Status: source-audited, fixed, backend recovery implemented, validated

## Decision

`CUSTOMER-DINE-IN-CHECKOUT` was promoted from slice-ledgered to source audit
before changing paid-session checkout behavior. The audit found a real authz
drift: the Mini Program paid-result path calls
`/v1/dining-sessions/{id}/checkout` as the customer, while backend
`logic.CheckoutDiningSession` previously required the caller to be a merchant.

Fix commit:

- `ad0e609d fix: allow paid dine-in customer checkout`

Follow-up implementation in this change set:

- backend scheduler recovery for paid open dine-in sessions;
- SQL/sqlc query that scans only open sessions with active paid dine-in orders;
- partial index for the recovery scan; and
- scheduler registration in the runtime composition root.

The backend now allows a customer to close only their own dining session when
the session has an active dine-in order owned by that customer and that order is
`paid`. Merchant manual checkout remains available only when the authenticated
merchant owns the target session's merchant. Both paths still reuse
`CloseDiningSessionTx`, so session close, billing group close, table release,
and reservation completion remain in the transaction-owned close boundary.

## Current Runtime Path

1. Customer scans table or enters dining params, then scan-entry opens or resumes
   a dining session.
2. Menu and checkout pages rehydrate session, billing group, merchant, table,
   and cart from backend state.
3. Dine-in checkout creates an order with `billing_group_id`, creates payment,
   and navigates to the shared payment result page.
4. Payment result polls backend payment truth while status is
   `pending_confirmation`.
5. When payment becomes paid, the result page calls
   `checkoutPaidDineInSession`, which uses the saved pending checkout context
   and calls backend `checkoutDiningSession`.
6. Backend `checkoutDiningSession` calls `logic.CheckoutDiningSession`, closes
   the dining session, and publishes table/reservation updates.
7. If the customer checkout API call fails transiently after payment success,
   `DineInCheckoutRecoveryScheduler` scans paid open dine-in sessions after a
   short buffer and reuses `CloseDiningSessionTx` to close the session, billing
   groups, table occupancy, and reservation state.

## Source-Audit Answers

| Question | Answer |
| --- | --- |
| Does a paid order always have enough context to locate the dining session? | The Mini Program saves pending checkout context with `session_id`, `order_id`, and optional `payment_order_id` before navigating to the payment result page. Backend closure uses the trusted `session_id` to read the session and validates `active_order_id` server-side. |
| What happens if payment succeeds but `checkoutDiningSession` fails once? | The result page catches the failure and leaves a user-visible sync note. Backend recovery now scans paid open dine-in sessions older than the recovery buffer and closes them through `CloseDiningSessionTx`. |
| Is `checkoutDiningSession` idempotent for an already closed session? | `CloseDiningSessionTx` returns an already closed session as success. The new customer path still requires the session's active paid order guard before reaching the transaction. |
| Does billing group membership block an otherwise paid session close? | No. Checkout close is authorized by merchant ownership or by session owner plus active paid dine-in order; it does not require billing group membership. Billing groups are closed by `CloseDiningSessionTx`. |
| Do table/reservation websocket failures affect durable session closure? | No. Handler websocket sends happen after logic returns a closed session; durable close is already committed by the transaction. |

## Code Changes

- `locallife/logic/dining_session.go`: `CheckoutDiningSession` now reads the
  target session before choosing the authz path. Merchant callers can close
  sessions only for their own merchant; session owners can close only after a
  paid active dine-in order is verified.
- `locallife/api/dining_session.go`: Swagger description and `409` failure
  contract now describe both merchant manual checkout and paid customer
  checkout.
- `locallife/api/dining_session_test.go`: API coverage includes merchant
  checkout, customer paid checkout, missing session, missing active order, and
  internal failure.
- `locallife/logic/dining_session_test.go`: logic coverage includes paid
  customer checkout, customer who also owns a different merchant, unpaid active
  order rejection, and non-owner rejection.
- `locallife/db/query/dining_session.sql`: adds a recovery query that requires
  `dining_sessions.status = 'open'`, non-null active order via the join, order
  `status = 'paid'`, order `order_type = 'dine_in'`, and matching merchant/user.
- `locallife/scheduler/dine_in_checkout_recovery.go`: adds the recurring
  recovery job and per-session failure isolation while reusing
  `CloseDiningSessionTx`.
- `locallife/main.go`: registers `dine-in-checkout-recovery` with the existing
  scheduler manager.
- `locallife/db/migration/000269_add_dine_in_checkout_recovery_index.up.sql`:
  adds the partial index used by the recovery scan.
- `locallife/db/sqlc/tx_dining_session_test.go` and
  `locallife/scheduler/dine_in_checkout_recovery_test.go`: cover recovery
  query filters, index presence, scheduler success, list failure, and
  per-session close failure continuation.

## Evidence Anchors

- Customer dine-in slice:
  `artifacts/codegraph/customer-state-flows/customer-dine-in-session-menu-checkout.slice.md`.
- Backend menu and checkout handlers:
  `locallife/api/dining_session.go:350` and
  `locallife/api/dining_session.go:713`.
- Backend checkout logic:
  `locallife/logic/dining_session.go:501`.
- Session close SQL:
  `locallife/db/query/dining_session.sql:32`.
- Paid open session recovery SQL:
  `locallife/db/query/dining_session.sql`.
- Backend scheduler recovery:
  `locallife/scheduler/dine_in_checkout_recovery.go`.
- Recovery scan index:
  `locallife/db/migration/000269_add_dine_in_checkout_recovery_index.up.sql`.
- Billing group/order session validation:
  `locallife/logic/order_session.go:169` through `:213`.
- Payment result polling and dine-in close trigger:
  `weapp/miniprogram/pages/payment/result/index.ts:77`,
  `weapp/miniprogram/pages/payment/result/index.ts:136`, and
  `weapp/miniprogram/pages/payment/result/index.ts:168`.
- Pending dine-in checkout context and backend checkout call:
  `weapp/miniprogram/pages/payment/_main_shared/services/dine-in-session.ts:78`
  through `:120`.

## Source-Audit Questions

| Question | Closure status |
| --- | --- |
| Does a paid order always have enough context to locate the dining session? | Closed for current path: frontend pending checkout context carries `session_id`; backend verifies server-side active order. |
| What happens if payment succeeds but `checkoutDiningSession` fails once? | Closed for backend convergence: recovery scheduler closes eligible paid open sessions through the same transaction boundary. |
| Is `checkoutDiningSession` idempotent for an already closed session? | Closed at transaction layer via `CloseDiningSessionTx`; caller still must satisfy authz/order guard. |
| Does billing group membership block an otherwise paid session close? | Closed: not required for close. |
| Do table/reservation websocket failures affect durable session closure? | Closed: websocket emits are post-close best effort. |

## Focused Validation To Add Or Run

From `locallife/`:

```bash
go test ./api -run 'Test.*DiningSession|TestCheckoutDiningSession' -count=1
go test ./logic -run 'Test.*DiningSession|Test.*OrderSession' -count=1
```

Executed focused and package validation:

```bash
go test ./logic -run 'TestCheckoutDiningSession' -count=1
go test ./api -run TestCheckoutDiningSessionAPI -count=1
go test ./logic -run 'TestCheckoutDiningSession|TestResolveDiningSessionMenu|TestOpenDiningSession' -count=1
go test ./api -run 'TestCheckoutDiningSessionAPI|TestOpenDiningSessionAPI|TestGetDiningSessionMenuAPI|TestGetDiningSessionEntryAPI|TestPrecheckDiningSessionAPI' -count=1
go test ./db/sqlc -run 'Test.*CloseDiningSession|Test.*DiningSession' -count=1
go test ./logic -count=1
go test ./api -count=1
make swagger
make check-generated
```

Executed backend recovery validation:

```bash
make migrateup
go test ./db/sqlc -run 'TestPaidOpenDineInCheckoutRecoveryIndexExists|TestListPaidOpenDineInSessionsForCheckoutRecovery' -count=1
go test ./db/sqlc -run 'Test.*DiningSession' -count=1
go test ./scheduler -run TestDineInCheckoutRecoveryScheduler -count=1
go test ./scheduler -count=1
go test . -run '^$' -count=1
make check-generated
```

From `weapp/`, add or run a focused script covering:

```bash
node scripts/<new-or-existing-dine-in-checkout-contract-test>.js
```

The missing high-value regression is:

1. QR scan/open session.
2. Add cart.
3. Dine-in checkout creates order/payment and saves pending checkout context.
4. Payment result starts as pending.
5. Backend payment query/callback reaches paid.
6. Payment result calls `checkoutPaidDineInSession`.
7. Backend session is closed and subsequent menu/session read rehydrates closed
   or non-actionable state.

## Remaining Follow-Ups

The original authz blocker and backend post-paid recovery gap are fixed and
covered. Remaining follow-ups are:

1. Add Mini Program contract/E2E coverage for pending checkout context survival
   across result-page reload and paid-status polling.
2. Consider an operational alert if the recovery scheduler repeatedly fails to
   list or close eligible sessions.
3. Keep this card as the rerun checklist before changing dine-in checkout,
   shared payment result, or dining-session close behavior.
