# State Sequencing Audit Card: Customer Takeout Checkout Rehydration And Payment

Date: 2026-06-15
Risk theme: state sequencing / idempotency and retry / transaction consistency
Risk class: G3 - customer cart, order creation, payment callback/recovery, visible order status
Status: source-audited, Mini Program contract covered, backend payment-create proof covered, backend order-create guard covered when `Idempotency-Key` is supplied, frontend stable-key propagation covered, provider proof pending

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

Backend payment-create follow-up:

- `locallife/db/sqlc/tx_create_partner_payment.go` locks the target order with
  `GetOrderForUpdate` before it checks for an existing pending `payment_orders`
  row and inserts a Baofu aggregate payment order.
- `locallife/db/sqlc/tx_create_partner_payment_test.go` now covers concurrent
  same-order `CreatePartnerPaymentTx` calls: exactly one transaction creates a
  pending Baofu aggregate `payment_orders` row and the loser receives a
  request-level `409 Conflict` with `pending payment order`.
- `locallife/logic/payment_order_service_test.go` now covers the upstream
  service boundary: a transaction-level pending-payment conflict is mapped to
  `409` / `支付订单状态已变化，请刷新后重试` and does not call Baofu unified order
  or create an external payment command.
- This proves duplicate payment-order creation for one already-created order is
  transaction-owned. It does not prove request-level idempotency for duplicate
  order creation before an order id exists.

Backend order-create idempotency follow-up:

- `POST /v1/orders` accepts an optional `Idempotency-Key` header. When callers
  supply a stable key, `OrderService.CreateOrder` canonicalizes the
  actor/order target/items/options into a request hash and passes the scoped
  key/hash to `CreateOrderTx`.
- `locallife/db/migration/000270_add_order_create_idempotency.up.sql` adds
  `order_create_request_idempotency` with a unique
  `(operation_scope, actor_user_id, idempotency_key)` guard. `CreateOrderTx`
  creates or locks the guard row, replays the bound `order_id` for the same
  hash, rejects same-key/different-hash reuse with `409 Conflict`, and binds
  the created order inside the same transaction.
- Focused backend tests cover sequential replay/conflict, concurrent same-key
  single-order creation, API header propagation, logic conflict mapping, and
  skipping duplicate payment-timeout scheduling on replay.
- This closes the backend duplicate order-create window only for callers that
  reuse a stable key. The Mini Program takeout submit path now generates and
  reuses a stable key for the same pending order-create request, passes it to
  `POST /v1/orders`, clears it after an order id is returned, and preserves it
  across unknown/network failures before an order id exists.

Mini Program order-create idempotency follow-up:

- `weapp/miniprogram/pages/takeout/order-confirm/_services/takeout-order-create-idempotency.ts`
  canonicalizes the create-order request into a stable signature, stores one
  pending `Idempotency-Key` in local storage for that signature, rotates it when
  the request payload changes, and clears the consumed key after order creation
  returns an order id. The local signature is a digest rather than plaintext
  order payload, and the pending key has a 2-hour recovery window so unknown
  network failures can retry safely without allowing long-lived stale replay.
- `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/api/order.ts`
  accepts an optional `idempotencyKey` option and sends it as `Idempotency-Key`
  to `POST /v1/orders`.
- `weapp/miniprogram/pages/takeout/order-confirm/index.ts` builds the request,
  derives the signature, obtains the pending key, passes it to `createOrder`,
  and only clears it after `createOrder` succeeds. If order creation fails
  before a durable order id is returned to the client, the key remains available
  for safe retry/replay.
- `weapp/scripts/check-takeout-checkout-rehydration-payment-contract.test.js`
  now locks the stable signature, same-attempt key reuse, changed-payload key
  rotation, stale-key rotation after the recovery window, key clearing, wrapper
  header propagation, and page-level wiring.
