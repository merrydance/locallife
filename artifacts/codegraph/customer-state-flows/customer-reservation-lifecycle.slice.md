# Customer Reservation Lifecycle Slice

Status: customer-state flow slice created 2026-06-14
Risk class: G3 boundary - room availability, reservation status, deposit/addon payment, cancellation/refund, dine-in handoff
Scope: reservation home/create/confirm/detail/list/modify/room-detail/user-center reservations -> room/reservation/payment/order/dine-in APIs -> reservation/table/order/payment/refund SQL -> payment/refund/recovery boundaries

## Variant Coverage

This slice covers:

- Reservation home list/search/filter, room discovery, room detail, availability query, create page, confirm page, and reservation create.
- Reservation detail/list/user-center reservations, cancel, pay deposit, modify dishes, addon payment, refund branch, check-in/start-cooking customer actions, and dine-in menu handoff.
- Backend customer rooms, room availability, reservation create/list/get/cancel/add-dishes/modify-dishes/checkin/start-cooking routes.
- Reservation deposit/addon payment via generic payment route and payment result/workflow.
- SQL boundaries for tables, table reservations, reservation items/payments/adjustments, orders, payment orders, and refund orders.

This slice does not fully cover:

- Merchant reservation workbench, merchant confirm/no-show/complete/update, room/table management, and inventory administration.
- Platform/admin or operator region/rules that influence room availability.
- Provider callback correctness; referenced as payment-domain boundary.
- Dine-in session internals after reservation handoff; covered by `customer-dine-in-session-menu-checkout.slice.md`.

## Product Invariant

Reservation is a persisted status machine with time/room/payment constraints:

- Room availability and reservation status must be backend truth; client date/time/party-size selection is draft input.
- Deposit and addon payments are not terminal after `wx.requestPayment`; backend payment callback/fact/recovery must converge reservation/payment state.
- Cancellation and dish modification can initiate refund/payment branches; customer pages must render pending/asynchronous outcomes.
- Check-in/start-cooking and dine-in menu handoff are customer actions only when backend status allows them.
- Merchant confirmation/completion/no-show are not customer-owned.

## Primary Forward Chain

1. Customer Mini Program declares reservation main page, reservation subpackages, user-center reservation list, dine-in handoff, and payment result.
   Evidence: `weapp/miniprogram/app.json:4`, `weapp/miniprogram/app.json:180`, `weapp/miniprogram/app.json:186`, `weapp/miniprogram/app.json:192`, `weapp/miniprogram/app.json:198`, `weapp/miniprogram/app.json:204`, `weapp/miniprogram/app.json:210`, `weapp/miniprogram/app.json:266`, `weapp/miniprogram/app.json:110`, `weapp/miniprogram/app.json:117`.

2. Reservation home loads room/merchant items, supports refresh/pagination/filtering, and navigates to room detail or reservation create/confirm.
   Evidence: `weapp/miniprogram/pages/reservation/index.ts:38`, `weapp/miniprogram/pages/reservation/index.ts:102`, `weapp/miniprogram/pages/reservation/index.ts:144`, `weapp/miniprogram/pages/reservation/index.ts:287`, `weapp/miniprogram/pages/reservation/index.ts:326`, `weapp/miniprogram/pages/reservation/index.ts:338`.

3. Room detail loads room detail and availability, then builds confirm navigation with selected room/date/time/party data.
   Evidence: `weapp/miniprogram/pages/reservation/room-detail/index.ts:26`, `weapp/miniprogram/pages/reservation/room-detail/index.ts:42`, `weapp/miniprogram/pages/reservation/room-detail/index.ts:57`, `weapp/miniprogram/pages/reservation/room-detail/index.ts:85`, `weapp/miniprogram/pages/reservation/room-detail/index.ts:205`.

4. Reservation create and confirm pages create reservations, confirm availability, and enter payment workflow when required.
   Evidence: `weapp/miniprogram/pages/reservation/create/index.ts:10`, `weapp/miniprogram/pages/reservation/create/index.ts:65`, `weapp/miniprogram/pages/reservation/create/index.ts:155`, `weapp/miniprogram/pages/reservation/confirm/index.ts:19`, `weapp/miniprogram/pages/reservation/confirm/index.ts:50`, `weapp/miniprogram/pages/reservation/confirm/index.ts:130`, `weapp/miniprogram/pages/reservation/confirm/index.ts:263`, `weapp/miniprogram/pages/reservation/confirm/index.ts:273`.

