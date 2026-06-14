# Operator Merchant Management Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G2 boundary - operator regional merchant visibility, merchant capability writes, system-label reconciliation, merchant stats exposure
Scope: Mini Program operator merchant list/detail pages -> operator merchant APIs -> region authority checks -> merchant/capability/stat SQL -> capability transaction/system-label reconciliation -> recovery and food-safety boundary

## Variant Coverage

This slice covers:

- Operator merchant list page, including optional `region_id` and status entry parameters, refresh, pagination, status filter, search UI, and detail navigation.
- Operator merchant detail page, including detail load, capability load/edit, stats load, retry states, and capability editor gating.
- Frontend service/API wrappers for `/v1/operator/merchants`, `/summary`, `/:id`, `/:id/capabilities`, and `/:id/stats`.
- Backend list/summary/detail/capability/stats handlers in the operator route group.
- SQL reads over `merchants`, `orders`, `order_items`, `dishes`, and operator-region authority tables.
- Capability write transaction that upserts `merchant_capabilities` and reconciles `merchant_system_labels` from the system tag catalog.

This slice does not fully cover:

- Dashboard merchant ranking; the API wrapper contains `getMerchantRanking`, but the current ranking route/page ownership is covered by `operator-dashboard-analytics-notifications.slice.md`.
- Merchant pause/resume/suspension. The current operator route group has no suspend/resume merchant endpoints; food-safety and recovery flows own those actions.
- Merchant onboarding/application review; operator merchant management here reads current `merchants` state, not application approval.
- Customer-facing merchant discovery effects after system-label changes; this slice records the label write boundary only.

## Product Invariant

Operator merchant management is a region-scoped read surface with one narrow write path:

- Merchant list without `region_id` aggregates all active regions managed by the current operator; a requested `region_id` must pass `checkOperatorManagesRegion`.
- Merchant detail, capabilities, and stats load the merchant first and then check that the merchant's `region_id` belongs to the operator before returning data or writing capabilities.
- Capability edits can only update `open_kitchen_status`, `dine_in_status`, and `note`; the transaction then derives system labels from the stored capability truth.
- Operator pages do not expose merchant suspend/resume. Suspension/recovery closure belongs to food-safety/recovery/admin-owned paths.
- Frontend filters are convenience state. Backend region checks and SQL state are the source of visibility truth.

## Primary Forward Chain

1. Operator Mini Program declares merchant list and merchant detail pages.
   Evidence: `weapp/miniprogram/app.json:41`, `weapp/miniprogram/app.json:42`.

2. Merchant list accepts optional `region_id`/`status`, loads the first page, supports refresh/load-more/status filtering/search debounce, and navigates to detail.
   Evidence: `weapp/miniprogram/pages/operator/merchants/index.ts:44`, `weapp/miniprogram/pages/operator/merchants/index.ts:77`, `weapp/miniprogram/pages/operator/merchants/index.ts:91`, `weapp/miniprogram/pages/operator/merchants/index.ts:134`, `weapp/miniprogram/pages/operator/merchants/index.ts:155`, `weapp/miniprogram/pages/operator/merchants/index.ts:163`.

3. Merchant list service maps page state to query params and calls the operator merchant API wrapper.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:156`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:163`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:173`, `weapp/miniprogram/pages/operator/_api/operator-merchant-management.ts:188`.

4. Backend list route is registered under `/v1/operator`, resolves managed-region selection, treats `approved` as approved plus active, reads per-region merchant rows/counts, sorts by newest, and returns a paged response.
   Evidence: `locallife/api/server.go:1380`, `locallife/api/operator_merchant_rider.go:68`, `locallife/api/operator_merchant_rider.go:99`, `locallife/api/operator_merchant_rider.go:176`, `locallife/api/operator_merchant_rider.go:191`, `locallife/api/operator_merchant_rider.go:197`, `locallife/api/operator_merchant_rider.go:233`, `locallife/db/query/merchant.sql:669`, `locallife/db/query/merchant.sql:677`, `locallife/db/query/merchant.sql:683`, `locallife/db/query/merchant.sql:692`.

5. Summary route aggregates merchant counts across all managed regions and status buckets for the current operator.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-merchant-management.ts:196`, `locallife/api/server.go:1381`, `locallife/api/operator_merchant_rider.go:257`, `locallife/api/operator_merchant_rider.go:264`, `locallife/api/operator_merchant_rider.go:270`, `locallife/api/operator_merchant_rider.go:320`.

6. Merchant detail page validates the `id`, loads detail first, then loads capabilities and 30-day stats as secondary panels.
   Evidence: `weapp/miniprogram/pages/operator/merchants/detail/index.ts:59`, `weapp/miniprogram/pages/operator/merchants/detail/index.ts:73`, `weapp/miniprogram/pages/operator/merchants/detail/index.ts:77`, `weapp/miniprogram/pages/operator/merchants/detail/index.ts:84`, `weapp/miniprogram/pages/operator/merchants/detail/index.ts:87`.

