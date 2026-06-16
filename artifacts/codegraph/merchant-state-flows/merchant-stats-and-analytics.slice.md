# Merchant Stats And Analytics Slice

Status: merchant-state flow slice created 2026-06-14
Risk class: G2 - merchant-facing read path exposes revenue, order, reservation, dish-performance, and customer contact/profile signals but does not mutate production state
Scope: Mini Program merchant stats pages -> merchant stats/order/reservation read APIs -> aggregate SQL reads over orders, order items, users, dishes, print logs, and reservations

## Variant Coverage

This slice covers:

- Merchant Mini Program stats overview page under `weapp/miniprogram/pages/merchant/stats/index`.
- Merchant customer analytics list and detail pages under `weapp/miniprogram/pages/merchant/stats/customers`.
- Merchant dashboard overview read helper that reuses the merchant stats overview API.
- Backend `/v1/merchant/stats/**` routes for overview, daily trend, top dishes, customers, customer detail, hourly distribution, order sources, repurchase, and dish categories.
- Stats-page read dependencies on `/v1/merchant/orders/stats` and `/v1/reservations/merchant/stats`.
- Aggregate SQL read boundaries for completed/delivered orders, order items, users, dishes/categories, print anomalies, and table reservations.

This slice does not cover:

- Order accept/reject/ready/complete, refund, print retry, websocket, and kitchen state mutations, which stay in `merchant-order-operations`.
- Reservation/table lifecycle writes and dining-session state, which stay in `merchant-reservation-and-table`.
- Dish/combo/catalog write semantics, which stay in `merchant-dish-status-and-inventory` and `merchant-combo-and-catalog`.
- Finance settlement/withdrawal truth, which stays in `merchant-finance-withdrawal`.
- Customer-facing public merchant/menu stats helpers that share `merchant_stats.sql` for storefront reads; those remain in the catalog and marketing slices.

## Product Invariant

Merchant analytics must be a derived read model over backend truth, not a second source of truth:

- The client must not provide a merchant id for `/v1/merchant/stats/**`; the backend derives the merchant from the authenticated user and merchant staff context.
- Customer list/detail analytics expose phone and avatar-derived URLs, so they must stay behind merchant authorization and SQL merchant/user filtering.
- Date-bounded BI reads must validate date ranges before SQL execution. Merchant stats handlers use the shared 365-day parser, while the order-stats dependency has its own 90-day limit.
- Stats pages may partially degrade when one section fails, but they must not silently replace a trusted prior section with empty data during refresh.
- All statistics are read-only. No worker, scheduler, transaction, outbox, or websocket path is owned by this slice.

## Primary Forward Chain

1. Merchant subpackage registers the stats overview page, customer list page, and customer detail page.
   Evidence: `weapp/miniprogram/app.json:309`, `weapp/miniprogram/app.json:310`, `weapp/miniprogram/app.json:311`.

2. Stats overview page builds a 7-day or 30-day range, then loads overview, order status counts, top dishes, daily trend, hourly distribution, order sources, repurchase, category stats, and reservation counts in one settled batch.
   Evidence: `weapp/miniprogram/pages/merchant/stats/index.ts:152`, `weapp/miniprogram/pages/merchant/stats/index.ts:372`, `weapp/miniprogram/pages/merchant/stats/index.ts:403`, `weapp/miniprogram/pages/merchant/stats/index.ts:414`.

3. Stats overview uses partial-result handling: successful sections update, failed sections expose section-level errors, and silent refresh can preserve previous trusted values.
   Evidence: `weapp/miniprogram/pages/merchant/stats/index.ts:416`, `weapp/miniprogram/pages/merchant/stats/index.ts:425`, `weapp/miniprogram/pages/merchant/stats/index.ts:439`, `weapp/miniprogram/pages/merchant/stats/index.ts:459`, `weapp/miniprogram/pages/merchant/stats/index.ts:544`.

4. Customer analytics page loads sorted, paginated customer rows and keeps prior data on silent refresh failure.
   Evidence: `weapp/miniprogram/pages/merchant/stats/customers/index.ts:109`, `weapp/miniprogram/pages/merchant/stats/customers/index.ts:131`, `weapp/miniprogram/pages/merchant/stats/customers/index.ts:138`, `weapp/miniprogram/pages/merchant/stats/customers/index.ts:158`.

5. Customer detail page requires a concrete `userId`, loads customer aggregate truth and favorite dishes, preserves previous detail on refresh failure, and exposes a phone-call action only when the returned phone exists.
   Evidence: `weapp/miniprogram/pages/merchant/stats/customers/detail/index.ts:75`, `weapp/miniprogram/pages/merchant/stats/customers/detail/index.ts:104`, `weapp/miniprogram/pages/merchant/stats/customers/detail/index.ts:118`, `weapp/miniprogram/pages/merchant/stats/customers/detail/index.ts:137`, `weapp/miniprogram/pages/merchant/stats/customers/detail/index.ts:148`.

