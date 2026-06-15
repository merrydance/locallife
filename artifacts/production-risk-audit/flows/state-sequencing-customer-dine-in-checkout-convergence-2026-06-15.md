# State Sequencing Audit Card: Customer Dine-In Checkout Convergence

Date: 2026-06-15
Risk theme: state sequencing / transaction consistency / release configuration
Risk class: G3 - table session, billing group, order payment, post-paid session checkout
Status: execution card, documentation-only

## Decision

Promote `CUSTOMER-DINE-IN-CHECKOUT` from slice-ledgered to source-audit before
changing paid-session checkout behavior. The critical gap is not a known code
bug; it is missing proof that paid order convergence reliably triggers
`checkoutDiningSession` and leaves table/session/billing state rehydratable.

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

| Question | Required answer before code changes |
| --- | --- |
| Does a paid order always have enough context to locate the dining session? | Prove pending checkout context is saved before payment and is still available after payment-result reload. |
| What happens if payment succeeds but `checkoutDiningSession` fails once? | Prove menu/session rehydration or retry can close the paid session later. |
| Is `checkoutDiningSession` idempotent for an already closed session? | Verify source/tests before deciding whether retries should return success or require readback. |
| Does billing group membership block an otherwise paid session close? | Verify current backend ownership/session guards. |
| Do table/reservation websocket failures affect durable session closure? | They should be post-write side effects; prove this in source. |

## Focused Validation To Add Or Run

From `locallife/`:

```bash
go test ./api -run 'Test.*DiningSession|TestCheckoutDiningSession' -count=1
go test ./logic -run 'Test.*DiningSession|Test.*OrderSession' -count=1
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

## Remaining Real Issue

Paid order -> session checkout convergence is still only slice-ledgered. It
needs source-level proof and E2E/contract coverage before any change to dine-in
checkout, shared payment result, or dining-session close behavior.
