# Customer Related Completeness Audit

Status: re-reviewed and refreshed 2026-06-14
Purpose: answer whether the customer codegraph artifacts exhaust ordinary consumer-facing and consumer-related flows, including Mini Program entrypoints, backend/API-only boundaries, background convergence, and known role-exit paths.

## Verdict

The customer codegraph now exhausts the current ordinary customer-facing and customer-actionable closure as of 2026-06-14: every ordinary consumer Mini Program entrypoint declared in `weapp/miniprogram/app.json:3` through `weapp/miniprogram/app.json:8` and customer subpackages in `weapp/miniprogram/app.json:101` through `weapp/miniprogram/app.json:290` is either directly covered by a customer slice or explicitly excluded as a role-registration/cross-role boundary.

The broader phrase "all customer-related flows" includes payment provider callbacks, refund recovery, merchant/rider/operator actions, platform approvals, claim recovery/dispute enforcement, table QR administration, and notification producers. This audit indexes those touchpoints so they are not silently omitted, while keeping their operational closure owned by the relevant role/domain artifacts.

Re-review correction: the original six business slices covered customer business actions but did not explicitly model the shared runtime identity/session support chain, and the order-tracking artifacts did not fully model delivery tracking plus route-planning reads. This pass adds `customer-runtime-auth-session-support.slice.md` and updates the after-sales graph for `/v1/delivery/order/:id`, `/v1/delivery/:id/rider-location`, `/v1/delivery/:id/track`, and the display-only `/v1/location/direction/bicycling` route proxy.

Remaining issues are implementation or product-alignment backlog, not missing codegraph coverage. They are tracked in `flow-variant-index.md`.

## Customer-Facing Closure Covered

1. Discovery, merchant browse, search, coupons, wanted merchants, and customer role-exit boundaries: `customer-discovery-and-merchant-browse.slice.md`.
2. Takeout carts, checkout, order creation, payment workflow, result page, and callback/timeout/recovery boundary: `customer-takeout-cart-checkout-payment.slice.md`.
3. Dine-in scan, session open/resume/transfer, menu, billing group, checkout, order/payment, and session close boundary: `customer-dine-in-session-menu-checkout.slice.md`.
4. Reservation browse, room availability, create/confirm/detail/list/modify/cancel/payment/refund, and dine-in handoff: `customer-reservation-lifecycle.slice.md`.
5. Order list/detail/tracking, cancel/refund, retry pay, urge, confirm receipt, reviews, service center, claims, and food-safety report: `customer-order-tracking-refund-after-sales.slice.md`.
6. User center, profile, address, wallet, membership, coupons, favorites, notifications, agreements, and review management: `customer-profile-address-wallet-membership-reviews.slice.md`.
7. Runtime authentication/session support, request token refresh/retry, Web login QR confirmation, and authenticated frontend error logging: `customer-runtime-auth-session-support.slice.md`.

## Mini Program Entrypoint Matrix

