# Operator Region Rules And Expansion Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G2/G3 boundary - regional authority, pricing/rider-deposit rule writes, weather coefficient cache invalidation, rider operational status reconciliation, region expansion approval handoff
Scope: Mini Program operator region/rules/config/fee/timeslot/region-expansion pages -> operator region/rules/delivery-fee/region APIs -> SQL region authority and rule/config tables -> platform approval boundary for new regions -> rule-engine proxy/read-hit boundary

## Variant Coverage

This slice covers:

- Operator region list page, region config overview, delivery-fee config page, rules page, peak-hour timeslot page, and region-expansion page.
- Managed-region selection through `/v1/operator/regions`, plus navigation into delivery-fee, peak-hour, and rule configuration pages.
- Direct delivery-fee config read/write through `/v1/delivery-fee/regions/:region_id/config`.
- Peak-hour list/create/delete through `/v1/operator/regions/:region_id/peak-hours` and `/v1/operator/peak-hours/:id`.
- Operator editable rules through `/v1/operator/rules` and `/v1/operator/rules/:key`, including rider deposit, delivery-fee parameters, weather coefficient rules, and read-only platform-maintained values.
- Region expansion application list and submit through `/v1/operator/region-expansion`, plus the platform/admin approval boundary that actually writes `operator_regions`.
- Operator rule-engine proxy routes under `/v1/operators/me/rules/**`, including region-constrained rule versions and rule-hit reads.

This slice does not fully cover:

- Platform/admin UI for approving region-expansion applications; this slice records the backend approval boundary only.
- Rider deposit payment/withdrawal flows after a rider becomes eligible or ineligible; covered by rider deposit and rider income slices.
- Delivery fee calculation for customer/cart/order pricing; this slice records config ownership, not every fee consumption site.
- Platform-global operator rule defaults; this slice only records operator-visible fallback and proxy boundaries.

## Product Invariant

Operator regional configuration must stay bounded to regions the operator manages:

- Region list and region selectors are derived from server-side `operator_regions`, with legacy `operators.region_id` fallback only where backend helpers explicitly allow it.
- Delivery-fee writes require operator auth plus `ValidateOperatorRegionMiddleware("region_id")`.
- Peak-hour creates/deletes check that the operator manages the target region before writing.
- Rules written through `/v1/operator/rules/:key` first resolve a target region and reject unmanaged region ids.
- Region-expansion submit only creates or refreshes an application; it does not grant region authority until a platform/admin approval transaction writes `operator_regions`.
- Rule-engine proxy writes force the rule version scope/gray config to include the current operator region, and hit reads are filtered by `region_id`.

## Primary Forward Chain

1. Operator Mini Program declares the region, region config, rules, timeslot, delivery-fee, and region-expansion pages.
   Evidence: `weapp/miniprogram/app.json:45`, `weapp/miniprogram/app.json:47`, `weapp/miniprogram/app.json:48`, `weapp/miniprogram/app.json:49`, `weapp/miniprogram/app.json:59`.

2. Region list loads managed regions and routes either to region config or region-scoped rules depending on the entry target.
   Evidence: `weapp/miniprogram/pages/operator/region/index.ts:20`, `weapp/miniprogram/pages/operator/region/index.ts:43`, `weapp/miniprogram/pages/operator/region/index.ts:68`, `weapp/miniprogram/pages/operator/region/index.ts:76`, `weapp/miniprogram/pages/operator/region/index.ts:82`.

3. Managed-region service calls `GET /v1/operator/regions`; backend reads active `operator_regions` rows and returns region id/name/code context for selectors.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-regions.ts:43`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:226`, `locallife/api/server.go:1361`, `locallife/api/operator_stats.go:128`, `locallife/db/query/operator_region.sql:15`.

4. Region config overview loads delivery-fee config and peak-hour configs concurrently, then navigates into the dedicated delivery-fee and timeslot pages.
   Evidence: `weapp/miniprogram/pages/operator/region/config.ts:46`, `weapp/miniprogram/pages/operator/region/config.ts:65`, `weapp/miniprogram/pages/operator/region/config.ts:71`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:150`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:194`.

