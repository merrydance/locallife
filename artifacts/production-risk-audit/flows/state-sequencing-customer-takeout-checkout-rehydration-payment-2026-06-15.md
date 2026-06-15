# State Sequencing Audit Card: Customer Takeout Checkout Rehydration And Payment

Date: 2026-06-15
Risk theme: state sequencing / idempotency and retry / transaction consistency
Risk class: G3 - customer cart, order creation, payment callback/recovery, visible order status
Status: source-audited, Mini Program contract covered, backend/provider proof pending

## Decision

Promote `CUSTOMER-TAKEOUT-CHECKOUT` before checkout/payment contract changes.
The active gap is proof, not a confirmed production-code defect: stale frontend
checkout snapshots must be shown to rehydrate through backend cart/pricing,
then payment callback/recovery must advance visible order status.

Reservation checkout has the same provider-recovery shape, but this card is
limited to takeout. Reservation should get a separate source card before
reservation payment/add-on changes.

## Current Runtime Path

1. Cart page sends a checkout snapshot through the event channel.
2. Order confirm accepts the snapshot as draft state, but falls back to
   `loadCart` if no snapshot arrives and recalculates delivery fee through
   backend cart calculation.
3. Order submit creates an order from the selected backend-cart-derived view.
4. Payment creation runs the shared payment workflow.
5. Payment result displays pending confirmation when needed and polls backend
   payment status.
6. Provider callback, payment fact application, timeout workers, and recovery
   schedulers own terminal payment/order truth.

Mini Program contract follow-up:

- `weapp/scripts/check-takeout-checkout-rehydration-payment-contract.test.js`
  now locks the order-confirm and payment-result contract for stale snapshot
  rehydration and payment recovery.
- The check proves event-channel snapshots are draft-only until backend
  `calculateCart` replaces pricing, delivery-fee, and payment-assessment truth.
- The check proves `pricingError` blocks order creation, partial order creation
  sends the customer to the durable order list, payment-create failure sends the
  customer to the created order detail, and payment-result reload/re-entry polls
  backend payment truth before rendering terminal status.
- The check also keeps the active takeout order/payment wrapper copies and the
  shared payment-result payment wrapper aligned around order create, order
  detail, payment create, payment detail, and payment query endpoints.

## Evidence Anchors

- Takeout checkout slice:
  `artifacts/codegraph/customer-state-flows/customer-takeout-cart-checkout-payment.slice.md`.
- Event-channel snapshot:
  `weapp/miniprogram/pages/takeout/cart/index.ts:633`.
- Snapshot parsing and fallback timer:
  `weapp/miniprogram/pages/takeout/order-confirm/index.ts:78` through `:104`.
- Backend cart rehydration:
  `weapp/miniprogram/pages/takeout/order-confirm/index.ts:175` through `:227`.
- Backend delivery-fee/pricing calculation:
  `weapp/miniprogram/pages/takeout/order-confirm/index.ts:291` through `:375`.
- Submit guard, order create, and payment create:
  `weapp/miniprogram/pages/takeout/order-confirm/index.ts:423` through `:555`.
- Payment result polling:
  `weapp/miniprogram/pages/payment/result/index.ts:77` through `:157`.
- Backend cart/order/payment routes:
  `locallife/api/server.go:1009`, `locallife/api/server.go:1105`, and
  `locallife/api/server.go:1540`.
- Shared callback/recovery boundary:
  `locallife/api/server.go:548` and `locallife/worker/task_order_timeout.go:55`.

## Source-Audit Questions

| Question | Required answer before code changes |
| --- | --- |
| Can a stale event-channel snapshot create an order with stale price/address truth? | Prove backend cart reload/calculation and order create validation are authoritative. |
| Can duplicate submit create duplicate orders or duplicate payment orders? | Prove frontend guard plus backend constraints/status behavior under concurrent or retried submit. |
| Can payment callback/recovery advance visible order state after client leaves result page? | Prove payment fact application and result/detail reads converge independently of `wx.requestPayment`. |
| What happens if order create succeeds but payment create fails? | Prove partial success copy/readback leads user to existing order rather than repeated blind order creation. |
| Are copied customer wrappers in sync? | Audit active `_main_shared/api/order.ts` and `payment.ts` copies before contract changes. |

## Focused Validation To Add Or Run

From `locallife/`:

```bash
go test ./api -run 'Test.*Order|Test.*PaymentOrder|Test.*PaymentCallback' -count=1
go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication|TestPaymentOrderService|TestCreateOrder' -count=1
go test ./db/sqlc -run 'TestCreateOrderTx|TestProcessPaymentSuccessTx_OrderSetsPaidFields|TestPaymentOrder' -count=1
go test ./worker -run 'Test.*Payment.*Timeout|TestPaymentRecoverySchedulerRunOnce' -count=1
```

From `weapp/`, add or run focused contract scripts for:

```bash
npm run check:takeout-checkout-rehydration-payment-contract
```

This covers:

- stale event-channel snapshot -> backend cart rehydration -> submit guard
- order create -> payment create failure -> durable order detail recovery
- pending payment result -> backend payment query/polling -> result/detail/list
  recovery
- wrapper-copy drift for active order/payment APIs

## Remaining Real Issue

Customer takeout checkout now has a Mini Program contract proof for stale draft
rehydration, pricing-error submit blocking, payment-create failure recovery, and
payment-result re-entry readback. Remaining proof gaps are backend duplicate
submit across order/payment creation, real provider callback/recovery evidence,
and an actual end-to-end run that shows order detail/list visibility after the
client leaves the payment result page.