6. Mini Program `MerchantStatsService` maps the stats pages to `/v1/merchant/stats/overview`, `/dishes/top`, `/daily`, `/customers`, `/customers/:user_id`, `/hourly`, `/sources`, `/repurchase`, and `/categories`.
   Evidence: `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:87`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:90`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:98`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:106`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:118`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:126`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:133`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:141`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:149`, `weapp/miniprogram/pages/merchant/_api/merchant-stats.ts:157`.

7. Backend registers the merchant stats route group under authenticated routes and protects it with `MerchantStaffMiddleware("owner", "manager")`.
   Evidence: `locallife/api/server.go:1290`, `locallife/api/server.go:1291`, `locallife/api/server.go:1292`, `locallife/api/server.go:1294`, `locallife/api/server.go:1303`.

8. Each merchant stats handler resolves the merchant from auth context before reading SQL. Customer detail additionally binds the user id from the URI and reads by `(merchant_id, user_id)`.
   Evidence: `locallife/api/merchant_stats.go:51`, `locallife/api/merchant_stats.go:71`, `locallife/api/merchant_stats.go:139`, `locallife/api/merchant_stats.go:159`, `locallife/api/merchant_stats.go:336`, `locallife/api/merchant_stats.go:360`, `locallife/api/merchant_stats.go:467`, `locallife/api/merchant_stats.go:478`, `locallife/api/merchant_stats.go:489`.

9. Overview, daily, top-dish, hourly, source, repurchase, and category reads are aggregate SQL over completed/user-delivered order truth and related item/category tables.
   Evidence: `locallife/db/query/merchant_stats.sql:3`, `locallife/db/query/merchant_stats.sql:20`, `locallife/db/query/merchant_stats.sql:38`, `locallife/db/query/merchant_stats.sql:135`, `locallife/db/query/merchant_stats.sql:150`, `locallife/db/query/merchant_stats.sql:165`, `locallife/db/query/merchant_stats.sql:196`.

10. Customer analytics reads join users only after constraining by merchant orders, and customer detail/favorite dishes also constrain by merchant id and user id.
    Evidence: `locallife/db/query/merchant_stats.sql:57`, `locallife/db/query/merchant_stats.sql:76`, `locallife/db/query/merchant_stats.sql:87`, `locallife/db/query/merchant_stats.sql:94`, `locallife/db/query/merchant_stats.sql:113`, `locallife/db/query/merchant_stats.sql:118`, `locallife/db/query/merchant_stats.sql:128`.

11. Stats overview also depends on the merchant order-stats and reservation-stats read endpoints for operational counts. Those APIs remain owned by their domain slices but are included here as read dependencies.
    Evidence: `weapp/miniprogram/pages/merchant/_api/order-management.ts:371`, `weapp/miniprogram/pages/merchant/_main_shared/api/reservation.ts:652`, `locallife/api/server.go:1041`, `locallife/api/server.go:976`, `locallife/api/order.go:2546`, `locallife/api/table_reservation.go:1430`.

12. Print anomaly count is folded into merchant overview from the print-log anomaly aggregate.
    Evidence: `locallife/api/merchant_stats.go:180`, `locallife/db/query/print_log.sql:127`.

## Reverse-Reference Findings

- Fixed by this artifact: merchant BI/stats pages and `/v1/merchant/stats/**` were active code but had no merchant-state-flow slice or direct `edges.json` page nodes.
- `merchant_stats.sql` is not exclusively merchant-BI. Lower public storefront/menu queries in the same SQL file are already referenced by combo/catalog and marketing slices. This slice owns the BI/customer-analytics subset through `GetMerchantDailyStats` to `GetDishCategoryStats`.
- The stats overview page is an aggregator over order, reservation, print, dish, and customer reads. It does not own those domains' write semantics or recovery state machines.
- Customer analytics is the most sensitive branch in this slice because it exposes phone and avatar/profile-derived fields. Its safety depends on owner/manager routing and backend merchant/user SQL predicates, not on client-side page location.

## SQL And Durable State Boundaries

- `orders`: source for sales totals, commission, order counts, order types, repurchase, customer cohorts, order status counts, and print anomaly merchant ownership.
- `order_items`: source for top dishes and customer favorite dishes.
- `users`: source for customer names, phone, and avatar media references in customer analytics.
- `dishes`, `dish_categories`, and `merchant_dish_categories`: source for dish/category labels and category sales grouping.
- `print_logs`: source for latest non-success print anomaly count included in overview.
- `table_reservations`: source for reservation status counts shown on stats overview.
- This slice writes no table and owns no generated state. Every number is a read-time aggregate over current durable state.

