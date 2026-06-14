# Customer Order Tracking Refund After Sales Slice

Status: customer-state flow slice created 2026-06-14
Risk class: G3 boundary - customer order status, delivery tracking, cancellation/refund, receipt confirmation, reviews, claims, food-safety escalation, claim payout confirmation
Scope: orders list/detail/tracking, payment/refund detail, service center, claim submit/detail, food-safety report, review handoff -> order/payment/refund/claim/food-safety/review backend routes -> order/payment/refund/trust/review SQL -> recovery/provider/cross-role boundaries

## Variant Coverage

This slice covers:

- Customer order list with filters/select mode, pagination, cancel, retry pay, reorder, and order detail navigation.
- Customer order detail with order/reservation bundle read, cancel/refund workflow, urge, retry pay, confirm receipt, tracking navigation, support center, food-safety report, reorder, review create/update handoff.
- Customer tracking page with delivery state refresh, rider location display, backend-proxied bicycling route planning, and confirm receipt.
- Payment detail/refund detail as customer-visible payment/refund progress surfaces.
- Service center list, claim submit, duplicate existing claim detection, claim detail, confirm continue, withdraw, payout confirmation, and food-safety report.
- Backend order/payment/refund/delivery-tracking/location-route/claim/food-safety/review routes, durable SQL state, and map-provider boundary.

This slice does not fully cover:

- Merchant/rider/operator fulfillment actions after customer submit/urge/cancel/claim.
- Provider refund/payment internals; referenced as payment-domain boundary.
- Operator food-safety investigation/resolution and automatic recovery/dispute enforcement.
- Merchant/rider claim recovery payment or dispute flows.

## Product Invariant

After-sales pages are customer request and visibility surfaces, not all downstream closure:

- Customer pages must read order/payment/refund/claim status from backend durable state.
- Cancel, confirm receipt, urge, claim submit, food-safety report, continue, withdraw, and payout confirmation are the customer-owned writes in this slice.
- Refund success, payment success, payout, recovery, and food-safety resolution may converge asynchronously through provider callbacks, workers, schedulers, merchant/rider/operator/platform actions.
- Tracking location/route state is informational. The delivery lifecycle transitions are rider/merchant/backend-owned, while bicycling route planning is a backend-proxied map-provider read used only for display.
- Reviews must be tied to customer-owned orders and visible review status.

## Primary Forward Chain

1. Customer Mini Program declares order list/detail/tracking, payment result, refund detail, reviews create/list, and service center pages.
   Evidence: `weapp/miniprogram/app.json:101`, `weapp/miniprogram/app.json:104`, `weapp/miniprogram/app.json:105`, `weapp/miniprogram/app.json:106`, `weapp/miniprogram/app.json:241`, `weapp/miniprogram/app.json:260`, `weapp/miniprogram/app.json:285`.

2. Orders list loads paginated orders, applies filters/select mode, cancels orders with refund expectation copy, starts retry payment, and supports reorder.
   Evidence: `weapp/miniprogram/pages/orders/list/index.ts:34`, `weapp/miniprogram/pages/orders/list/index.ts:60`, `weapp/miniprogram/pages/orders/list/index.ts:123`, `weapp/miniprogram/pages/orders/list/index.ts:351`, `weapp/miniprogram/pages/orders/list/index.ts:368`, `weapp/miniprogram/pages/orders/list/index.ts:371`, `weapp/miniprogram/pages/orders/list/index.ts:404`, `weapp/miniprogram/pages/orders/list/index.ts:446`.

3. Order detail loads order/reservation bundle, preserves cancellation/refund UI state during refresh, computes actions, navigates to support center/food safety/tracking/review, urges, retries payment, confirms receipt, and reloads durable order state.
   Evidence: `weapp/miniprogram/pages/orders/detail/index.ts:32`, `weapp/miniprogram/pages/orders/detail/index.ts:64`, `weapp/miniprogram/pages/orders/detail/index.ts:87`, `weapp/miniprogram/pages/orders/detail/index.ts:94`, `weapp/miniprogram/pages/orders/detail/index.ts:114`, `weapp/miniprogram/pages/orders/detail/index.ts:252`, `weapp/miniprogram/pages/orders/detail/index.ts:278`, `weapp/miniprogram/pages/orders/detail/index.ts:354`, `weapp/miniprogram/pages/orders/detail/index.ts:364`, `weapp/miniprogram/pages/orders/detail/index.ts:396`, `weapp/miniprogram/pages/orders/detail/index.ts:408`, `weapp/miniprogram/pages/orders/detail/index.ts:425`.

