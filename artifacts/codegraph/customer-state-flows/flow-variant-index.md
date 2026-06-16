# Customer Flow Variant Index

Status: re-reviewed and refreshed 2026-06-14
Purpose: cross-slice branch, drift, no-entry, duplicate-wrapper, and recovery checklist for the ordinary customer-side codegraph. Detailed evidence remains in the individual `*.slice.md` files.
Boundary note: this index is scoped to ordinary consumer-visible pages, customer APIs, and customer-adjacent background convergence. It does not replace merchant, rider, operator, platform, provider, onboarding, or payment-domain source slices.

## How To Use This Index

Use this file as the handoff queue after reading the relevant slice:

- `Boundary exit`: a customer page links to role registration or another role's workbench; it is deliberately excluded from customer closure.
- `Duplicate wrapper`: shared Mini Program API code is copied into page-local `_main_shared` folders; verify behavior once, then keep callers aligned.
- `Recovery dependency`: the customer page shows a pending state whose final truth comes from callbacks, workers, or schedulers.
- `No page caller`: backend/API path exists and is reviewed as a boundary, but no current customer Mini Program entry invokes it.
- `Operational closure`: the customer can see/request state but the next write is merchant, rider, operator, platform, or provider-owned.

For implementation, start from the owning slice and source evidence instead of changing code from this index alone.

## Scope And Boundary Exits

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-SCOPE-001 | Boundary exit | Takeout home exposes merchant/operator registration entry actions, but user confirmed customer-side closure means ordinary consumers only. | Registration flows are reachable but not customer business-flow closure. | Keep registration paths in onboarding/role slices; do not mix them into customer CodeGraph verdicts. | `customer-discovery-and-merchant-browse.slice.md` |
| CU-SCOPE-002 | Boundary exit | User center still includes role/workbench affordances such as bind merchant and cross-role workbench state. | These are account/role exits, not consumer lifecycle state. | Treat as explicit boundary exits unless a future task requests account-role switch analysis. | `customer-profile-address-wallet-membership-reviews.slice.md` |
| CU-SCOPE-003 | Operational closure | Customer service center can submit and continue/withdraw claims, but recovery deductions, disputes, merchant/rider repayment, and operator visibility are owned elsewhere. | Customer closure stops at user action and read visibility; downstream enforcement is cross-role. | Keep downstream recovery/dispute/payment in merchant/rider/operator/payment slices. | `customer-order-tracking-refund-after-sales.slice.md` |
| CU-SCOPE-004 | Runtime support | Mini Program login/session/refresh/Web-login confirmation is required for every customer flow but is not one business vertical. | Without a runtime slice, the six business slices could look exhaustive while omitting auth/session support. | Keep runtime identity support in `customer-runtime-auth-session-support.slice.md` and reference it from business-flow reviews. | `customer-runtime-auth-session-support.slice.md` |

## Duplicate Wrapper And API Drift Risks

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-DUP-001 | Duplicate wrapper | `order.ts`, `payment.ts`, and `reservation.ts` appear in several `_main_shared/api` folders under takeout, dine-in, orders, reservation, payment, and user center pages. | A fix in one page-local copy can leave another customer entry stale. | When changing customer order/payment/reservation contracts, update all active copies or consolidate to a shared customer API owner. | takeout/cart/order, dine-in, reservation, orders, payment slices |
| CU-DUP-002 | Duplicate wrapper | Payment workflow service copies all follow the rule that `wx.requestPayment` is not terminal truth, but each page-local copy can drift. | Pending-confirmation and refresh behavior can differ between checkout, reservation, orders, and payment result. | Keep `payment/result` and all payment workflow copies aligned around backend query/ledger truth. | `customer-takeout-cart-checkout-payment.slice.md`, `customer-reservation-lifecycle.slice.md` |
| CU-DUP-003 | Contract drift risk | Cart checkout passes a local event-channel snapshot to order confirm, then order confirm rehydrates when snapshot is absent or stale. | Weak-network or re-entry bugs can appear if frontend trusts the snapshot over backend cart/order calculation. | Preserve backend rehydration and cart calculation before order submit. | `customer-takeout-cart-checkout-payment.slice.md` |
| CU-DUP-004 | Wrapper noise / boundary risk | Customer-reachable `_main_shared` directories can contain merchant/rider/operator/onboarding wrappers that are not page-called customer actions. | Static path scans over-report customer coverage and can hide real business-chain gaps. | When auditing, trace from `app.json` pages and actual imports/callers before treating a wrapper as customer-visible. | `customer-runtime-auth-session-support.slice.md`, this re-review |