- The Mini Program-generated key is a retry correlation token, not a trust or
  authorization boundary. Production idempotency remains owned by the backend
  actor/key/hash guard and the `CreateOrderTx` transaction binding.

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
- Mini Program order-create idempotency service and wrapper:
  `weapp/miniprogram/pages/takeout/order-confirm/_services/takeout-order-create-idempotency.ts`
  and `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/api/order.ts`.
- Payment result polling:
  `weapp/miniprogram/pages/payment/result/index.ts:77` through `:157`.
- Backend payment creation transaction:
  `locallife/db/sqlc/tx_create_partner_payment.go:44` through `:219`.
- Concurrent payment creation proof:
  `locallife/db/sqlc/tx_create_partner_payment_test.go:221`.
- Logic conflict/no-upstream-call proof:
  `locallife/logic/payment_order_service_test.go:358`.
- Backend order-create request idempotency:
  `locallife/db/sqlc/tx_create_order.go:58` through `:340`,
  `locallife/db/migration/000270_add_order_create_idempotency.up.sql`, and
  `locallife/api/order.go:544`.
- Order-create replay/conflict/concurrency proof:
  `locallife/db/sqlc/tx_create_order_test.go:245`,
  `locallife/logic/order_service_create_test.go:51`, and
  `locallife/api/order_test.go:428`.
- Backend cart/order/payment routes:
  `locallife/api/server.go:1009`, `locallife/api/server.go:1105`, and
  `locallife/api/server.go:1540`.
- Shared callback/recovery boundary:
  `locallife/api/server.go:548` and `locallife/worker/task_order_timeout.go:55`.

## Source-Audit Questions

| Question | Required answer before code changes |
| --- | --- |
| Can a stale event-channel snapshot create an order with stale price/address truth? | Prove backend cart reload/calculation and order create validation are authoritative. |
| Can duplicate submit create duplicate orders or duplicate payment orders? | Payment-order duplication for an already-created order is now covered by transaction and logic tests. Backend order-create duplication is covered when the caller supplies a stable `Idempotency-Key`; Mini Program takeout now generates/reuses that key for the same pending submit/retry attempt. |
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

Focused backend proof now added:

```bash
go test ./db/sqlc -run 'TestCreatePartnerPaymentTx_ConcurrentOrderPaymentAllowsSinglePendingPayment' -count=1
go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder_TxPendingConflictDoesNotCallBaofu' -count=1
go test ./db/sqlc -run 'TestCreateOrderTx_(RequestIdempotencyReplayAndConflict|ConcurrentSameIdempotencyKeyCreatesSingleOrder)' -count=1
go test ./logic -run 'TestOrderServiceCreateOrder_(PassesIdempotencyMetadataAndSkipsTimeoutOnReplay|MapsIdempotencyConflict)' -count=1
go test ./api -run 'TestCreateOrderAPI/PassesIdempotencyKey' -count=1
```

From `weapp/`, add or run focused contract scripts for:

```bash
npm run check:takeout-checkout-rehydration-payment-contract
```

This covers:

- stale event-channel snapshot -> backend cart rehydration -> submit guard
- takeout order-create request signature -> stable `Idempotency-Key` reuse ->
  payload-change key rotation -> stale-key rotation -> consumed-key clearing
- order create -> payment create failure -> durable order detail recovery
- pending payment result -> backend payment query/polling -> result/detail/list
  recovery
- wrapper-copy drift for active order/payment APIs

## Remaining Real Issue

Customer takeout checkout now has a Mini Program contract proof for stale draft
rehydration, pricing-error submit blocking, payment-create failure recovery, and
payment-result re-entry readback. It also has backend proof that repeated
payment creation for the same already-created order cannot create two pending
Baofu payment orders or call Baofu after the transaction reports a pending
payment conflict. Backend order creation now has request-level idempotency when
callers supply a stable `Idempotency-Key`, including same-key replay,
same-key/different-hash `409`, and same-key concurrent single-order proof.
The Mini Program takeout submit path now supplies that stable key and preserves
it for unknown order-create failures before an order id exists. Remaining proof
gaps are real provider callback/recovery evidence and an actual end-to-end run
that shows order detail/list visibility after the client leaves the payment
result page.
