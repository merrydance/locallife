# Customer Takeout Cart Checkout Payment Slice

Status: customer-state flow slice created 2026-06-14
Risk class: G3 boundary - customer order creation, payment initiation, address/price truth, duplicate submission, provider callback/recovery
Scope: takeout item/detail/cart/order-confirm/payment-result pages -> cart/order/payment API wrappers -> `/v1/cart`, `/v1/orders`, `/v1/payments`, `/v1/refunds` routes -> order/cart/payment SQL and callback/timeout/recovery boundary

## Variant Coverage

This slice covers:

- Multi-merchant cart list, cart summary, per-merchant load, item add/update/delete/clear, local subtotal recalc, backend delivery-fee calculation, checkout selection, and event-channel checkout snapshot.
- Order confirm page snapshot receive, backend cart rehydration, address selection, delivery fee/order calculation, payment method selection, membership balance option, duplicate submit guard, order create, payment create, and payment result navigation.
- Payment result page pending-confirmation, refresh, backend query, detail navigation, and retry/action handling.
- Backend `/v1/cart`, `/v1/orders`, `/v1/payments`, `/v1/refunds`, order/payment SQL, timeout workers, callback routes, and recovery schedulers as convergence boundaries.

This slice does not fully cover:

- Merchant order acceptance/kitchen/preparation and rider delivery lifecycle; those are merchant/rider/operator flows.
- Payment provider correctness, callback signature verification, and fact application details; referenced as payment-domain boundary.
- Generic manual refund UI; current ordinary customer refund visibility is covered through cancellation/refund detail and payment detail in after-sales slices.
- Discovery before item is added to cart; covered by `customer-discovery-and-merchant-browse.slice.md`.

## Product Invariant

Takeout checkout must converge on backend durable truth before money moves:

- Local cart snapshots are convenience state, never final pricing truth.
- Order confirm must rehydrate cart/address/pricing and use backend order/cart calculation before submit.
- Duplicate submits are guarded locally and must also be safe through backend order/payment idempotency/status constraints.
- `wx.requestPayment` is not terminal truth. Final customer-visible payment status comes from backend payment query/callback/fact/recovery state.
- Paid order state is the boundary that later merchant/rider/order fulfillment flows consume.

## Primary Forward Chain

1. Customer cart and order confirm pages are declared as takeout subpackages, and payment result is declared as the shared customer payment result page.
   Evidence: `weapp/miniprogram/app.json:110`, `weapp/miniprogram/app.json:126`, `weapp/miniprogram/app.json:144`.

2. Cart page loads all carts on entry/show, dedupes short-window reloads, loads per-merchant cart rows, calculates backend delivery fees, keeps local subtotal, and navigates selected carts to order confirm via event channel.
   Evidence: `weapp/miniprogram/pages/takeout/cart/index.ts:21`, `weapp/miniprogram/pages/takeout/cart/index.ts:60`, `weapp/miniprogram/pages/takeout/cart/index.ts:172`, `weapp/miniprogram/pages/takeout/cart/index.ts:250`, `weapp/miniprogram/pages/takeout/cart/index.ts:441`, `weapp/miniprogram/pages/takeout/cart/index.ts:630`.

3. Cart API wrappers call `/v1/cart` for cart reads, summaries, item mutations, clear, calculate, user carts, and combined checkout preview.
   Evidence: `weapp/miniprogram/api/cart.ts:180`, `weapp/miniprogram/api/cart.ts:201`, `weapp/miniprogram/api/cart.ts:214`, `weapp/miniprogram/api/cart.ts:230`, `weapp/miniprogram/api/cart.ts:247`, `weapp/miniprogram/api/cart.ts:264`, `weapp/miniprogram/api/cart.ts:279`, `weapp/miniprogram/api/cart.ts:291`, `locallife/api/server.go:1540`.

4. Order confirm receives checkout snapshot, falls back to `loadCart`, calculates delivery fee through backend cart calculation, updates payment-method view, validates address/price errors, and guards duplicate submit.
   Evidence: `weapp/miniprogram/pages/takeout/order-confirm/index.ts:32`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:81`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:100`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:175`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:313`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:385`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:428`.