## Payment, Refund, And Async Recovery

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-RECOVERY-001 | Recovery dependency | Takeout, dine-in, reservation deposit/addon, membership recharge, and claim recovery payments all converge through payment orders, callbacks, facts, timeout workers, and recovery schedulers. | Customer pages can show pending confirmation until backend/provider truth lands. | Treat page payment result as provisional; validate callback/fact/recovery when changing any payment-bearing customer flow. | all payment-bearing slices |
| CU-RECOVERY-002 | Recovery dependency | Refund detail and payment detail pages poll/read local refund/payment records; provider callback or recovery scheduler owns terminal truth. | Customer can see pending/processing state without immediate provider terminal state. | Do not claim refund completion from client actions; use backend refund status and ledger. | `customer-order-tracking-refund-after-sales.slice.md` |
| CU-RECOVERY-003 | No page caller / API boundary | `/v1/refunds` create is callable by wrappers and tests, but ordinary customer pages mostly reach refund through order cancel/reservation modification/system workflows rather than a generic manual refund form. | Generic refund creation should not be treated as a visible customer button unless a page actually wires it. | Keep as payment/refund API boundary; add UI-specific slice if a customer refund form lands. | `customer-order-tracking-refund-after-sales.slice.md` |
| CU-RECOVERY-004 | Runtime recovery | Cold start can reuse token, refresh token, or fall back to `wx.login`; every request can retry after 401/token-expired through refresh. | Weak-network or expired-token bugs can block all customer flows even when the business slice itself is correct. | Validate login reuse/refresh/full-login and request retry when changing auth, request runtime, or high-value customer entry pages. | `customer-runtime-auth-session-support.slice.md` |

## Discovery And Promotion Variants

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-DISC-001 | Boundary exit | Wanted-merchant voting is customer-submitted demand signal, not merchant onboarding approval. | Customer can vote/request, but invitation, onboarding, and approval are platform/merchant-owned. | Keep only vote/list closure in customer slice. | `customer-discovery-and-merchant-browse.slice.md` |
| CU-DISC-002 | Recovery dependency | Coupon claim appears in merchant detail and coupons/user-center reads; availability depends on voucher status, remaining quantity, and previous user claim. | UI must re-read voucher/user voucher state after claim or entry refresh. | Keep voucher claim and list pages aligned with `/v1/vouchers/**` backend truth. | `customer-discovery-and-merchant-browse.slice.md`, `customer-profile-address-wallet-membership-reviews.slice.md` |
| CU-DISC-003 | Contract drift risk | Merchant/dish/combo public detail, search, and homepage category feeds all depend on online/open/status filters in backend SQL. | Showing unavailable items as orderable is a customer-facing business bug. | Preserve backend orderability/status checks when refactoring discovery adapters. | `customer-discovery-and-merchant-browse.slice.md` |