5. Delivery-fee page loads current region config, edits local form state, and saves via the operator delivery-fee shared API wrapper.
   Evidence: `weapp/miniprogram/pages/operator/delivery-fee/index.ts:29`, `weapp/miniprogram/pages/operator/delivery-fee/index.ts:50`, `weapp/miniprogram/pages/operator/delivery-fee/index.ts:86`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:109`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:122`, `weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts:189`, `weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts:201`.

6. Backend delivery-fee write routes are under `/v1/delivery-fee`, require operator role and region middleware, then create/update `delivery_fee_configs` and write audit logs.
   Evidence: `locallife/api/server.go:881`, `locallife/api/server.go:885`, `locallife/api/server.go:887`, `locallife/api/server.go:888`, `locallife/api/casbin_enforcer.go:501`, `locallife/api/delivery_fee.go:317`, `locallife/api/delivery_fee.go:367`, `locallife/api/delivery_fee.go:480`, `locallife/api/delivery_fee.go:602`, `locallife/db/query/delivery_fee_config.sql:1`, `locallife/db/query/delivery_fee_config.sql:37`.

7. Timeslot page lists, validates, creates, and deletes peak-hour configs. Frontend checks obvious overlap, but backend authority remains the operator-region check.
   Evidence: `weapp/miniprogram/pages/operator/timeslot/index.ts:60`, `weapp/miniprogram/pages/operator/timeslot/index.ts:85`, `weapp/miniprogram/pages/operator/timeslot/index.ts:171`, `weapp/miniprogram/pages/operator/timeslot/index.ts:222`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:134`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:142`, `weapp/miniprogram/pages/operator/_services/operator-region-config.ts:146`, `weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts:229`, `weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts:239`, `weapp/miniprogram/pages/operator/_main_shared/api/delivery-fee.ts:250`.

8. Backend peak-hour handlers read/write `peak_hour_configs`; create checks `checkOperatorManagesRegion(req.RegionID)`, delete reloads the config first and then checks its region.
   Evidence: `locallife/api/server.go:1365`, `locallife/api/server.go:1366`, `locallife/api/server.go:1377`, `locallife/api/delivery_fee.go:704`, `locallife/api/delivery_fee.go:714`, `locallife/api/delivery_fee.go:740`, `locallife/api/delivery_fee.go:784`, `locallife/api/delivery_fee.go:791`, `locallife/api/delivery_fee.go:824`, `locallife/api/delivery_fee.go:834`, `locallife/api/delivery_fee.go:845`, `locallife/api/delivery_fee.go:850`, `locallife/db/query/peak_hour_config.sql:1`, `locallife/db/query/peak_hour_config.sql:18`, `locallife/db/query/peak_hour_config.sql:41`.

9. Rules page requires a selected region, lists rules, opens editable rule modal or navigates peak-hour rules, validates values locally, and calls the operator rules API.
   Evidence: `weapp/miniprogram/pages/operator/rules/index.ts:60`, `weapp/miniprogram/pages/operator/rules/index.ts:78`, `weapp/miniprogram/pages/operator/rules/index.ts:104`, `weapp/miniprogram/pages/operator/rules/index.ts:115`, `weapp/miniprogram/pages/operator/rules/index.ts:140`, `weapp/miniprogram/pages/operator/_services/operator-rules-management.ts:45`, `weapp/miniprogram/pages/operator/_services/operator-rules-management.ts:64`, `weapp/miniprogram/pages/operator/_api/operator-rules.ts:31`, `weapp/miniprogram/pages/operator/_api/operator-rules.ts:39`.

10. Backend rules list resolves the operator target region, reads region rule config, platform fallback config, profit-sharing config, delivery-fee config, weather rules/latest weather, and returns editable/read-only rule items.
    Evidence: `locallife/api/server.go:1407`, `locallife/api/operator_rules.go:190`, `locallife/api/operator_rules.go:222`, `locallife/api/operator_rules.go:246`, `locallife/api/operator_rules.go:255`, `locallife/api/operator_rules.go:263`, `locallife/api/operator_rules.go:275`, `locallife/api/operator_rules.go:340`, `locallife/api/operator_rules.go:496`, `locallife/db/query/region_rule_config.sql:1`, `locallife/db/query/behavior_trace.sql:30`, `locallife/db/query/profit_sharing_config.sql:3`, `locallife/db/query/weather_coefficient.sql:34`.

