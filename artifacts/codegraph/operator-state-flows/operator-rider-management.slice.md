# Operator Rider Management Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G2 boundary - operator regional rider visibility, masked identity fields, rider deposit/status visibility, delivery performance stats exposure
Scope: Mini Program operator rider list/detail pages -> operator rider APIs -> region authority checks -> rider/delivery SQL -> rule-driven suspension and rider-deposit boundary

## Variant Coverage

This slice covers:

- Operator rider list page, including optional `region_id` and status entry parameters, refresh, pagination, status filter, search UI, and detail navigation.
- Operator rider detail page, including detail load, masked ID-card display, online/location/deposit/status fields, stats load, and retry states.
- Frontend service/API wrappers for `/v1/operator/riders`, `/summary`, `/:id`, and `/:id/stats`.
- Backend list/summary/detail/stats handlers in the operator route group.
- SQL reads over `riders`, `deliveries`, and operator-region authority tables.
- The explicit rule-driven boundary that operator rider management does not expose pause/resume actions.

This slice does not fully cover:

- Dashboard/analytics rider ranking. The API wrapper contains `getRiderRanking`, and dashboard/analytics services call it, but ranking ownership is covered by `operator-dashboard-analytics-notifications.slice.md`.
- Rider deposit payment, deposit withdrawal, credit/refund, or online-block recovery. Those are rider-role finance/deposit slices and operator rules boundary.
- Rule-driven status reconciliation after rider deposit rule changes; covered by `operator-region-rules-and-expansion.slice.md`.
- Platform/admin rider onboarding or approval workflows.

## Product Invariant

Operator rider management is a region-scoped visibility surface, not a second rider state machine:

- Rider detail and stats load the rider first and then check that the rider's stored region belongs to the operator.
- Rider list with an explicit `region_id` checks operator-region authority before reading.
- Rider list without `region_id` currently defaults to `operator.region_id`, while summary aggregates all managed regions through shared region-selection helpers.
- The UI displays status, online state, deposit amount, frozen deposit, earnings, and masked ID-card data; it does not mutate rider status or deposits.
- Operator route registration explicitly does not provide rider pause/resume entrypoints; suspension visibility is downstream of rule/platform-owned status updates.

## Primary Forward Chain

1. Operator Mini Program declares rider list and rider detail pages.
   Evidence: `weapp/miniprogram/app.json:43`, `weapp/miniprogram/app.json:44`.

2. Rider list accepts optional `region_id`/`status`, loads the first page, supports refresh/load-more/status filter/search debounce, and navigates to detail.
   Evidence: `weapp/miniprogram/pages/operator/riders/index.ts:40`, `weapp/miniprogram/pages/operator/riders/index.ts:69`, `weapp/miniprogram/pages/operator/riders/index.ts:79`, `weapp/miniprogram/pages/operator/riders/index.ts:115`, `weapp/miniprogram/pages/operator/riders/index.ts:135`, `weapp/miniprogram/pages/operator/riders/index.ts:140`.

3. Rider list service maps page state to query params, normalizes `pending` to `pending_approval`, adapts online/status labels, and calls the operator rider API wrapper.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-rider-management.ts:111`, `weapp/miniprogram/pages/operator/_api/operator-rider-management.ts:119`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:71`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:97`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:104`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:114`, `weapp/miniprogram/pages/operator/_api/operator-rider-management.ts:224`.

4. Backend list route is registered under `/v1/operator`; it requires operator context, defaults a missing region to `operator.region_id`, checks explicit requested regions, and reads paged rider rows/counts by region and optional status.
   Evidence: `locallife/api/server.go:1388`, `locallife/api/operator_merchant_rider.go:797`, `locallife/api/operator_merchant_rider.go:805`, `locallife/api/operator_merchant_rider.go:819`, `locallife/api/operator_merchant_rider.go:830`, `locallife/api/operator_merchant_rider.go:836`, `locallife/api/operator_merchant_rider.go:844`, `locallife/api/operator_merchant_rider.go:857`, `locallife/db/query/rider.sql:141`, `locallife/db/query/rider.sql:148`, `locallife/db/query/rider.sql:153`, `locallife/db/query/rider.sql:160`.

5. Summary route aggregates all server-resolved managed regions, counts status buckets, and counts online riders per managed region.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-rider-management.ts:232`, `locallife/api/server.go:1389`, `locallife/api/operator_merchant_rider.go:926`, `locallife/api/operator_merchant_rider.go:933`, `locallife/api/operator_merchant_rider.go:939`, `locallife/api/operator_merchant_rider.go:989`, `locallife/api/operator_merchant_rider.go:999`, `locallife/db/query/rider.sql:165`.

