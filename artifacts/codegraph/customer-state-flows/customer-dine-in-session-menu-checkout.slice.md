# Customer Dine-In Session Menu Checkout Slice

Status: customer-state flow slice created 2026-06-14
Risk class: G3 boundary - table scan trust, dine-in session ownership, billing group/order/payment/session checkout state
Scope: dining entry -> scan-entry -> dine-in menu -> dine-in checkout -> dining-session/billing-group/cart/order/payment backend routes -> SQL session/billing/cart/order/payment tables -> paid-session checkout convergence

## Variant Coverage

This slice covers:

- `pages/dining/index` redirect into scan-entry.
- Scan table entry via `scene`, `q`, `table_id`, `merchant_id`, and `table_no`; table scan lookup; dine-in precheck; open/resume/transfer session actions; optional table code; transfer dialog; and menu navigation.
- Dine-in menu initialization by active session or reservation handoff, session persistence, menu load, cart load, add/update item, and checkout navigation.
- Dine-in checkout initialization by session/billing group or reservation fallback, cart calculation, payment method selection, order create, payment workflow, payment result navigation, and post-paid `checkoutDiningSession`.
- Billing group create/join/list/orders API boundary.

This slice does not fully cover:

- Merchant table/QR generation and table code administration; merchant-owned.
- Reservation creation and reservation dish modification before dine-in; covered by `customer-reservation-lifecycle.slice.md`.
- Merchant kitchen/order preparation after paid dine-in order; merchant-owned.
- Provider payment internals; referenced as payment-domain boundary.

## Product Invariant

Dine-in customer flow is a table/session state machine, not just a menu page:

- A customer may only open/resume/transfer a dine-in session through backend scan/precheck/session rules.
- Local stored session context is a convenience; menu/checkout must rehydrate session, merchant, table, billing group, and cart from backend.
- Billing group membership and active order linkage are durable backend state.
- Payment completion is not final client truth. After backend payment is confirmed, session checkout must converge through `checkoutDiningSession`.
- Reservation handoff can enter dine-in menu, but reservation lifecycle state remains owned by reservation slice.

## Primary Forward Chain

1. Customer Mini Program declares dining main page and the dine-in subpackage.
   Evidence: `weapp/miniprogram/app.json:6`, `weapp/miniprogram/app.json:117`, `weapp/miniprogram/app.json:120`, `weapp/miniprogram/app.json:121`, `weapp/miniprogram/app.json:122`.

2. Dining page redirects route params into scan-entry so customer-facing dining always enters the session precheck/open flow.
   Evidence: `weapp/miniprogram/pages/dining/index.ts:1`, `weapp/miniprogram/pages/dining/index.ts:10`.

3. Scan-entry resolves `scene`, `q`, direct table params, loads dining session entry, renders open/resume/transfer/blocked action, and calls open or transfer session before navigating to menu.
   Evidence: `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:73`, `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:101`, `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:142`, `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:145`, `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:219`, `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:246`, `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts:254`.

4. Dine-in shared API calls dining-session entry/precheck/open/menu/transfer/checkout and order creation with `billing_group_id`.
   Evidence: `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:130`, `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:138`, `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:151`, `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:159`, `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:167`, `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:175`, `weapp/miniprogram/pages/dine-in/_main_shared/api/dining-session.ts:184`.

5. Menu page initializes by session id or reservation id, loads backend session/menu/cart state, stores session context, and navigates to checkout with session and billing group.
   Evidence: `weapp/miniprogram/pages/dine-in/menu/menu.ts:38`, `weapp/miniprogram/pages/dine-in/menu/menu.ts:83`, `weapp/miniprogram/pages/dine-in/menu/menu.ts:153`, `weapp/miniprogram/pages/dine-in/menu/menu.ts:173`, `weapp/miniprogram/pages/dine-in/menu/menu.ts:200`, `weapp/miniprogram/pages/dine-in/menu/menu.ts:426`, `weapp/miniprogram/pages/dine-in/_main_shared/services/dine-in-session.ts:124`, `weapp/miniprogram/pages/dine-in/_main_shared/services/dine-in-session.ts:139`.

6. Checkout page initializes from direct session/billing group or stored context, loads checkout session/cart, calculates backend cart totals, creates dine-in order, creates payment, and handles payment result.
   Evidence: `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:31`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:83`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:143`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:156`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:243`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:246`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:252`, `weapp/miniprogram/pages/dine-in/checkout/checkout.ts:289`.

7. After paid dine-in order, shared session service calls backend checkout session to close/update session state.
   Evidence: `weapp/miniprogram/pages/dine-in/_main_shared/services/dine-in-session.ts:105`, `weapp/miniprogram/pages/dine-in/_main_shared/services/dine-in-session.ts:117`, `locallife/api/dining_session.go:713`.

8. Backend scan, dining-session, billing-group, order, and payment routes are registered under authenticated customer routes.
   Evidence: `locallife/api/server.go:619`, `locallife/api/server.go:625`, `locallife/api/server.go:989`, `locallife/api/server.go:992`, `locallife/api/server.go:995`, `locallife/api/server.go:997`, `locallife/api/server.go:1000`, `locallife/api/server.go:1003`, `locallife/api/server.go:1005`, `locallife/api/server.go:1009`, `locallife/api/server.go:1105`.