5. Reservation detail/list/user-center reservations read reservation state, cancel, pay deposit, and hand off to dine-in menu.
   Evidence: `weapp/miniprogram/pages/reservation/list/index.ts:9`, `weapp/miniprogram/pages/reservation/list/index.ts:76`, `weapp/miniprogram/pages/reservation/detail/index.ts:18`, `weapp/miniprogram/pages/reservation/detail/index.ts:56`, `weapp/miniprogram/pages/reservation/detail/index.ts:135`, `weapp/miniprogram/pages/reservation/detail/index.ts:155`, `weapp/miniprogram/pages/reservation/detail/index.ts:192`, `weapp/miniprogram/pages/user_center/reservations/index.ts:36`, `weapp/miniprogram/pages/user_center/reservations/index.ts:77`, `weapp/miniprogram/pages/user_center/reservations/index.ts:154`, `weapp/miniprogram/pages/user_center/reservations/index.ts:173`, `weapp/miniprogram/pages/user_center/reservations/index.ts:214`.

6. Reservation modify page loads reservation/menu data, submits dish changes, and handles applied/payment-required/refund-initiated outcomes.
   Evidence: `weapp/miniprogram/pages/reservation/modify/index.ts:61`, `weapp/miniprogram/pages/reservation/modify/index.ts:84`, `weapp/miniprogram/pages/reservation/modify/index.ts:338`, `weapp/miniprogram/pages/reservation/modify/index.ts:339`, `weapp/miniprogram/pages/reservation/modify/index.ts:341`, `weapp/miniprogram/pages/reservation/modify/_main_shared/api/reservation.ts:590`.

7. Backend customer room and reservation routes are registered under authenticated routes and separated from merchant reservation operations.
   Evidence: `locallife/api/server.go:943`, `locallife/api/server.go:948`, `locallife/api/server.go:954`, `locallife/api/server.go:958`, `locallife/api/server.go:959`, `locallife/api/server.go:960`, `locallife/api/server.go:962`, `locallife/api/server.go:963`, `locallife/api/server.go:964`, `locallife/api/server.go:965`, `locallife/api/server.go:966`, `locallife/api/server.go:969`, `locallife/api/server.go:982`.

8. Backend handlers map customer room/reservation reads/writes to table/reservation/payment/refund durable state.
   Evidence: `locallife/api/table.go:621`, `locallife/api/table.go:682`, `locallife/api/table.go:1750`, `locallife/api/table.go:1883`, `locallife/api/table_reservation.go:331`, `locallife/api/table_reservation.go:488`, `locallife/api/table_reservation.go:608`, `locallife/api/table_reservation.go:1335`, `locallife/api/table_reservation.go:1486`, `locallife/api/table_reservation.go:1587`, `locallife/api/table_reservation.go:1822`, `locallife/api/table_reservation.go:1869`.

9. Reservation/payment/refund SQL and recovery paths own durable convergence for deposit/addon payment and refunds.
   Evidence: `locallife/db/query/table.sql:78`, `locallife/db/query/table.sql:176`, `locallife/db/query/reservation_item.sql:1`, `locallife/db/query/reservation_payment.sql:1`, `locallife/db/query/reservation_adjustment.sql:1`, `locallife/db/query/payment_order.sql:50`, `locallife/db/query/refund_order.sql:131`, `locallife/worker/task_reservation_timeout.go:118`, `locallife/logic/reservation_dishes.go:367`.

## SQL And Durable State Boundaries

- `tables`, table images, and table tags: room source and customer-visible room data.
- `table_reservations`: reservation status, date/time, payment mode/deposit/prepaid amount, deadlines, cancellation/check-in/cooking timestamps.
- `reservation_items`: pre-ordered dishes tied to reservation.
- `reservation_payments`: deposit/reservation payment linkage.
- `reservation_adjustments` and adjustment items/holds: dish-change diff, addon/refund state, and expiration.
- `orders`: reservation-related order and dine-in order handoff.
- `payment_orders`: deposit, reservation addon, and possible related order payment state.
- `refund_orders`: cancellation/modification refund state and provider convergence.