5. Order confirm creates the order, then either navigates to payment result for already-paid/balance-paid status or creates a payment order and runs the payment workflow.
   Evidence: `weapp/miniprogram/pages/takeout/order-confirm/index.ts:462`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:492`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:527`, `weapp/miniprogram/pages/takeout/order-confirm/index.ts:542`, `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/api/order.ts:349`, `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/api/payment.ts:590`, `weapp/miniprogram/pages/takeout/order-confirm/_main_shared/services/payment-workflow.ts:18`.

6. Payment result page normalizes workflow status, shows pending-confirmation when necessary, refreshes by querying backend payment status, and navigates to order/detail follow-up actions.
   Evidence: `weapp/miniprogram/pages/payment/result/index.ts:24`, `weapp/miniprogram/pages/payment/result/index.ts:50`, `weapp/miniprogram/pages/payment/result/index.ts:77`, `weapp/miniprogram/pages/payment/result/index.ts:112`, `weapp/miniprogram/pages/payment/result/index.ts:124`, `weapp/miniprogram/pages/payment/result/index.ts:193`, `weapp/miniprogram/pages/payment/_main_shared/api/payment.ts:583`.

7. Backend cart routes map to cart handlers and cart SQL for cart creation/read/item mutation/clear/calculate/user cart summary.
   Evidence: `locallife/api/server.go:1540`, `locallife/api/cart.go:70`, `locallife/api/cart.go:206`, `locallife/api/cart.go:302`, `locallife/api/cart.go:351`, `locallife/api/cart.go:423`, `locallife/api/cart.go:562`, `locallife/api/cart.go:910`, `locallife/api/cart.go:1039`, `locallife/db/query/cart.sql:1`, `locallife/db/query/cart.sql:46`, `locallife/db/query/cart.sql:131`, `locallife/db/query/cart.sql:201`.

8. Backend order routes map to customer create/list/get/cancel/urge/replace/confirm/calculate handlers and order SQL.
   Evidence: `locallife/api/server.go:1009`, `locallife/api/order.go:554`, `locallife/api/order.go:698`, `locallife/api/order.go:806`, `locallife/api/order.go:881`, `locallife/api/order.go:935`, `locallife/api/order.go:983`, `locallife/api/order.go:1051`, `locallife/api/order.go:2786`, `locallife/db/query/order.sql:1`, `locallife/db/query/order.sql:48`, `locallife/db/query/order.sql:77`, `locallife/db/query/order.sql:224`.

9. Backend payment/refund routes map to payment order and refund order handlers/SQL; provider callbacks and recovery workers converge terminal status.
   Evidence: `locallife/api/server.go:1105`, `locallife/api/payment_order.go:323`, `locallife/api/payment_order.go:389`, `locallife/api/payment_order.go:642`, `locallife/api/payment_order.go:783`, `locallife/api/payment_order.go:923`, `locallife/api/payment_order.go:1202`, `locallife/db/query/payment_order.sql:1`, `locallife/db/query/payment_order.sql:76`, `locallife/db/query/payment_order.sql:153`, `locallife/db/query/refund_order.sql:1`, `locallife/api/server.go:548`, `locallife/api/payment_callback.go:1156`, `locallife/worker/task_payment_timeout.go:56`, `locallife/worker/task_order_timeout.go:55`.

## SQL And Durable State Boundaries

- `carts` and `cart_items`: customer cart grouping, item quantities, dish/combo/customization references.
- `orders` and order item tables: submitted order, status, price, delivery fee, address, voucher, membership/balance usage, cancellation/replacement, and fulfillment state.
- `payment_orders`: payment amount, business type, order/reservation/recharge/claim linkage, provider ids, local status, expires_at, and processed_at.
- `refund_orders` and refund idempotency table: refund request, amount, status, provider refund id, and duplicate-request guard.
- `user_addresses`: checkout address reference and delivery eligibility.
- `memberships` and membership transactions: balance payment/discount boundary when selected.
- `vouchers` and `user_vouchers`: coupon/voucher application boundary.
- Provider fact/command tables: callback/fact/recovery state for payment-domain convergence.

## Trust, Authorization, And Tenant Checks