7. Detail/capabilities/stats service functions adapt backend responses into view models and formatted finance/stat values.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:104`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:127`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:185`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:190`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:207`.

8. Backend merchant detail route loads `merchants`, checks the merchant region, and returns public card-logo URL when a logo asset exists.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-merchant-management.ts:208`, `locallife/api/server.go:1382`, `locallife/api/operator_merchant_rider.go:562`, `locallife/api/operator_merchant_rider.go:570`, `locallife/api/operator_merchant_rider.go:581`, `locallife/api/operator_merchant_rider.go:586`, `locallife/api/operator_merchant_rider.go:603`.

9. Capabilities read route loads the merchant, checks region authority, reads `merchant_capabilities`, falls back to unknown/default capability truth when absent, and returns stored or derived system labels.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-merchant-management.ts:215`, `locallife/api/server.go:1383`, `locallife/api/operator_merchant_rider.go:426`, `locallife/api/operator_merchant_rider.go:433`, `locallife/api/operator_merchant_rider.go:442`, `locallife/api/operator_merchant_rider.go:447`, `locallife/api/operator_merchant_rider.go:463`, `locallife/api/operator_merchant_rider.go:472`, `locallife/db/query/merchant_system_label.sql:6`, `locallife/db/query/merchant_system_label.sql:33`.

10. Capability editor only opens after capability load succeeds; submit sends selected statuses plus a trimmed note and refreshes local capability state from the backend response.
    Evidence: `weapp/miniprogram/pages/operator/merchants/detail/index.ts:123`, `weapp/miniprogram/pages/operator/merchants/detail/index.ts:159`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:195`, `weapp/miniprogram/pages/operator/_services/operator-merchant-management.ts:199`.

11. Backend capability update rejects an empty patch, checks merchant-region authority, then runs `UpdateMerchantCapabilitiesTx` with `manual_review` source.
    Evidence: `weapp/miniprogram/pages/operator/_api/operator-merchant-management.ts:222`, `locallife/api/server.go:1384`, `locallife/api/operator_merchant_rider.go:491`, `locallife/api/operator_merchant_rider.go:502`, `locallife/api/operator_merchant_rider.go:507`, `locallife/api/operator_merchant_rider.go:516`, `locallife/api/operator_merchant_rider.go:521`, `locallife/api/operator_merchant_rider.go:538`.

12. Capability transaction upserts capability truth, loads system tags, reconciles desired `merchant_system_labels`, and returns the final capability plus active labels.
    Evidence: `locallife/db/sqlc/tx_merchant_capabilities.go:34`, `locallife/db/sqlc/tx_merchant_capabilities.go:37`, `locallife/db/sqlc/tx_merchant_capabilities.go:38`, `locallife/db/sqlc/tx_merchant_capabilities.go:50`, `locallife/db/sqlc/tx_merchant_capabilities.go:55`, `locallife/db/sqlc/tx_merchant_capabilities.go:59`, `locallife/db/sqlc/merchant_system_label_reconciler.go:30`, `locallife/db/sqlc/merchant_system_label_reconciler.go:80`, `locallife/db/query/merchant_system_label.sql:11`, `locallife/db/query/merchant_system_label.sql:48`, `locallife/db/query/merchant_system_label.sql:60`.

13. Stats route validates `days`, checks merchant-region authority, then reads overview, repurchase, and top-dish aggregates over completed/user-delivered order state.
    Evidence: `weapp/miniprogram/pages/operator/_api/operator-merchant-management.ts:250`, `locallife/api/server.go:1385`, `locallife/api/operator_merchant_rider.go:651`, `locallife/api/operator_merchant_rider.go:657`, `locallife/api/operator_merchant_rider.go:661`, `locallife/api/operator_merchant_rider.go:666`, `locallife/api/operator_merchant_rider.go:675`, `locallife/api/operator_merchant_rider.go:683`, `locallife/api/operator_merchant_rider.go:694`, `locallife/api/operator_merchant_rider.go:705`, `locallife/db/query/merchant_stats.sql:20`, `locallife/db/query/merchant_stats.sql:38`, `locallife/db/query/merchant_stats.sql:165`.

14. Platform/admin food-safety merchant suspension route exists separately and is not reachable through the operator merchant-management route group.
    Evidence: `locallife/api/server.go:1380`, `locallife/api/server.go:1385`, `locallife/api/server.go:1531`.

## SQL And Durable State Boundaries

- `operator_regions`: active operator-region authority used by region selection and region checks.
- `operators`: operator identity and legacy primary region fallback through shared region-selection helpers.
- `regions`: region metadata joined by operator-region helpers.
- `merchants`: current merchant visibility, status, opening state, owner, region, logo asset, and coordinates.
- `merchant_capabilities`: durable open-kitchen/dine-in capability truth plus manual/default source and note.
- `merchant_system_labels`: derived system label links used by display/search surfaces.
- `tags`: system tag catalog required by capability reconciliation.
- `orders`: stats source for total orders, sales, commission, and repeat customers.
- `order_items`: top-dish quantity and revenue source.
- `dishes`: top-dish name/price source.

## Trust, Authorization, And Tenant Checks