4. Tracking page loads delivery/order data, route/rider location, refreshes tracking state, plans route through the backend location proxy, and can confirm receipt through the same recovery-aware confirmation service.
   Evidence: `weapp/miniprogram/pages/orders/tracking/index.ts:56`, `weapp/miniprogram/pages/orders/tracking/index.ts:95`, `weapp/miniprogram/pages/orders/tracking/index.ts:134`, `weapp/miniprogram/pages/orders/tracking/index.ts:199`, `weapp/miniprogram/pages/orders/tracking/index.ts:319`, `weapp/miniprogram/pages/orders/tracking/index.ts:372`, `weapp/miniprogram/pages/orders/tracking/index.ts:397`, `weapp/miniprogram/api/location.ts:112`, `locallife/api/server.go:688`, `locallife/api/location.go:212`.

5. Payment detail and refund detail read payment/refund state, list refunds, navigate to refund detail, and poll non-terminal refund state until terminal or timeout.
   Evidence: `weapp/miniprogram/pages/user_center/payment-detail/index.ts:48`, `weapp/miniprogram/pages/user_center/payment-detail/index.ts:89`, `weapp/miniprogram/pages/user_center/payment-detail/index.ts:255`, `weapp/miniprogram/pages/user_center/payment-detail/index.ts:328`, `weapp/miniprogram/pages/user_center/refund-detail/index.ts:11`, `weapp/miniprogram/pages/user_center/refund-detail/index.ts:46`, `weapp/miniprogram/pages/user_center/refund-detail/index.ts:69`, `weapp/miniprogram/pages/user_center/refund-detail/index.ts:83`, `weapp/miniprogram/pages/user_center/refund-detail/index.ts:132`.

6. Service center lists customer claims, opens claim submit/detail, and routes food-safety report to a separate food-safety page.
   Evidence: `weapp/miniprogram/pages/user_center/service_center/index.ts:71`, `weapp/miniprogram/pages/user_center/service_center/index.ts:98`, `weapp/miniprogram/pages/user_center/service_center/index.ts:129`, `weapp/miniprogram/pages/user_center/service_center/index.ts:145`, `weapp/miniprogram/pages/user_center/service_center/index.ts:153`.

7. Claim submit page loads order candidates/existing claims, submits claim, handles customer-action-required result, confirm-continue, withdraw, and detail navigation.
   Evidence: `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:50`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:86`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:141`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:233`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:238`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:276`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:284`, `weapp/miniprogram/pages/user_center/service_center/submit/index.ts:339`.

8. Claim detail and food-safety report pages read/update claim or food-safety durable state and support customer confirmation actions.
   Evidence: `weapp/miniprogram/pages/user_center/service_center/detail/index.ts:150`, `weapp/miniprogram/pages/user_center/service_center/detail/index.ts:170`, `weapp/miniprogram/pages/user_center/service_center/detail/index.ts:179`, `weapp/miniprogram/pages/user_center/service_center/detail/index.ts:214`, `weapp/miniprogram/pages/user_center/service_center/detail/index.ts:252`, `weapp/miniprogram/pages/user_center/service_center/detail/index.ts:271`, `weapp/miniprogram/pages/user_center/service_center/food-safety/index.ts:35`, `weapp/miniprogram/pages/user_center/service_center/food-safety/index.ts:65`, `weapp/miniprogram/pages/user_center/service_center/food-safety/index.ts:151`, `weapp/miniprogram/pages/user_center/service_center/food-safety/index.ts:155`.

9. Backend order/payment/refund/delivery-tracking/location-route/claim/food-safety/review routes and handlers own durable writes, read projections, and map-provider display reads.
   Evidence: `locallife/api/server.go:1009`, `locallife/api/server.go:1105`, `locallife/api/server.go:1122`, `locallife/api/server.go:688`, `locallife/api/server.go:1206`, `locallife/api/server.go:1515`, `locallife/api/server.go:1527`, `locallife/api/server.go:1597`, `locallife/api/order.go:698`, `locallife/api/order.go:806`, `locallife/api/order.go:881`, `locallife/api/order.go:935`, `locallife/api/order.go:983`, `locallife/api/order.go:1051`, `locallife/api/delivery.go:785`, `locallife/api/delivery.go:877`, `locallife/api/delivery.go:967`, `locallife/api/location.go:212`, `locallife/api/payment_order.go:1318`, `locallife/api/risk_management.go:327`, `locallife/api/risk_management.go:890`, `locallife/api/risk_management.go:1603`, `locallife/api/risk_management.go:1684`, `locallife/api/risk_management.go:1745`, `locallife/api/risk_management.go:1802`, `locallife/api/risk_management.go:1874`, `locallife/api/review.go:80`, `locallife/api/review_owner.go:40`.

## SQL And Durable State Boundaries

- `orders`, order items, deliveries: customer order status, cancellation, replacement, receipt confirmation, tracking status, rider/merchant delivery data.
- `payment_orders`: retry payment, payment detail, ledger, refund linkage.
- `refund_orders` and refund idempotency: cancellation/refund request/progress/terminal state.
- `reviews` and review images: customer review create/update/delete/read.
- `claims`, claim decisions/recoveries/behavior actions, and trust-score tables: claim submit, customer action required, warning/continue, withdraw, payout status.
- `food_safety_cases` and `food_safety_incidents`: food-safety report and downstream investigation boundary.
- `notifications`: customer notification of order/payment/refund/claim outcomes when produced.