9. Backend handlers and SQL own session creation/transfer/menu, billing group state, cart/order/payment state.
   Evidence: `locallife/api/scan.go:131`, `locallife/api/dining_session.go:265`, `locallife/api/dining_session.go:350`, `locallife/api/dining_session.go:407`, `locallife/api/dining_session.go:470`, `locallife/api/dining_session.go:609`, `locallife/api/dining_session.go:713`, `locallife/api/billing_group.go:195`, `locallife/api/billing_group.go:266`, `locallife/db/query/dining_session.sql:1`, `locallife/db/query/dining_session.sql:25`, `locallife/db/query/billing_group.sql:3`, `locallife/db/query/billing_group.sql:64`.

## SQL And Durable State Boundaries

- `tables`: scan target, merchant/table linkage, table status, table code/QR boundary.
- `dining_sessions`: active/closed dine-in session, merchant/table/user/reservation linkage, active order pointer.
- `billing_groups`, `billing_group_members`, and `billing_group_orders`: group ownership, participants, order association, and amount read model.
- `carts` and `cart_items`: dine-in cart item state keyed to merchant/order type/session context.
- `orders`: dine-in order submitted with `billing_group_id` and order type.
- `payment_orders`: payment state for dine-in order.
- `table_reservations` and reservation items: reservation handoff boundary into dine-in menu.

## Trust, Authorization, And Tenant Checks

- Scan/table routes are authenticated; raw QR/table params are not sufficient to mutate session state.
- `getDiningSessionEntry` and `openDiningSession` must validate table/merchant/session status and table code requirements.
- Transfer session must verify the active session belongs to the current user and same merchant constraints.
- Billing group operations must validate session/group membership.
- Order create and payment create must use authenticated user and backend session/order state.

## Idempotency And Duplicate-Submit Checks

- Scan-entry uses `submitting` to guard open/transfer double taps.
- Checkout uses `submitting` to guard order/payment double submit.
- Backend session open/resume must converge existing active sessions rather than creating duplicate active sessions per user/table/merchant where prohibited.
- Billing group create/join should converge default group membership and reject invalid duplicate joins through backend constraints.
- Payment workflow and provider facts handle duplicate callbacks through payment-domain dedupe.

## Recovery And Async Convergence Paths

- Scan-entry can retry load entry and re-run precheck.
- Menu page can reload backend session/menu/cart on entry and after cart item mutations.
- Checkout page can rehydrate direct session context or stored session context.
- Payment pending-confirmation converges through payment result refresh/callback/recovery.
- If post-paid session checkout fails, menu/session rehydration and `checkoutPaidDineInSession` should be used as the recovery path; this is a high-value E2E gap.

## Frontend Draft And Backend Rehydration

- QR params, table code input, transfer code input, and stored session context are frontend drafts.
- Session/menu/billing group/cart state is rehydrated from backend before checkout.
- Reservation handoff params are route hints; menu initialization must load reservation/session state from backend.
- Payment result URL params are display hints; backend payment query and session checkout are durable truth.

## Test Coverage Signals

Observed tests:

- `locallife/api/scan_test.go` covers scan table handler branches and media/WeChat client setup signals.
- `locallife/api/dining_session.go` and `billing_group.go` have distinct handlers, with SQL sources for active session and billing group state.
- Payment/order tests cover shared order/payment/refund behaviors used by dine-in.

Missing high-value tests:

- End-to-end QR scan -> open session -> add cart -> dine-in checkout -> payment callback -> checkout session.
- Transfer session with table code success/failure and same-merchant constraints.
- Duplicate open/resume behavior under repeated scan-entry taps.
- Billing group join/list/orders customer contract test.

## Gaps And Refactor Notes

- Dine-in and payment copies of shared order/payment/session APIs can drift. Keep wrappers synchronized before changing customer payment semantics.
- `checkoutDiningSession` after paid order is a critical convergence step and deserves focused regression coverage.
- Table QR/code trust belongs to merchant/table source flows; customer docs should not imply customers can administer tables.

## Branch Exhaustion

- Entry branches checked: dining redirect, scan by scene/q/table id, open session, resume active session, transfer session, blocked precheck, reservation handoff, direct session menu, direct checkout, stored session checkout.
- Request branches checked: `/v1/scan/table`, `/v1/dining-sessions/entry`, `/precheck`, `/open`, `/:id/menu`, `/:id/transfer-table`, `/:id/checkout`, `/v1/billing-groups`, `/:id/join`, `/:id/orders`, `/v1/cart`, `/v1/orders`, `/v1/payments`.
- Backend state branches checked: table missing, code required/missing, active session exists, transfer candidate exists, closed session, empty cart, payment pending/paid/failed, session checkout after payment.
- Async branches checked: payment callback/fact/recovery boundary and post-paid session checkout retry boundary.
- Dead/orphan branches checked: no customer dine-in entry omitted; merchant table/QR management excluded by design.
