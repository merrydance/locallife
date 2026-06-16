# Operator Dashboard Analytics Notifications Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G2 - regional authority, operator operational visibility, finance summary, dispatch-timeout notifications, read-state mutation
Scope: Mini Program operator dashboard/analytics/notifications pages -> operator region, analytics, finance, notification APIs -> operator route groups -> SQL reads/writes over regions, profit-sharing stats, merchant/rider counts, and operator-audience notifications

## Variant Coverage

This slice covers:

- Operator dashboard page loading managed regions, dashboard stats, finance summary card, merchant/rider rankings, and latest notification summary.
- Operator analytics page loading managed regions, realtime stats, daily trends, optional region stats, and merchant/rider rankings.
- Operator notification list/detail pages, including category filter, pagination, mark single read, mark all read, detail auto-read, and dispatch-hall handoff.
- Backend `/v1/operator/regions`, `/regions/:region_id/stats`, `/stats/realtime`, `/trend/daily`, `/merchants/ranking`, and `/riders/ranking`.
- Backend `/v1/operators/me/finance/overview` as the dashboard finance summary source.
- Backend `/v1/operators/me/notifications/**` list/summary/detail/read/read-all routes.

This slice does not fully cover:

- The dispatch hall itself after notification detail hands off by region id; covered by planned `operator-dispatch-hall`.
- Finance bills, Baofu settlement account, and Baofu withdrawal workflows; covered by planned `operator-finance-and-baofu-withdrawal`.
- Merchant/rider management list/detail pages beyond their ranking data used by dashboard/analytics; covered by planned merchant/rider slices.
- Platform-level operator management and cross-role alert resolution ownership.

## Product Invariant

Operator dashboard and analytics must stay scoped to the current operator's managed regions:

- Region selectors are built from server-side operator-region relationships, with legacy fallback to the operator's primary region.
- Region-scoped reads must reject unauthorized `region_id`; all-region mode aggregates only regions returned by the operator region selection helper.
- Dashboard finance summary reads successful profit-sharing truth; it is not a Baofu withdrawal balance and does not mutate money state.
- Operator notifications are user-scoped and additionally require `extra_data.audience='operator'`.
- Notification read/read-all writes only the current operator user's operator-audience notifications.

## Primary Forward Chain

1. The operator Mini Program package declares dashboard, analytics, notification list, and notification detail entries.
   Evidence: `weapp/miniprogram/app.json:36`, `weapp/miniprogram/app.json:37`, `weapp/miniprogram/app.json:39`, `weapp/miniprogram/app.json:40`.

2. Dashboard initializes managed-region picker state, then loads dashboard data for the selected region and time dimension.
   Evidence: `weapp/miniprogram/pages/operator/dashboard/index.ts:96`, `weapp/miniprogram/pages/operator/dashboard/index.ts:123`.

3. Dashboard data service concurrently loads finance overview, realtime stats, merchant ranking, rider ranking, daily trend, and notification summary; non-critical ranking/trend/finance/notification reads degrade locally where the service explicitly catches them, while realtime stats remains required for the main dashboard.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:88`, `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:103`, `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:104`, `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:105`, `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:107`, `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:109`, `weapp/miniprogram/pages/operator/_services/operator-workbench.ts:111`.

4. Analytics page initializes the same managed-region picker and loads realtime stats, daily trends, optional region stats, merchant ranking, and rider ranking for current/previous period comparison.
   Evidence: `weapp/miniprogram/pages/operator/analytics/index.ts:53`, `weapp/miniprogram/pages/operator/analytics/index.ts:69`, `weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts:113`, `weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts:124`, `weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts:125`, `weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts:127`, `weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts:130`, `weapp/miniprogram/pages/operator/_services/operator-analytics-dashboard.ts:135`.

5. Managed-region frontend state calls `GET /v1/operator/regions`; backend returns active `operator_regions` rows first, includes legacy `operators.region_id` when needed, and falls back to the old role-bound region only when no operator-region relationship is available.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-regions.ts:43`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:226`, `locallife/api/server.go:1361`, `locallife/api/operator_stats.go:129`, `locallife/api/operator_stats.go:148`, `locallife/db/query/operator_region.sql:15`.

6. Realtime stats route accepts an optional selected region id, verifies operator management before using it, otherwise resolves the operator default region, and reads active/pending merchant and rider counts.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-analytics.ts:153`, `locallife/api/server.go:1369`, `locallife/api/operator_realtime.go:32`, `locallife/api/operator_realtime.go:39`, `locallife/api/operator_realtime.go:45`, `locallife/api/operator_realtime.go:62`, `locallife/api/operator_realtime.go:72`, `locallife/api/operator_realtime.go:82`, `locallife/api/operator_realtime.go:92`.