| app.json entry | Primary slice | Edge node | Verdict |
| --- | --- | --- | --- |
| `pages/takeout/index` | `customer-discovery-and-merchant-browse` | `weapp.takeoutHome` | Covered |
| `pages/reservation/index` | `customer-reservation-lifecycle` | `weapp.reservationHome` | Covered |
| `pages/user_center/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.userCenter` | Covered |
| `pages/dining/index` | `customer-dine-in-session-menu-checkout` | `weapp.diningEntry` | Covered |
| `pages/notification/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.notifications` | Covered |
| `pages/user/bind-merchant/index` | Boundary exit | n/a | Excluded from ordinary-customer closure |
| `pages/orders/detail/index` | `customer-order-tracking-refund-after-sales` | `weapp.orderDetail` | Covered |
| `pages/orders/list/index` | `customer-order-tracking-refund-after-sales` | `weapp.orderList` | Covered |
| `pages/orders/tracking/index` | `customer-order-tracking-refund-after-sales` | `weapp.orderTracking` | Covered |
| `pages/payment/result/index` | `customer-takeout-cart-checkout-payment` | `weapp.paymentResult` | Covered; also referenced by reservation/dine-in/membership payment flows |
| `pages/dine-in/scan-entry/scan-entry` | `customer-dine-in-session-menu-checkout` | `weapp.scanEntry` | Covered |
| `pages/dine-in/menu/menu` | `customer-dine-in-session-menu-checkout` | `weapp.dineInMenu` | Covered |
| `pages/dine-in/checkout/checkout` | `customer-dine-in-session-menu-checkout` | `weapp.dineInCheckout` | Covered |
| `pages/takeout/cart/index` | `customer-takeout-cart-checkout-payment` | `weapp.cart` | Covered |
| `pages/takeout/dish-detail/index` | `customer-discovery-and-merchant-browse` | `weapp.dishDetail` | Covered |
| `pages/takeout/combo-detail/index` | `customer-discovery-and-merchant-browse` | `weapp.comboDetail` | Covered |
| `pages/takeout/order-confirm/index` | `customer-takeout-cart-checkout-payment` | `weapp.orderConfirm` | Covered |
| `pages/takeout/restaurant-detail/index` | `customer-discovery-and-merchant-browse` | `weapp.restaurantDetail` | Covered |
| `pages/takeout/search/index` | `customer-discovery-and-merchant-browse` | `weapp.search` | Covered |
| `pages/takeout/wanted-merchants/index` | `customer-discovery-and-merchant-browse` | `weapp.wantedMerchants` | Covered |
| `pages/takeout/merchant-info/index` | `customer-discovery-and-merchant-browse` | `weapp.merchantInfo` | Covered |
| `pages/takeout/category/index` | `customer-discovery-and-merchant-browse` | `weapp.category` | Covered |
| `pages/reservation/create/index` | `customer-reservation-lifecycle` | `weapp.reservationCreate` | Covered |
| `pages/reservation/confirm/index` | `customer-reservation-lifecycle` | `weapp.reservationConfirm` | Covered |
| `pages/reservation/detail/index` | `customer-reservation-lifecycle` | `weapp.reservationDetail` | Covered |
| `pages/reservation/list/index` | `customer-reservation-lifecycle` | `weapp.reservationList` | Covered |
| `pages/reservation/modify/index` | `customer-reservation-lifecycle` | `weapp.reservationModify` | Covered |
| `pages/reservation/room-detail/index` | `customer-reservation-lifecycle` | `weapp.roomDetail` | Covered |
| `pages/user_center/addresses/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.addresses` | Covered |
| `pages/user_center/addresses/edit/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.addressEdit` | Covered |
| `pages/user_center/coupons/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.coupons` | Covered |
| `pages/user_center/favorites/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.favorites` | Covered |
| `pages/user_center/membership/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.membership` | Covered |
| `pages/user_center/reviews/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.reviewList` | Covered |
| `pages/user_center/reviews/create/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.reviewCreate` | Covered; also after-sales handoff |
| `pages/user_center/wallet/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.wallet` | Covered |
| `pages/user_center/payment-detail/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.paymentDetail` | Covered |
| `pages/user_center/refund-detail/index` | `customer-order-tracking-refund-after-sales` | `weapp.refundDetail` | Covered |
| `pages/user_center/reservations/index` | `customer-reservation-lifecycle` | `weapp.userReservations` | Covered |
| `pages/user_center/agreements/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.agreements` | Covered |
| `pages/user_center/agreements/detail/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.agreementDetail` | Covered |
| `pages/user_center/about_us/index` | `customer-profile-address-wallet-membership-reviews` | `weapp.aboutUs` | Covered as static/profile support surface |
| `pages/user_center/service_center/index` | `customer-order-tracking-refund-after-sales` | `weapp.serviceCenter` | Covered |
| `pages/user_center/service_center/submit/index` | `customer-order-tracking-refund-after-sales` | `weapp.claimSubmit` | Covered |
| `pages/user_center/service_center/detail/index` | `customer-order-tracking-refund-after-sales` | `weapp.claimDetail` | Covered |
| `pages/user_center/service_center/food-safety/index` | `customer-order-tracking-refund-after-sales` | `weapp.foodSafetyReport` | Covered |

## Customer-Related Backend And Background Touchpoints

These touch customer-visible state but are not necessarily customer page entrypoints.

1. Authenticated and auth-support customer route surface under `/v1`.
   Covered across the customer slices: auth/login/refresh/session support, search/public merchant, cart, orders, payments, refunds, delivery tracking reads, tracking route-planning display reads, reservations, dining sessions, claims, food-safety, profile, notifications, vouchers, favorites, reviews, and memberships.