## Dine-In And Reservation Closure Boundaries

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-DINE-001 | Operational closure | Dine-in scan entry can open/resume/transfer a customer session, but table QR generation and table configuration are merchant-owned. | Customer slice covers scan/open/transfer, not QR/table lifecycle. | Keep QR and table admin in merchant/table slices. | `customer-dine-in-session-menu-checkout.slice.md` |
| CU-DINE-002 | Recovery dependency | Dine-in checkout calls order/payment and then `checkoutDiningSession` after paid order confirmation. | Session closure may lag behind payment result if payment confirmation or checkout call fails. | Retain session rehydration/retry behavior and consider focused E2E around paid order -> session checkout. | `customer-dine-in-session-menu-checkout.slice.md` |
| CU-RES-001 | Operational closure | Reservation detail/user center can check in/start cooking handoff, but merchant confirm/no-show/complete remains staff-owned. | Customer sees and initiates allowed customer actions; merchant state machine is a boundary. | Keep merchant reservation workbench in merchant slices. | `customer-reservation-lifecycle.slice.md` |
| CU-RES-002 | Recovery dependency | Reservation modification can be applied, require addon payment, or initiate refund. | Customer page must represent asynchronous payment/refund outcomes rather than treating submit as final. | Validate modification branches against `reservation_adjustments`, payment orders, and refund recovery. | `customer-reservation-lifecycle.slice.md` |

## Profile, Wallet, And Notification Variants

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-PROFILE-001 | PII / media boundary | Avatar update uses media upload/complete/private access plus `/v1/users/me`; user center caches local avatar when backend returns empty. | Profile display can differ from persisted backend state during media or re-entry failures. | Keep media visibility/private access rules and profile re-read behavior explicit. | `customer-profile-address-wallet-membership-reviews.slice.md` |
| CU-PROFILE-002 | PII boundary | Address edit can use WeChat address/location fields, reverse geocode, and region resolution before `/v1/addresses` write. | Incorrect local region resolution can affect takeout checkout delivery eligibility. | Keep address writes and checkout delivery-fee calculation tied to backend region/address truth. | `customer-profile-address-wallet-membership-reviews.slice.md`, `customer-takeout-cart-checkout-payment.slice.md` |
| CU-NOTIF-001 | Notification category boundary | Customer notification page filters and icon mapping depend on backend notification categories/audience. | New backend categories can be hidden or mislabeled if frontend tabs/copy are not updated. | Update notification producer, backend filter semantics, and customer notification UI together. | `customer-profile-address-wallet-membership-reviews.slice.md` |

## Tracking And No-Page API Boundaries

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| CU-TRACK-001 | Operational closure | Customer order tracking reads `/v1/delivery/order/:id`, rider latest location, delivery track, and display-only `/v1/location/direction/bicycling`, but rider status writes remain rider-owned. | Customer can observe delivery progress, map route display, and confirm receipt through order flow; they do not own pickup/delivery transitions. | Keep customer tracking and map-display reads in after-sales slice and rider mutation closure in rider slices. | `customer-order-tracking-refund-after-sales.slice.md` |
| CU-NOPAGE-001 | No page caller / API boundary | `/v1/history/browse` and `getBrowseHistory` exist, but no ordinary customer page currently invokes browse-history UI. | Backend/API existence should not be counted as a missing customer page flow. | Add a page-specific customer slice if browse history becomes visible in user center or discovery. | `customer-related-completeness-audit.md` |
| CU-NOPAGE-002 | Metadata boundary | `/v1/role-access` and `/v1/app/version/latest` exist as public metadata routes; current ordinary customer pages do not call them. | They are runtime/platform metadata, not active customer business chains. | Document only if a customer page starts rendering or gating behavior from them. | `customer-runtime-auth-session-support.slice.md` |

## Resolved Or Explicit Non-Issues

- No ordinary customer entrypoint is intentionally modeled as a merchant, rider, operator, platform, or registration flow in this pass.
- `pages/notification/index` is covered as a customer notification center, while operator notification pages remain in operator slices.
- Payment provider callbacks are referenced as convergence boundaries only; their provider-contract truth remains in payment/domain artifacts.
- Generic media/OCR/admin routes under the authenticated group are not customer business-flow closure unless invoked by the customer pages listed in the completeness audit.
- Runtime auth/session support is covered separately from business slices; this prevents every business slice from duplicating login and token-refresh edges.