7. Region stats, daily trends, merchant ranking, and rider ranking all resolve operator region selection before querying region/profit-sharing/order/delivery truth.
   Evidence: `locallife/api/server.go:1362`, `locallife/api/server.go:1372`, `locallife/api/server.go:1373`, `locallife/api/server.go:1374`, `locallife/api/operator_stats.go:60`, `locallife/api/operator_stats.go:245`, `locallife/api/operator_stats.go:359`, `locallife/api/operator_stats.go:515`.

8. Region, trend, and ranking SQL use `profit_sharing_orders(status='finished')` for GMV/commission truth and delivery/order joins for rider performance.
   Evidence: `locallife/db/query/operator_stats.sql:7`, `locallife/db/query/operator_stats.sql:26`, `locallife/db/query/operator_stats.sql:50`, `locallife/db/query/operator_stats.sql:73`, `locallife/db/query/operator_stats.sql:92`.

9. Dashboard finance summary calls `/v1/operators/me/finance/overview`; backend resolves current operator, applies the same operator-region selection, sums current-month and all-time successful profit-sharing region stats, and derives operator share ratio from actual operator commission over total commission.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:258`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:265`, `locallife/api/server.go:1415`, `locallife/api/operator_stats.go:686`, `locallife/api/operator_stats.go:694`, `locallife/api/operator_stats.go:719`, `locallife/api/operator_stats.go:737`, `locallife/api/operator_stats.go:757`, `locallife/api/operator_stats.go:773`, `locallife/api/operator_stats.go:790`.

10. Dashboard notification card opens notification list; notification detail can hand off to dispatch hall with the notification's region id.
    Evidence: `weapp/miniprogram/pages/operator/dashboard/index.ts:227`, `weapp/miniprogram/pages/operator/notifications/detail/index.ts:63`.

11. Notification list loads `/v1/operators/me/notifications` and optionally `/summary`, supports `dispatch_timeout` category filtering, marks a row read before navigation, and can mark all unread rows read.
    Evidence: `weapp/miniprogram/pages/operator/notifications/index.ts:77`, `weapp/miniprogram/pages/operator/_services/operator-notification-center.ts:97`, `weapp/miniprogram/pages/operator/_api/operator-notification.ts:44`, `weapp/miniprogram/pages/operator/_api/operator-notification.ts:62`, `weapp/miniprogram/pages/operator/_api/operator-notification.ts:72`, `weapp/miniprogram/pages/operator/_api/operator-notification.ts:79`.

12. Notification detail reads `/v1/operators/me/notifications/:id`, then best-effort marks it read through the same operator notification service.
    Evidence: `weapp/miniprogram/pages/operator/notifications/detail/index.ts:37`, `weapp/miniprogram/pages/operator/_services/operator-notification-center.ts:135`, `weapp/miniprogram/pages/operator/_api/operator-notification.ts:69`, `weapp/miniprogram/pages/operator/_api/operator-notification.ts:72`.

13. Backend notification routes are under `/v1/operators/me`, use the loaded operator user id, filter `notifications.extra_data->>'audience'='operator'`, and scope read writes by `notifications.user_id`.
    Evidence: `locallife/api/server.go:1424`, `locallife/api/server.go:1425`, `locallife/api/server.go:1426`, `locallife/api/server.go:1427`, `locallife/api/server.go:1428`, `locallife/api/operator_notification.go:175`, `locallife/api/operator_notification.go:246`, `locallife/api/operator_notification.go:295`, `locallife/api/operator_notification.go:338`, `locallife/api/operator_notification.go:378`, `locallife/db/query/notification.sql:38`, `locallife/db/query/notification.sql:47`, `locallife/db/query/notification.sql:54`, `locallife/db/query/notification.sql:70`, `locallife/db/query/notification.sql:89`.

## SQL And Durable State Boundaries