11. Backend rules update rejects read-only/platform-only keys, writes rider deposit to both legacy operator and region rule config, reconciles rider operational status by region, writes delivery-fee parameters, or writes weather coefficient rules and invalidates weather cache.
    Evidence: `locallife/api/server.go:1408`, `locallife/api/operator_rules.go:547`, `locallife/api/operator_rules.go:562`, `locallife/api/operator_rules.go:576`, `locallife/api/operator_rules.go:595`, `locallife/api/operator_rules.go:619`, `locallife/api/operator_rules.go:627`, `locallife/api/operator_rules.go:635`, `locallife/api/operator_rules.go:644`, `locallife/api/operator_rules.go:646`, `locallife/api/operator_rules.go:655`, `locallife/api/operator_rules.go:760`, `locallife/api/operator_rules.go:766`, `locallife/api/operator_rules.go:794`, `locallife/api/operator_rules.go:799`, `locallife/api/operator_rules.go:810`, `locallife/db/query/region_rule_config.sql:7`, `locallife/api/rider_operational_status_sync.go:13`, `locallife/db/sqlc/rider_status_helpers.go:123`.

12. Region-expansion page loads existing applications, city options, available regions, and submits a target region through `/v1/operator/region-expansion`.
    Evidence: `weapp/miniprogram/pages/operator/region-expansion/index.ts:38`, `weapp/miniprogram/pages/operator/region-expansion/index.ts:49`, `weapp/miniprogram/pages/operator/region-expansion/index.ts:68`, `weapp/miniprogram/pages/operator/region-expansion/index.ts:84`, `weapp/miniprogram/pages/operator/region-expansion/index.ts:169`, `weapp/miniprogram/pages/operator/_services/operator-region-expansion.ts:50`, `weapp/miniprogram/pages/operator/_services/operator-region-expansion.ts:64`, `weapp/miniprogram/pages/operator/_services/operator-region-expansion.ts:87`, `weapp/miniprogram/pages/operator/_services/operator-region-expansion.ts:125`.

13. Region expansion uses shared region APIs for city/available-region discovery and excludes already occupied or pending/approved regions at SQL level.
    Evidence: `weapp/miniprogram/pages/operator/_main_shared/api/operator-application.ts:226`, `weapp/miniprogram/pages/operator/_main_shared/api/operator-application.ts:237`, `locallife/api/server.go:576`, `locallife/api/server.go:579`, `locallife/api/region.go:177`, `locallife/api/region.go:335`, `locallife/api/region.go:373`, `locallife/api/region.go:405`, `locallife/db/query/region.sql:50`, `locallife/db/query/region.sql:107`.

14. Operator region-expansion submit verifies operator context, region existence, not-already-managed, pending/approved duplicate state, then creates `operator_region_applications`. Rejected old applications can be deleted and resubmitted.
    Evidence: `locallife/api/server.go:1357`, `locallife/api/operator_region_expansion.go:74`, `locallife/api/operator_region_expansion.go:84`, `locallife/api/operator_region_expansion.go:99`, `locallife/api/operator_region_expansion.go:110`, `locallife/api/operator_region_expansion.go:124`, `locallife/api/operator_region_expansion.go:138`, `locallife/api/operator_region_expansion.go:148`, `locallife/db/query/operator_region_application.sql:1`, `locallife/db/query/operator_region_application.sql:13`, `locallife/db/query/operator_region_application.sql:69`.

15. Existing applications are listed by operator and joined to region names for display.
    Evidence: `locallife/api/server.go:1358`, `locallife/api/operator_region_expansion.go:173`, `locallife/api/operator_region_expansion.go:190`, `locallife/api/operator_region_expansion.go:196`, `locallife/db/query/operator_region_application.sql:17`.

16. Platform/admin approval is the write boundary that turns an expansion application into a managed region: approval checks pending state and region availability, then the SQL transaction updates the application and inserts `operator_regions`.
    Evidence: `locallife/api/operator_region_expansion.go:307`, `locallife/api/operator_region_expansion.go:314`, `locallife/api/operator_region_expansion.go:323`, `locallife/api/operator_region_expansion.go:329`, `locallife/api/operator_region_expansion.go:338`, `locallife/db/sqlc/tx_operator_region_application.go:13`, `locallife/db/sqlc/tx_operator_region_application.go:19`, `locallife/db/sqlc/tx_operator_region_application.go:24`, `locallife/db/query/operator_region_application.sql:55`, `locallife/db/query/operator_region.sql:1`.