6. Rider detail page validates the `id`, loads detail first, then loads 30-day delivery stats as a secondary panel.
   Evidence: `weapp/miniprogram/pages/operator/riders/detail/index.ts:20`, `weapp/miniprogram/pages/operator/riders/detail/index.ts:34`, `weapp/miniprogram/pages/operator/riders/detail/index.ts:38`, `weapp/miniprogram/pages/operator/riders/detail/index.ts:45`, `weapp/miniprogram/pages/operator/riders/detail/index.ts:47`.

7. Detail/stats service functions adapt backend responses into status, online, deposit, frozen-deposit, earnings, completion-rate, and average-delivery-time view models.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:126`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:131`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:141`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:153`, `weapp/miniprogram/pages/operator/_services/operator-rider-management.ts:175`.

8. Backend detail route loads `riders`, rejects riders without a region, checks operator-region authority, masks ID-card number, and returns status/deposit/location/earnings fields.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-rider-management.ts:244`, `locallife/api/server.go:1390`, `locallife/api/operator_merchant_rider.go:1051`, `locallife/api/operator_merchant_rider.go:1059`, `locallife/api/operator_merchant_rider.go:1070`, `locallife/api/operator_merchant_rider.go:1076`, `locallife/api/operator_merchant_rider.go:1086`, `locallife/api/operator_merchant_rider.go:1091`, `locallife/api/operator_merchant_rider.go:1113`, `locallife/db/query/rider.sql:13`.

9. Backend stats route validates `days`, reloads and authorizes the rider, then reads delivery performance aggregates over the requested time window.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-rider-management.ts:268`, `locallife/api/server.go:1391`, `locallife/api/operator_merchant_rider.go:1153`, `locallife/api/operator_merchant_rider.go:1159`, `locallife/api/operator_merchant_rider.go:1163`, `locallife/api/operator_merchant_rider.go:1168`, `locallife/api/operator_merchant_rider.go:1177`, `locallife/api/operator_merchant_rider.go:1181`, `locallife/api/operator_merchant_rider.go:1189`, `locallife/db/query/operator_rider_stats.sql:3`.

10. Operator route registration documents the negative boundary: rider management has GET-only list/summary/detail/stats routes and no operator pause/resume routes.
    Evidence: `locallife/api/server.go:1387`, `locallife/api/server.go:1388`, `locallife/api/server.go:1391`, `locallife/api/server.go:1392`.

## SQL And Durable State Boundaries

- `operator_regions`: active operator-region authority used by explicit region checks and summary aggregation.
- `operators`: operator identity and legacy primary region used by rider list default region.
- `regions`: region metadata joined by operator-region helpers.
- `riders`: rider identity, region, status, online flag, deposit/frozen deposit, location, credit score, total orders, and total earnings.
- `deliveries`: rider performance source for total/completed deliveries, delayed count, average delivery time, and period earnings.

## Trust, Authorization, And Tenant Checks

- Rider routes require operator role and loaded operator context through the `/v1/operator` route group.
- Requested list `region_id` is checked with `checkOperatorManagesRegion`; no `region_id` defaults to `operator.region_id`.
- Summary uses server-side managed-region selection, not a client-owned region list.
- Detail and stats use `GetRider` first and authorize against the rider's stored nullable `region_id`; riders without a region are rejected.
- ID-card number is masked before response shaping.
- The current list handler does not bind or apply frontend `keyword` or `online_status`.

## Idempotency And Duplicate-Submit Checks