- `operator_regions`: current operator managed-region authority for selectors and region-scoped reads.
- `operators`: operator identity and legacy primary region fallback.
- `regions`: region names and ids shown in selectors and summaries.
- `merchants`, `riders`, `orders`, `deliveries`: realtime counts and ranking context.
- `profit_sharing_orders`: successful GMV/commission/operator-income truth for dashboard, analytics, and finance summary.
- `payment_orders`: active-user context for daily trend SQL.
- `notifications`: operator-audience dispatch/system notifications, read state, and related dispatch region metadata.

## Trust, Authorization, And Tenant Checks

- Both `/v1/operator` and `/v1/operators/me` route groups require `CasbinRoleMiddleware(RoleOperator)` and `LoadOperatorMiddleware`.
- Selected region ids are verified by `checkOperatorManagesRegion`; all-region mode is derived from server-side operator-region selection, not from client-supplied ids.
- Notification list/detail/read/read-all resolve the operator user id from context and require `extra_data.audience='operator'`.
- Notification detail and read return 404 for notifications outside the current operator user's operator-audience scope.

## Idempotency And Duplicate-Submit Checks

- Dashboard/analytics/region/stat/ranking reads are idempotent.
- Mark single notification read is conditional on `is_read=false`; repeating it returns not-found/already-read semantics.
- Mark all read is idempotent at the current operator user and operator-audience scope.
- Notification detail best-effort read marking does not block detail display.

## Recovery And Async Convergence Paths

- Dashboard and analytics rehydrate on retry, region change, and page show.
- Dashboard service degrades finance/ranking/trend/notification summary reads that are not essential to main realtime status.
- Notification list preserves pagination state and can refresh summary unread count independently.
- Dispatch timeout notification source and dispatch-hall handling are outside this slice; this slice records only operator notification read-side handling and handoff.

## Frontend Draft And Backend Rehydration

- Region selection state is frontend UI state only; backend region authority is rechecked on every scoped request.
- Dashboard and analytics view models adapt API responses into role-readable metrics and ranking rows before rendering.
- Notification list/detail treat backend notification rows as canonical and update local read state only after successful read/write calls, except detail auto-read which is best effort.

## Test Coverage Signals

Observed tests:

- `locallife/api/operator_stats_test.go` covers operator stats and finance overview behavior.
- `locallife/api/operator_notification_test.go` covers operator notification list, summary, detail, and audience scoping.
- `locallife/api/operator_access_test.go` covers operator region access helpers.

Missing high-value tests:

- Mini Program dashboard weak-network behavior when required realtime stats fails but degraded side reads succeed.
- End-to-end dispatch timeout alert creation -> operator notification summary/list/detail -> dispatch-hall handoff.
- Contract coverage that frontend analytics region summary shape matches the current backend `regionStatsResponse`, which is flatter than older richer DTO comments in the Mini Program wrapper.

## Gaps And Refactor Notes

- The Mini Program `OperatorRegionStatsResponse` and analytics service still describe richer nested region stats fields than the backend currently returns from `regionStatsResponse`; current analytics code tolerates missing region stats by showing fallback summary, but the type/comments should be aligned in a later frontend cleanup.
- Dashboard finance summary is intentionally a read over profit-sharing stats, not Baofu withdrawal balance; do not reuse it as cash-out availability.
- Keep notification category filtering tied to `extra_data.category`; adding new operator notification categories requires both backend allow-list and frontend tabs/copy updates.

## Branch Exhaustion

- Entry branches checked: dashboard load/show/retry, analytics load/retry, region picker change, time dimension change, ranking type change, notification list load/show/pull refresh/load more/category filter/mark single read/mark all read, notification detail load/retry/dispatch handoff.
- Request branches checked: `/v1/operator/regions`, `/regions/:region_id/stats`, `/stats/realtime`, `/trend/daily`, `/merchants/ranking`, `/riders/ranking`, `/v1/operators/me/finance/overview`, `/notifications`, `/notifications/summary`, `/notifications/:id`, `/notifications/:id/read`, `/notifications/read-all`.
- Backend state branches checked: no operator context, invalid or unauthorized region id, legacy primary region fallback, all-region aggregation, date range parse failure, empty region list fallback, notification non-owner/non-operator-audience, already-read notification, and empty notification summary.
- Async branches checked: dashboard degraded side reads, notification detail best-effort auto-read, dispatch notification handoff to planned dispatch-hall slice.
- Dead/orphan branches checked: no dead dashboard/analytics/notification route found in this slice.