## Trust, Authorization, And Tenant Checks

- Customer reservation routes use authenticated user id and must only expose the user's reservations.
- Room availability must be computed server-side from room/table/reservation state, not client calendars.
- Reservation cancel/modify/check-in/start-cooking must check reservation ownership and status.
- Payment/refund routes must validate reservation/payment ownership and status.
- Merchant reservation operations are behind merchant staff middleware and are excluded from customer authority.

## Idempotency And Duplicate-Submit Checks

- Reservation create/confirm pages guard local submit/payment states.
- Payment workflow uses backend payment order status and pending-confirmation refresh.
- Reservation modification must converge through active adjustment/payment/refund records and expire unpaid addon adjustments.
- Refund request idempotency and provider callback dedupe are payment/refund-domain boundaries.
- Timeout workers close stale reservation payment/order payment state.

## Recovery And Async Convergence Paths

- Reservation home/list/detail/user-center pages can refresh and reload persisted reservation state.
- Deposit/addon payments converge through payment result refresh, callback/fact application, and timeout/recovery workers.
- Dish modification refund branch can remain pending until refund callback/recovery updates refund order.
- Reservation detail/user-center can hand off paid/confirmed/check-in-ready reservations to dine-in menu; dine-in session open/menu state is rehydrated there.
- Merchant confirm/no-show/complete can change customer-visible state asynchronously and must be re-read from backend.

## Frontend Draft And Backend Rehydration

- Selected date/time/party size/contact/notes and draft dish changes are frontend drafts.
- Availability, price/deposit/addon/refund outcome, and allowed actions are backend state.
- Payment result params are display hints; payment order and reservation reads are authoritative.
- Dine-in handoff params are route hints; dine-in menu/session slice rehydrates session/menu state.

## Test Coverage Signals

Observed tests:

- `locallife/api/table_reservation_test.go` covers many reservation list, merchant operation, validation, status, and conflict branches.
- `locallife/logic/reservation_dishes_test.go` covers reservation dish adjustment/payment/refund logic.
- `locallife/worker/task_reservation_timeout.go` owns reservation payment timeout processing.
- Payment/refund tests cover generic payment/refund branches used by reservation deposit/addon flows.

Missing high-value tests:

- Customer Mini Program create/confirm -> deposit payment -> pending result -> callback -> reservation paid refresh.
- Reservation modify branches for applied, payment-required, refund-initiated from customer UI.
- Customer cancel with refund progress into refund detail.
- Customer handoff from reservation detail/user-center reservations into dine-in menu/session open.

## Gaps And Refactor Notes

- Reservation API wrappers are duplicated across reservation, orders, dine-in, and user-center page groups; keep contract changes synchronized.
- Merchant reservation operations are close in route namespace but must not be treated as customer authority.
- Reservation modification payment/refund outcomes are complex and deserve a separate high-risk implementation plan before code changes.

## Branch Exhaustion

- Entry branches checked: reservation home, room detail, create, confirm, detail, list, modify, user-center reservations, payment result, dine-in menu handoff.
- Request branches checked: `/v1/merchants/:id/rooms`, `/rooms/:id`, `/rooms/:id/availability`, `/v1/reservations`, `/me`, `/:id`, `/:id/cancel`, `/:id/add-dishes`, `/:id/modify-dishes`, `/:id/checkin`, `/:id/start-cooking`, `/v1/payments`, `/v1/refunds`.
- Backend state branches checked: room unavailable, reservation pending/paid/confirmed/checked-in/cooking/completed/cancelled/expired, payment required, refund initiated, no-payment modification, unauthorized reservation.
- Async branches checked: payment timeout, addon adjustment expiration, refund recovery, provider callback/fact boundary.
- Dead/orphan branches checked: no customer reservation page omitted; merchant reservation routes excluded by middleware boundary.