## Trust, Authorization, And Tenant Checks

- Customer order, payment, refund, claim, and review reads must be scoped to the authenticated user.
- Cancel/confirm/urge/replace/review must validate order ownership and allowed status.
- Claim submit must validate selected order belongs to the user and avoid duplicate existing claim for the same order where required.
- Food-safety report must validate order/merchant/user relationship and avoid duplicate open incident where backend enforces it.
- Payout confirmation and confirm-continue/withdraw must validate claim ownership and pending customer action.
- Provider callbacks and merchant/rider/operator actions are separate trust boundaries.

## Idempotency And Duplicate-Submit Checks

- Orders list/detail and tracking use local in-flight/confirmation UI for cancel/pay/confirm actions.
- Urge uses local cooldown countdown and backend action status.
- Claim submit guards `submitting`, `canSubmit`, duplicate selected order, and customer-action result actions.
- Backend refund create requires `Idempotency-Key`; provider callbacks/facts dedupe in payment domain.
- Food-safety submit uses local `submitting` and backend duplicate/open incident checks.

## Recovery And Async Convergence Paths

- Orders list/detail reload durable order state after cancel/pay/confirm/reorder.
- Refund detail waits/polls until refund terminal or retry refresh.
- Payment detail refreshes payment/refund lists and can route to refund detail.
- Tracking refreshes delivery state, rider location, and backend-proxied bicycling route display, but durable delivery transitions come from backend/rider flows.
- Claim payout/recovery actions may be applied by workers/schedulers after customer confirmation; service center detail reloads backend claim state.
- Food-safety downstream operator/merchant/order recovery is cross-role/background closure.

## Frontend Draft And Backend Rehydration

- Cancel reason, claim reason, food-safety description/type, selected order candidate, and payout real-name readiness are frontend drafts.
- Order/refund/payment/claim/food-safety status are backend-rehydrated.
- Tracking route points, map fallback lines, and rider phone masking are presentation state; delivery state is backend-owned.
- Review draft text/images are local until media upload and review create/update persist.

## Test Coverage Signals

Observed tests:

- `locallife/api/payment_order_test.go` covers refund create/idempotency/get/list.
- `locallife/api/recovery_dispute_test.go`, claim recovery logic tests, and payment fact application tests cover downstream claim/recovery boundaries.
- `locallife/api/payment_callback_notification_test.go` covers payment notification enqueue failure logging.
- `locallife/scheduler/stale_delivery_cleanup_test.go` covers stale delivery/order cancellation signals.

Missing high-value tests:

- Customer order cancel -> refund pending -> refund callback/recovery -> refund detail terminal.
- Customer confirm receipt from detail and tracking with backend recovery behavior.
- Claim submit duplicate detection and confirm-continue/withdraw from result and detail pages.
- Food-safety report from order detail into service center and downstream operator visibility.
- Order urge cooldown and backend duplicate semantics.

## Gaps And Refactor Notes

- Service center mixes ordinary claims and food-safety entry; downstream ownership is intentionally separated in docs.
- Generic refund API wrappers should not be treated as visible manual refund UI without a concrete page.
- Tracking map and bicycling-route data are presentation-only; never use client route/rider data as fulfillment truth.

## Branch Exhaustion

- Entry branches checked: orders list, order detail, tracking, payment detail, refund detail, service center index, claim submit, claim detail, food-safety report, review create handoff.
- Request branches checked: `/v1/orders`, `/:id`, `/:id/cancel`, `/:id/replace`, `/:id/urge`, `/:id/confirm`, `/v1/payments`, `/:id/refunds`, `/v1/refunds/:id`, `/v1/delivery/order/:order_id`, `/v1/delivery/:delivery_id/rider-location`, `/v1/delivery/:delivery_id/track`, `/v1/location/direction/bicycling`, `/v1/claims`, `/:id/confirm-continue`, `/:id/withdraw`, `/:id/payout-confirmation`, `/v1/food-safety/report`, `/v1/reviews`.
- Backend state branches checked: unpaid retry pay, paid/cancelled/refund expected, delivery tracking, completed/confirmable, reviewed/unreviewed, claim accepted/rejected/warned waiting confirmation/withdrawn, payout processing/paid, food-safety duplicate/open boundary.
- Async branches checked: refund callback/recovery, payment fact/recovery, claim behavior action recovery, stale delivery cleanup, map-provider route failure display fallback, food-safety operator downstream.
- Dead/orphan branches checked: no customer after-sales page omitted; merchant/rider/operator recovery/dispute actions excluded by boundary.