- Merchant routes require operator role and loaded operator context through the `/v1/operator` route group.
- Requested list `region_id` is checked with `checkOperatorManagesRegion`; no region parameter aggregates server-resolved managed regions.
- Detail, capability read/write, and stats use `GetMerchant` first and authorize against the merchant's stored `region_id`.
- Capability update does not trust frontend labels; backend writes capability fields and derives system labels in a transaction.
- The current list handler does not bind or apply frontend `keyword`; search text should be treated as UI-only until backend support is added.

## Idempotency And Duplicate-Submit Checks

- List, summary, detail, capability read, and stats are idempotent reads.
- Repeating the same capability patch converges to the same `merchant_capabilities` values and desired system labels.
- `UpsertMerchantCapabilities` is keyed by `merchant_id`; `UpsertMerchantSystemLabel` is keyed by `(merchant_id, tag_id)`.
- Reconciler removes no-longer-desired labels and avoids timestamp churn when a retained label already has the same source.
- There is no idempotency key for capability PATCH; this is acceptable for convergent field updates, but concurrent edits use last-writer-wins semantics.

## Recovery And Async Convergence Paths

- Merchant list supports retry, pull-down refresh, silent refresh on show, and pagination retry.
- Detail load failure blocks secondary panels; capability and stats failures do not mutate merchant state.
- Capability editor is gated on a successful capability load. After submit, the page uses the backend response as the new capability truth.
- Stats failure is intentionally non-blocking for the detail page and can be retried by reloading the page.
- Food-safety/recovery actions can later change merchant status outside this slice; this page rehydrates current merchant status on reload.

## Frontend Draft And Backend Rehydration

- `region_id`, `status`, page number, and search keyword are local list state. Backend region authority decides visibility.
- Capability form values are local draft state only while the editor is open; persisted truth comes from the PATCH response.
- Empty capability note is intentionally sent as an empty string after trim, allowing the user to clear a note.
- The page does not locally derive or persist system labels; it displays backend-returned labels or fallback labels from the backend response.

## Test Coverage Signals

Observed tests:

- `locallife/api/operator_merchant_rider_test.go` covers merchant list, approved-status expansion to active, all-managed-region aggregation, invalid auth/query branches, detail region denial/not-found, summary aggregation, capability read/default fallback, and capability update.
- `locallife/db/sqlc/tx_merchant_capabilities_test.go` covers capability transaction label reconciliation and retained-label source/timestamp behavior.
- `weapp/scripts/check-operator-merchant-capabilities.test.js` covers Mini Program API/service/page/WXML wiring for capability load/edit/display and clearing notes.

Missing high-value tests:

- Contract/page test proving operator merchant list search text either filters server-side or is intentionally not supported; current frontend sends `keyword`, backend ignores it.
- Backend summary test for `?region_id=` semantics if region-scoped summary is intended; current handler binds the request but aggregates all managed regions.
- Mini Program test for stale/unmanaged `region_id` deep link into merchant list and detail navigation.
- Concurrency test for two capability PATCH requests with different fields/notes to document last-writer-wins behavior.

## Gaps And Refactor Notes

- Frontend merchant list sends `keyword`, but `listOperatorMerchantsRequest` has no `keyword` field and list SQL does not filter by keyword. Either remove the UI/API promise or add backend keyword filtering.
- Frontend API has `SuspendOperatorMerchantRequest` and `ResumeOperatorMerchantRequest` types, but no current operator service method, page action, or backend route uses them. Treat them as stale non-entry types until a real action path lands.
- `getMerchantSummary(regionId?: number)` sends `region_id`, but backend summary currently calls `resolveOperatorRegionSelection(ctx)` and ignores requested region. Align API behavior or remove the parameter.
- Capability labels may affect customer-facing search/display through `merchant_system_labels`; this slice records the write, but customer discovery behavior needs a customer/merchant-search slice if changed.
- Operator merchant management is intentionally not the closure path for food-safety suspension, recovery dispute resolution, refund, or merchant onboarding.

## Branch Exhaustion

- Entry branches checked: list direct open, dashboard/deep-link `region_id`, status filter, search debounce, search clear, pull refresh, load more, retry, detail navigation, invalid detail id, detail retry, capability retry, capability editor open blocked by loading/error/no data, capability submit success/failure.
- Request branches checked: `/v1/operator/merchants`, `/v1/operator/merchants/summary`, `/v1/operator/merchants/:id`, `/v1/operator/merchants/:id/capabilities` GET/PATCH, `/v1/operator/merchants/:id/stats`.
- Backend state branches checked: no operator context, no managed regions, requested unmanaged region, empty list, multiple managed regions, `approved` plus `active`, merchant missing, merchant outside region, missing capability default, empty capability patch, stats days default/range.
- Durable-state branches checked: capability upsert insert/update, system label add/remove/retain, stats overview/repurchase/top-dishes, public logo URL projection.
- Dead/orphan branches checked: operator merchant suspend/resume request types are present only as unused frontend types; no live operator route or page action was found for merchant suspend/resume.