2. Provider callbacks and payment facts.
   Covered as convergence boundaries in payment-bearing slices. Webhooks for WeChat/Baofu payment/refund are registered at `locallife/api/server.go:548` through `locallife/api/server.go:557`; detailed provider correctness remains payment-domain-owned.

3. Payment timeout and recovery workers/schedulers.
   Covered as recovery dependencies: `worker/task_payment_timeout.go`, `worker/task_order_timeout.go`, `worker/task_reservation_timeout.go`, and Baofu/payment fact recovery schedulers affect customer-visible pending states.

4. Refund recovery.
   Covered in order/reservation/after-sales slices as customer-visible pending/refund-detail state. Provider callbacks and fact application own terminal truth.

5. Merchant/rider/order fulfillment actions.
   Customer order detail and tracking can display/cancel/urge/confirm/reorder, but merchant accept/prepare/complete, rider grab/pick/deliver, dispatch, and operator escalation are cross-role boundaries.

6. Claim and food-safety downstream enforcement.
   Customer service center covers submit, detail, continue, withdraw, and payout confirmation. Merchant/rider recovery, disputes, operator food-safety investigation, and behavior action recovery are cross-role/background closure.

7. Table QR and merchant room/menu administration.
   Customer dine-in/reservation covers scan/open/menu/room availability. QR generation, table configuration, dish/combo management, room inventory, and merchant reservation workbench are merchant-owned.

8. Media and profile assets.
   Customer profile/reviews can upload avatar/review images through shared media routes. Media moderation, private access, and lifecycle remain media-domain boundaries unless the customer page behavior changes.

9. Runtime auth/session support.
   Mini Program cold start, request token refresh, Web login confirmation, and frontend error logs are covered as shared customer runtime support. Web-side QR creation/consume, app-bind verification, and invite-code onboarding remain cross-role/account boundaries.

## Dead, Stale, Or Non-Current Customer-Related Code

1. Role-registration entries from customer pages.
   Status: explicit boundary exit; not missing customer coverage.

2. User center invite/bind merchant scan branch.
   Status: reachable from ordinary user center scan, but it mutates cross-role account state through `/v1/bind-merchant`; documented as a boundary exit in `customer-runtime-auth-session-support.slice.md` and `flow-variant-index.md`.

3. Generic `/v1/refunds` create wrappers.
   Status: API boundary and tested backend surface; current customer pages mostly reach refund through order cancellation, reservation modification/cancellation, or system workflows rather than a generic refund form.

4. `/v1/history/browse`.
   Status: backend/API wrapper boundary with no current ordinary customer page entry; `customer-discovery-and-merchant-browse` covers search history, while browse-history UI would need a future page-specific slice if it lands.

5. Page-local duplicate API wrappers.
   Status: live but duplicated; tracked in `flow-variant-index.md` because contract changes can drift between page groups.

6. Payment result page reuse.
   Status: covered as common customer payment confirmation UI; each payment-bearing slice references its own entry into payment workflow.

7. Cross-role notification producers.
   Status: customer notification list/read/preferences are covered; creation of merchant/rider/operator/platform notifications belongs to producer slices.

## Residual Non-Closure

No additional ordinary customer Mini Program page entrypoint was found missing after this pass. The re-review did find and patch artifact-level omissions: the shared runtime auth/session support chain, delivery-tracking route edges, and the tracking map-route proxy edge in the after-sales graph.

Residual work remains, but it is not missing codegraph coverage:

- Consolidation or strict synchronization of page-local `_main_shared/api` wrappers, especially copied wrappers that expose merchant/rider/operator functions inside customer-reachable directories.
- Focused E2E coverage for payment pending-confirmation -> callback/fact -> result refresh across takeout, dine-in, reservation, membership, and claim recovery.
- Focused E2E coverage for cold-start token reuse/refresh/full-login fallback, request 401 refresh retry, and Web login QR confirmation branches.
- Explicit customer UX decisions for generic refund initiation versus system-triggered refund visibility.
- Cross-role action-loop design for claim recovery, food-safety handling, dispatch escalation, and merchant/rider dispute paths.
- Provider-contract evidence before claiming real funds-action success for payment/refund/provider recovery.

Those items are deliberately durable in `flow-variant-index.md` so they survive handoff, model context switching, and future implementation planning.
