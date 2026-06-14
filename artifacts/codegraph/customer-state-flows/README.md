# Customer State Flows Codegraph

Status: customer completeness re-review refreshed 2026-06-14
Risk class: G2/G3 mix - customer order/payment/refund/reservation/dine-in state, address/profile PII, claim/food-safety escalation, notification and recovery paths
Scope: WeChat Mini Program ordinary consumer pages -> customer-facing backend routes -> logic/transactions -> SQL tables -> payment/refund callbacks and recovery boundaries -> dead/orphan customer-facing paths
Boundary note: this directory judges ordinary customer/consumer closure only: what a customer can browse, submit, pay, cancel, track, review, claim, recover, and authenticate to use those flows. Rider, merchant, operator, platform, registration, provider, and staff/admin source flows are referenced at their boundary but remain owned by their role/domain slices.

Before creating or refreshing a customer slice, use the workflow in
`artifacts/codegraph/README.md`: CodeGraph may be used for discovery and line
anchor drift checks, but the slice and edge artifacts are the durable
LocalLife-aware source of truth after review.

## Slice Map

- `customer-discovery-and-merchant-browse.slice.md`: takeout home/search/category, merchant/dish/combo/room discovery, merchant detail, coupon claim, search history/popular/suggestions, wanted-merchant votes, and role-registration exit boundary.
- `customer-takeout-cart-checkout-payment.slice.md`: multi-merchant carts, cart item mutation, checkout snapshot, address/payment method selection, order creation, payment order creation, payment result polling, timeout/callback/recovery boundaries.
- `customer-dine-in-session-menu-checkout.slice.md`: scan table entry, dine-in precheck/open/resume/transfer, menu/cart reuse, billing group boundary, dine-in order creation/payment, and session checkout after paid order.
- `customer-reservation-lifecycle.slice.md`: reservation discovery, room availability, reservation create/confirm/detail/list/modify, deposit/addon payment, cancel/refund, check-in/start-cooking customer handoff, and dine-in handoff.
- `customer-order-tracking-refund-after-sales.slice.md`: order list/detail/tracking, delivery and route display, cancel/refund progress, retry pay, urge, reorder, confirm receipt, reviews, service-center claims, food-safety report, claim continue/withdraw/payout confirmation.
- `customer-profile-address-wallet-membership-reviews.slice.md`: user center profile/avatar/nickname, address book, wallet/ledger, memberships/recharge, coupons, favorites, agreements, notifications, and review list/create/update/delete.
- `customer-runtime-auth-session-support.slice.md`: Mini Program silent login, token refresh, request 401 retry, Web login QR confirmation, authenticated frontend error logging, and cross-role account exits.
- `flow-variant-index.md`: compact branch, drift, no-entry, duplicate wrapper, recovery, and customer-closure checklist across all customer slices.
- `customer-related-completeness-audit.md`: explicit verdict for consumer-side closure versus all customer-related cross-role/background touchpoints, including Mini Program entrypoints in `app.json`.

Each `*.edges.json` uses the same compact edge schema as the existing operator slices: only core page/API/logic/transaction/table/provider edges are modeled, while branch detail stays in the Markdown slices.

## Mini Program Entrypoints

The ordinary customer-facing Mini Program entries are declared in
`weapp/miniprogram/app.json:3` through `weapp/miniprogram/app.json:8` and the
customer subpackages in `weapp/miniprogram/app.json:101` through
`weapp/miniprogram/app.json:290`:

- Main pages: `pages/takeout/index`, `pages/reservation/index`, `pages/user_center/index`, `pages/dining/index`, `pages/notification/index`.
- Excluded from customer closure despite being reachable from a customer page: `pages/user/bind-merchant/index`, `pages/register/**`, rider/operator/platform/merchant packages, and merchant/operator/rider/platform workbench entrypoints.
- Orders: `pages/orders/detail/index`, `pages/orders/list/index`, `pages/orders/tracking/index`.
- Payment: `pages/payment/result/index`.
- Dine-in: `pages/dine-in/scan-entry/scan-entry`, `pages/dine-in/menu/menu`, `pages/dine-in/checkout/checkout`.
- Takeout: cart, dish detail, combo detail, order confirm, restaurant detail, search, wanted merchants, merchant info, and category.
- Reservation: create, confirm, detail, list, modify, and room detail.
- User center: addresses, coupons, favorites, membership, reviews, wallet, payment detail, refund detail, reservations, agreements, about us, and service center.

## Backend Route Surface

Customer-facing route groups are registered in `locallife/api/server.go`:

- Auth/login/runtime support: public `/v1/auth/wechat-login`, `/v1/auth/refresh`, Web login session routes, authenticated `/v1/auth/web-login/confirm`, `/v1/users/me`, and `/v1/logs/error`: `locallife/api/server.go:527` through `locallife/api/server.go:572` and `locallife/api/server.go:650` through `locallife/api/server.go:654`.
- Search, agreements, merchant public browse, wanted merchants, scan table, users/me, media, addresses, and location or map route-planning support: `locallife/api/server.go:583` through `locallife/api/server.go:688`.
- Customer rooms/reservations, dining sessions, billing groups, orders, payments, refunds, and delivery tracking read routes: `locallife/api/server.go:943` through `locallife/api/server.go:1128` and `locallife/api/server.go:1206` through `locallife/api/server.go:1209`.
- Notifications: `locallife/api/server.go:1271` through `locallife/api/server.go:1280`.
- Claims, food-safety report, cart, favorites, browse history, memberships, reviews, and vouchers: `locallife/api/server.go:1515` through `locallife/api/server.go:1707`.

Provider callback and recovery boundaries that affect customer-visible state are registered separately under webhooks in `locallife/api/server.go:548` through `locallife/api/server.go:557` and worker/scheduler paths referenced by the individual slices.

## Validation

This directory is documentation/artifact-only. Validate edge JSON with:

```bash
jq empty artifacts/codegraph/customer-state-flows/*.edges.json
```

Cross-check the local CodeGraph index with:

```bash
/home/sam/.nvm/versions/node/v24.12.0/bin/codegraph status
```