17. Operator rule-engine proxy routes under `/v1/operators/me/rules` expose rule CRUD/version publish/rollback/disable/hits with region-constrained versions and region-filtered hit reads.
    Evidence: `locallife/api/server.go:1430`, `locallife/api/server.go:1432`, `locallife/api/server.go:1433`, `locallife/api/server.go:1434`, `locallife/api/server.go:1435`, `locallife/api/server.go:1436`, `locallife/api/server.go:1437`, `locallife/api/server.go:1438`, `locallife/api/server.go:1439`, `locallife/api/rules_operator_proxy.go:29`, `locallife/api/rules_operator_proxy.go:86`, `locallife/api/rules_operator_proxy.go:137`, `locallife/api/rules_operator_proxy.go:190`, `locallife/api/rules_operator_proxy.go:281`, `locallife/api/rules_operator_proxy.go:361`, `locallife/api/rules_operator_proxy.go:480`, `locallife/api/rules_operator_proxy.go:548`, `locallife/db/query/rules.sql:3`, `locallife/db/query/rules.sql:8`, `locallife/db/query/rules.sql:19`, `locallife/db/query/rule_hits.sql:14`.

## SQL And Durable State Boundaries

- `operator_regions`: active operator-region authority for selectors and writes.
- `operators`: operator identity and legacy primary region fallback for older region helpers and rule proxy scope.
- `regions`: city/district catalog and available-region response source.
- `operator_region_applications`: operator expansion application state, including pending/approved/rejected and reject reason.
- `delivery_fee_configs`: region-level delivery-fee config written by delivery-fee page and operator rules edits.
- `peak_hour_configs`: region-level peak-hour delivery-fee coefficients written by timeslot page.
- `region_rule_configs`: region-level rider deposit and weather coefficient rules.
- `platform_configs`: platform/global/city fallback rule source for rider deposit and weather coefficients.
- `profit_sharing_configs`: read-only operator commission source shown in rules.
- `weather_coefficients`: latest actual weather coefficient display source; current weather coefficient is read-only to operator.
- `rules`, `rule_versions`, `rule_audits`, `rule_hits`: generic rule-engine proxy state, region-scoped for operator access.
- `riders`: indirect side effect when rider deposit threshold changes and rider operational statuses are reconciled.

## Trust, Authorization, And Tenant Checks

- `/v1/operator/**` and `/v1/operators/me/**` routes require operator role and loaded operator context.
- `/v1/delivery-fee/regions/:region_id/config` writes additionally use `ValidateOperatorRegionMiddleware("region_id")`.
- `/v1/operator/rules` target region is resolved server-side and client-supplied `region_id` is checked with `checkOperatorManagesRegion`.
- Peak-hour create checks request-region authority; delete checks authority after loading the stored config's region.
- Region-expansion submit prevents applying for already managed regions and duplicate pending/approved applications.
- Rule-engine proxy writes constrain rule version scope and gray config to the current operator region; hits are queried by `(rule_id, operator.region_id)`.

## Idempotency And Duplicate-Submit Checks

- Region/fee/peak/rules reads are idempotent.
- Region-expansion duplicate pending/approved applications return conflict. Rejected applications are deleted before resubmission.
- Admin approval transaction is atomic: application approval and `operator_regions` insert succeed or roll back together.
- Delivery-fee page uses PATCH and falls back to POST when update fails in the frontend wrapper; backend still enforces uniqueness and region authorization.
- Peak-hour delete is repeat-safe at API semantics through 404/not-found after the first deletion.
- Rider deposit rule update is not idempotency-keyed, but repeated same-value updates converge to the same region config and rider status reconciliation output.

## Recovery And Async Convergence Paths

- Region list, rules, timeslot, delivery-fee, and expansion pages expose retry/refresh paths and rehydrate from backend state.
- Delivery-fee overview tolerates missing fee config and shows empty summary while still loading peak-hour state.
- Rules page reloads after a successful edit to confirm backend truth.
- Rider deposit threshold changes synchronously trigger region rider status reconciliation; rider-facing payment/refund recovery remains in rider slices.
- Weather coefficient rule changes invalidate weather cache if present; latest weather display continues to come from durable weather coefficient rows.
- Region expansion is eventually visible in managed-region selectors only after platform/admin approval writes `operator_regions`.