- List, summary, detail, and stats are idempotent reads.
- There are no operator-side rider write actions in this slice.
- Repeated refreshes rehydrate current rider state from SQL.
- Status and deposit changes can arrive from rider deposit/rules/platform paths; this page only observes the converged state.

## Recovery And Async Convergence Paths

- Rider list supports retry, pull-down refresh, silent refresh on show, pagination, search clear, and status-filter reload.
- Detail load failure blocks stats; stats failure is intentionally non-blocking for the main rider profile.
- If rider status/online/deposit state changes outside this page, list/detail reloads pick up the current durable `riders` row.
- Rider-deposit payment/withdrawal recovery and online-block convergence are outside this operator page.

## Frontend Draft And Backend Rehydration

- `region_id`, `status`, page number, and search keyword are local list state. Backend region authority and rider SQL state decide visibility.
- `pending` status is a frontend compatibility alias normalized to `pending_approval` before request.
- Detail/stats formatted values are view projections; the backend response remains the source of truth.
- The page displays online state from `is_online`; richer `online_status` values are not returned by current backend list/detail responses.

## Test Coverage Signals

Observed tests:

- `locallife/api/operator_merchant_rider_test.go` covers rider list happy path, status filtering, detail region denial/not-found, and all-managed-region summary aggregation including online counts.
- `locallife/db/sqlc/rider_test.go` covers `ListRidersByRegion`, pagination, status filtering, and regional counts at the SQL layer.
- Generated sqlc coverage exists for `GetOperatorRiderStats`, but no focused API test was observed in this slice for the stats route.

Missing high-value tests:

- API test for `GET /v1/operator/riders/:id/stats`, including region denial, no-region rider, default days, and invalid days.
- Contract/page test proving rider list search text either filters server-side or is intentionally not supported; current frontend sends `keyword`, backend ignores it.
- Backend list test for explicit managed/unmanaged `region_id` and for the multi-region mismatch where list defaults to primary region but summary aggregates all managed regions.
- Contract test that `offline` and `online_status` are not accepted list filters unless backend support is added.
- Mini Program test for stale/unmanaged `region_id` deep link into rider list and detail navigation.

## Gaps And Refactor Notes

- Rider list default scope differs from merchant list: merchant list aggregates all managed regions, while rider list defaults to `operator.region_id`. Decide whether that asymmetry is intended.
- `getRiderSummary(regionId?: number)` sends `region_id`, but backend summary currently calls `resolveOperatorRegionSelection(ctx)` and ignores requested region. Align API behavior or remove the parameter.
- Frontend rider list sends `keyword`, and the API type includes `online_status`, but `listOperatorRidersRequest` has neither field. Either remove the UI/API promise or add backend filtering.
- Frontend API has `SuspendOperatorRiderRequest` and `ResumeOperatorRiderRequest` types, but no current operator service method, page action, or backend route uses them. Treat them as stale non-entry types until a real action path lands.
- The API type includes `offline`, but backend `status` binding only accepts approved/active/suspended/pending_approval/rejected. `offline` is an online-state display concept here, not a backend rider status filter.
- Operator rider management is intentionally not the closure path for deposit payment, deposit withdrawal, rider online eligibility, or platform rider approval.

## Branch Exhaustion

- Entry branches checked: list direct open, dashboard/deep-link `region_id`, status filter, search debounce, search clear, pull refresh, load more, retry, detail navigation, invalid detail id, detail retry, stats success/failure.
- Request branches checked: `/v1/operator/riders`, `/v1/operator/riders/summary`, `/v1/operator/riders/:id`, `/v1/operator/riders/:id/stats`.
- Backend state branches checked: missing operator context, operator without primary region, requested unmanaged region, no-region rider, rider missing, rider outside region, empty list, status filter, summary across multiple managed regions, online count, stats days default/range.
- Durable-state branches checked: rider status/online/deposit/location read projection, masked ID-card response, delivery performance aggregate.
- Dead/orphan branches checked: operator rider suspend/resume request types are present only as unused frontend types; no live operator route or page action was found for rider suspend/resume.