- Cart/order/payment routes are under authenticated `authGroup`; handlers must use the authenticated user id for cart/order ownership.
- Order create must validate merchant/item/orderability/pricing/address/server-calculated totals rather than trusting client totals.
- Payment create/query/list must only expose current user's payment orders except provider/internal callbacks.
- Refund creation must validate payment ownership, amount limits, status, business type, and `Idempotency-Key` where required.
- Provider webhooks validate signature/ownership separately from customer-authenticated routes.

## Idempotency And Duplicate-Submit Checks

- Cart page and order confirm page use local in-flight flags to avoid duplicate taps.
- Cart item SQL should converge repeated item/customization/combo writes through existing item lookup/update paths.
- Payment workflow treats missing/terminal pay params and pending confirmation as backend states, not client terminal success.
- Refund create API requires and bounds `Idempotency-Key` in backend tests; duplicate provider callbacks/facts use payment-domain dedupe.
- Timeout workers close stale pending payment/order/reservation payment records instead of leaving client-only pending state.

## Recovery And Async Convergence Paths

- Cart/order confirm can reload from backend on page show, pull refresh, missing snapshot, address change, payment-method change, and calculation failure retry.
- Payment result refresh queries backend and can move pending confirmation into paid/failed/closed/refunded view.
- Payment callbacks and fact application update durable payment/order state after provider confirmation.
- Payment timeout workers and Baofu/payment recovery schedulers re-query/close stale pending payments when callbacks are delayed or missing.
- Order fulfillment after paid state is outside this slice and handled by merchant/rider/operator flows.

## Frontend Draft And Backend Rehydration

- Event-channel checkout snapshot is draft state. Order confirm rehydrates through `loadCart` when absent/stale and recalculates backend fees before submit.
- Address selection is local until referenced in order create; backend address ownership and delivery calculation remain authoritative.
- Payment method selection is local until order/payment creation. Backend payment capabilities and order status decide whether payment is needed.
- Payment result URL params are display hints. Backend payment query/ledger is the source of status.

## Test Coverage Signals

Observed tests:

- `locallife/api/payment_order_test.go` covers refund create/get/list idempotency and validation branches.
- `locallife/api/payment_callback_notification_test.go` covers payment-success notification enqueue failure logging.
- `locallife/worker/task_payment_timeout.go` and `task_order_timeout.go` have worker test surface in the payment/order domain.
- CodeGraph query `createOrderFromCart` found active wrappers in takeout order confirm, dine-in, orders, and user-center copies.

Missing high-value tests:

- Mini Program checkout regression for stale event-channel snapshot -> backend cart rehydration -> submit.
- End-to-end takeout order create -> payment create -> pending result -> callback/fact -> order paid -> result refresh.
- Duplicate submit test across frontend guard and backend payment/order idempotency.
- Membership balance versus WeChat payment split behavior from customer checkout.

## Gaps And Refactor Notes

- Payment/order API wrappers are copied under multiple page-local `_main_shared` folders. Any contract change needs a customer-wide wrapper audit.
- Generic refund create exists as API/wrapper surface, but current ordinary customer UI mainly reaches refunds through system-triggered cancellation/modification flows.
- Payment provider closure needs provider-domain evidence. Do not claim real funds-action success from customer page verification alone.

## Branch Exhaustion

- Entry branches checked: cart direct open, item detail cart handoff, restaurant detail cart handoff, order confirm with event snapshot, order confirm rehydrate, address select/edit, payment result direct open.
- Request branches checked: `/v1/cart`, `/summary`, `/user-carts`, `/combined-checkout/preview`, `/items`, `/calculate`, `/v1/orders`, `/calculate`, `/:id/cancel`, `/:id/replace`, `/:id/urge`, `/:id/confirm`, `/v1/payments`, `/combined`, `/:id/query`, `/ledger`, `/v1/refunds`.
- Backend state branches checked: empty cart, multi-merchant cart, missing address, pricing error, already-paid/balance-paid order, pending payment, failed/closed payment, refund pending/success/failure boundary.
- Async branches checked: payment callback, refund callback, payment timeout worker, order payment timeout worker, payment fact/recovery scheduler boundary.
- Dead/orphan branches checked: no customer cart/order-confirm/payment-result entry omitted; provider callback internals remain payment-domain-owned.