## Frontend Draft And Backend Rehydration

- Selected region ids from navigation are UI state only; backend rechecks region authority on every write.
- Rules and delivery-fee pages keep local edit forms, but the final value source is the backend response/reload after save.
- Timeslot overlap checking in the Mini Program is UX assistance only; backend authorization and durable insert/delete remain decisive.
- Region expansion city/region search is a frontend picker over backend available-region truth, not an authority grant.

## Test Coverage Signals

Observed tests:

- `locallife/api/operator_deposit_rules_test.go` covers operator rules list/update, rider deposit region config, weather rules, delivery-fee rules, platform/read-only rules, and region authority paths.
- `locallife/api/delivery_fee_test.go` covers delivery-fee config create/update validation and peak-hour list/create/delete behavior.
- `locallife/api/operator_region_expansion_test.go` covers operator expansion application submit/list and duplicate/availability cases.
- `locallife/api/rules_operator_proxy_test.go` covers operator rule proxy and rule-hit region filtering.
- `locallife/db/sqlc/tx_operator_region_application_test.go` covers approval transaction atomicity and repeated approval failures.
- `locallife/db/sqlc/rider_status_helpers_test.go` covers rider deposit threshold fallback and status reconciliation helpers.

Missing high-value tests:

- Mini Program contract/page test for region list -> region config -> delivery-fee/timeslot/rules navigation with lost or stale `region_id`.
- End-to-end operator rider-deposit edit -> rider status reconciliation -> rider-visible status/deposit requirement update.
- Regression that `/v1/operator/regions/:region_id/peak-hours` list rejects unmanaged regions if that is intended; current list path reads by region id without an explicit operator-region check.
- Contract test for region-expansion page city search fallback and backend available-region filtering with pending region applications.
- Coverage proving the `/v1/operators/me/rules/**` proxy is either wired to a current operator UI or intentionally API-only.

## Gaps And Refactor Notes

- The region-expansion page can submit applications, but approval and rejection are platform/admin-owned; operator-side closure is status visibility, not self-service activation.
- `listPeakHourConfigs` currently reads by `region_id` under the operator route group but does not call `checkOperatorManagesRegion`; write/delete paths do check region authority. Decide whether this read should match the write boundary.
- The frontend delivery-fee wrapper catches any PATCH failure and tries POST. That is convenient for missing config but can mask non-404 failures until POST returns its own error.
- Operator rules update still writes `operators.rider_deposit` alongside `region_rule_configs`; region config is the effective source for current rider status logic.
- Rule-engine proxy routes exist under `/v1/operators/me/rules/**`, but the current Mini Program operator rules page uses `/v1/operator/rules` instead. Treat proxy coverage as API-only until a UI path is confirmed.

## Branch Exhaustion

- Entry branches checked: region list normal/target rules, region config missing id, delivery-fee missing id/retry/save, rules missing region redirect/category/edit/read-only/navigate peak/save, timeslot missing region/retry/add/delete, region expansion list/city picker/search/select/submit.
- Request branches checked: `/v1/operator/regions`, `/v1/delivery-fee/regions/:region_id/config` GET/PATCH/POST, `/v1/operator/regions/:region_id/peak-hours` GET/POST, `/v1/operator/peak-hours/:id` DELETE, `/v1/operator/rules` GET, `/v1/operator/rules/:key` PATCH, `/v1/regions`, `/v1/regions/available`, `/v1/operator/region-expansion` GET/POST, `/v1/operators/me/rules/**`.
- Backend state branches checked: no operator context, unmanaged region, missing fee config, invalid fee/min/max, duplicate fee config, invalid peak time/coefficient/days, read-only weather coefficient, platform-only rule, region rule fallback, weather cache invalidation, duplicate/pending/approved/rejected expansion applications, admin approval/rejection boundary.
- Async/convergence branches checked: rider status reconciliation after rider deposit rule update, weather cache invalidation, platform approval transaction, frontend reload after save.
- Dead/orphan branches checked: current Mini Program uses `/v1/operator/rules`; `/v1/operators/me/rules/**` is documented as active backend API/proxy surface but no direct operator Mini Program page caller was found in this slice.