## Trust, Authorization, And Tenant Checks

- `/v1/merchant/stats/**` is authenticated and protected by `MerchantStaffMiddleware("owner", "manager")`.
- Handlers derive the merchant from the authenticated user via `resolveMerchantForUser`, then pass `merchant.ID` into SQL.
- Customer list/detail cannot be scoped by a client-provided merchant id; detail requires the requested `user_id` to have matching completed/delivered orders for that merchant.
- Order stats and reservation stats are separate route groups but still derive merchant context server-side before reading.
- Frontend route presence is not an authorization boundary; the backend route groups and SQL predicates are the reviewed boundary.

## Idempotency And Duplicate-Submit Checks

- Stats reads have no mutations and no idempotency key requirement.
- Mini Program pages prevent overlapping loads with local `loading`, `loadingMore`, or section state guards.
- Repeated refreshes converge by rereading current backend truth; no local optimistic stats are persisted.

## Recovery And Async Convergence Paths

- No worker, scheduler, callback, transaction, outbox, or websocket is owned by this slice.
- Refresh failure is user-visible. Initial full failure shows page-level error; section-level failures can preserve prior trusted values.
- Aggregate data changes only when the underlying order, reservation, dish, customer, or print-log truth changes in the owning domain.

## Frontend Draft And Backend Rehydration

- Stats overview uses backend truth on every load and records a visible `updatedAtLabel` when at least one section succeeds.
- Date range is a local query parameter choice, not stored merchant configuration.
- Customer list resets paging when sorting changes and reloads from backend truth.
- Customer detail reloads from backend truth on pull-down refresh and keeps prior detail only when a refresh fails after existing data is present.

## Test Coverage Signals

Observed tests:

- `locallife/api/merchant_stats_test.go` covers daily stats, overview including print anomalies, top dishes, customer list, customer detail, hourly stats, source stats, repurchase, category stats, authorization failures, invalid dates/limits/order_by, merchant-not-found behavior, and internal errors for key handlers.
- `locallife/api/order_test.go` covers `/v1/merchant/orders/stats`.
- `locallife/api/table_reservation_test.go` covers `/v1/reservations/merchant/stats`.

Missing high-value tests:

- No Mini Program test currently proves the stats overview page's partial-section preservation behavior end-to-end.
- No single integration test exercises the full stats overview fan-out across merchant stats, order stats, and reservation stats.
- Customer analytics PII exposure relies on API authz/unit tests and SQL predicates; a cross-merchant API regression test would be a useful future hardening target if this area changes.

## Gaps And Refactor Notes

- This slice closes the merchant-role artifact coverage gap for stats/analytics. It does not claim new code/test repair.
- The `merchant-state-flows` page coverage diff should now include `weapp/miniprogram/pages/merchant/stats/index.ts`, `stats/customers/index.ts`, and `stats/customers/detail/index.ts`.
- Keep future analytics endpoints in this slice unless they mutate order/reservation/finance state; write flows belong in their owning domain slice.

## Branch Exhaustion

- Entry branches checked: stats overview, customer list, customer detail, dashboard overview helper, order-stats dependency, and reservation-stats dependency. Flutter App has no merchant analytics entry in `merchant_app/lib/features/**`; Web is out of this merchant Mini Program slice unless a future merchant web analytics surface is added.
- Request branches checked: `/overview`, `/daily`, `/dishes/top`, `/customers`, `/customers/:user_id`, `/hourly`, `/sources`, `/repurchase`, `/categories`, `/v1/merchant/orders/stats`, and `/v1/reservations/merchant/stats`.
- Backend state branches checked: completed/user-delivered order aggregates, order status counts, print anomaly count, reservation status counts, customer aggregate rows, customer detail rows, favorite dishes, dish/category grouping, and merchant-derived auth context.
- Async branches checked: none owned; all reads are synchronous.
- Failure/retry branches checked: initial page errors, section-level errors, silent refresh preservation, customer-list pagination refresh, customer-detail refresh, invalid date ranges, invalid sort/limit/user id, unauthorized access, merchant-not-found, and store errors.
- Reader/consumer branches checked: merchant stats page, customer analytics pages, dashboard overview helper, backend merchant stats handlers, SQL aggregate queries, order stats, reservation stats, and tests.
- Authorization/tenant branches checked: owner/manager merchant stats middleware, server-side merchant resolution, `(merchant_id, user_id)` customer detail filtering, order stats merchant context, and reservation stats merchant context.
- Zombie/unreachable branches checked: CodeGraph and route/page scans found active registered stats pages and `/v1/merchant/stats/**`; missing artifact coverage is now closed here.
- Test-proof gaps checked: API unit tests cover core backend branches; Mini Program partial-degradation behavior and full fan-out integration remain useful future hardening, not active blockers.
