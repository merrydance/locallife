# Operator Safety And Recovery Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G3 boundary - operator food-safety resolution can release merchant/takeout suspension and paused orders; recovery dispute and claim-recovery reads expose claim, order, merchant, user, rider, compensation, and regional authority state
Scope: Mini Program operator safety pages -> food-safety case APIs -> food-safety transaction -> merchant/order state recovery; operator recovery-dispute and claim-recovery API boundary -> automatic recovery dispute/recovery post-process boundary

## Variant Coverage

This slice covers:

- Operator food-safety case list page, status tabs, pagination, refresh, retry, and detail navigation.
- Operator food-safety detail page, investigation-report save, resolution submit, resolved/read-only state, and form validation.
- Frontend service/API wrappers for `/v1/operator/food-safety/cases`, `/:id`, `/:id/investigate`, and `/:id/resolve`.
- Backend food-safety handlers, region checks, SQL reads/writes, and `ResolveFoodSafetyCaseTx`.
- The customer-report boundary that creates food-safety incidents/cases and suspends merchant/takeout state before an operator sees the case.
- Operator recovery-dispute list/summary/detail APIs and operator claim-recovery detail API.
- Recovery-dispute automatic review and claim-recovery post-process as a backend boundary, because operator routes expose status/detail but do not provide a live review/write route.

This slice does not fully cover:

- Customer food-safety report submission UX and notification fanout. This slice references that flow only as the source of `food_safety_cases`.
- Merchant and rider claim/recovery pages, payment, deposits, compensation payout, refund, and provider callback closure.
- Platform/admin manual recovery-dispute review if it is reintroduced later.
- Operator dashboard recovery/safety cards if future pages consume the recovery APIs; no current operator Mini Program page caller was found for recovery-dispute or recovery detail APIs.

## Product Invariant

Operator safety is a real state-transition surface only for food-safety case handling:

- A food-safety case is created by customer reports and circuit-break logic, not by the operator page.
- The operator can read cases, save an investigation report, and resolve a case for the operator's authorized region.
- Resolution requires an investigation report plus merchant rectification and resolution text, then atomically resolves the case/incidents and releases only food-safety-owned merchant/takeout/order pause state.
- A resolved case is read-only to the Mini Program and rejected by backend duplicate-write guards.
- Operator recovery-dispute and claim-recovery APIs are visibility boundaries: current route registration has no operator review/waive/payment route. Recovery dispute mutation happens when merchant/rider creates a dispute and automatic backend review/post-process runs.

## Primary Forward Chain

1. Operator Mini Program declares the safety case list and detail pages.
   Evidence: `weapp/miniprogram/app.json:50`, `weapp/miniprogram/app.json:51`.

2. Safety list initializes list state, loads on entry/show, supports pull refresh, tab status filtering, load-more, and detail navigation.
   Evidence: `weapp/miniprogram/pages/operator/safety/report/index.ts:12`, `weapp/miniprogram/pages/operator/safety/report/index.ts:26`, `weapp/miniprogram/pages/operator/safety/report/index.ts:30`, `weapp/miniprogram/pages/operator/safety/report/index.ts:40`, `weapp/miniprogram/pages/operator/safety/report/index.ts:46`, `weapp/miniprogram/pages/operator/safety/report/index.ts:76`, `weapp/miniprogram/pages/operator/safety/report/index.ts:81`, `weapp/miniprogram/pages/operator/safety/report/index.ts:86`.

3. Safety detail validates `id`, loads case/incidents, and blocks investigation/resolution submission when the adapted case is no longer active.
   Evidence: `weapp/miniprogram/pages/operator/safety/detail/index.ts:14`, `weapp/miniprogram/pages/operator/safety/detail/index.ts:30`, `weapp/miniprogram/pages/operator/safety/detail/index.ts:44`, `weapp/miniprogram/pages/operator/safety/detail/index.ts:75`, `weapp/miniprogram/pages/operator/safety/detail/index.ts:102`.

4. Frontend safety service adapts backend case statuses into labels/themes/active flags and sends list/detail/investigate/resolve calls through `operatorBasicManagementService`.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-safety.ts:11`, `weapp/miniprogram/pages/operator/_services/operator-safety.ts:62`, `weapp/miniprogram/pages/operator/_services/operator-safety.ts:78`, `weapp/miniprogram/pages/operator/_services/operator-safety.ts:102`, `weapp/miniprogram/pages/operator/_services/operator-safety.ts:120`, `weapp/miniprogram/pages/operator/_services/operator-safety.ts:125`, `weapp/miniprogram/pages/operator/_services/operator-safety.ts:131`.

5. Frontend API wrapper declares the supported status enum and calls only the four food-safety routes; no operator recovery-dispute wrapper or page caller was found under `weapp/miniprogram/pages/operator`.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:117`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:291`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:315`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:322`, `weapp/miniprogram/pages/operator/_api/operator-basic-management.ts:330`.

6. Backend route group registers food-safety read/write routes and recovery-dispute/recovery read routes under `/v1/operator`.
   Evidence: `locallife/api/server.go:1396`, `locallife/api/server.go:1397`, `locallife/api/server.go:1398`, `locallife/api/server.go:1399`, `locallife/api/server.go:1401`, `locallife/api/server.go:1402`, `locallife/api/server.go:1403`, `locallife/api/server.go:1404`.

7. Food-safety list binds page/limit/status, resolves one operator region through `getOperatorRegionID`, and reads/counts region cases with optional status.
   Evidence: `locallife/api/operator_food_safety_cases.go:15`, `locallife/api/operator_food_safety_cases.go:153`, `locallife/api/operator_food_safety_cases.go:167`, `locallife/api/operator_food_safety_cases.go:173`, `locallife/api/operator_food_safety_cases.go:175`, `locallife/api/operator_food_safety_cases.go:185`, `locallife/api/operator_food_safety_cases.go:200`, `locallife/api/operator_food_safety_cases.go:211`.

8. Food-safety detail parses case id, resolves operator region, loads the case, rejects cross-region access, then reads incidents linked to the case.
   Evidence: `locallife/api/operator_food_safety_cases.go:241`, `locallife/api/operator_food_safety_cases.go:242`, `locallife/api/operator_food_safety_cases.go:248`, `locallife/api/operator_food_safety_cases.go:254`, `locallife/api/operator_food_safety_cases.go:263`, `locallife/api/operator_food_safety_cases.go:268`.

9. Investigation save checks region, rejects resolved cases, trims the report, and conditionally updates the case to `investigating`.
   Evidence: `locallife/api/operator_food_safety_cases.go:296`, `locallife/api/operator_food_safety_cases.go:309`, `locallife/api/operator_food_safety_cases.go:315`, `locallife/api/operator_food_safety_cases.go:324`, `locallife/api/operator_food_safety_cases.go:328`, `locallife/api/operator_food_safety_cases.go:333`, `locallife/api/operator_food_safety_cases.go:339`, `locallife/db/query/trust_score.sql:414`.

10. Resolution save checks region/resolved state, requires an effective investigation report and merchant resolution fields, then calls `ResolveFoodSafetyCaseTx`.
    Evidence: `locallife/api/operator_food_safety_cases.go:369`, `locallife/api/operator_food_safety_cases.go:382`, `locallife/api/operator_food_safety_cases.go:388`, `locallife/api/operator_food_safety_cases.go:397`, `locallife/api/operator_food_safety_cases.go:401`, `locallife/api/operator_food_safety_cases.go:406`, `locallife/api/operator_food_safety_cases.go:413`, `locallife/api/operator_food_safety_cases.go:426`.

11. `ResolveFoodSafetyCaseTx` locks the case, rechecks region/resolved/investigation requirements, resolves the case and incidents, loads merchant/profile state, releases merchant and takeout suspension only when the stored reason is food-safety owned, clears food-safety paused takeout orders, and reactivates merchant status only if the food-safety suspension was released.
    Evidence: `locallife/db/sqlc/tx_food_safety.go:238`, `locallife/db/sqlc/tx_food_safety.go:242`, `locallife/db/sqlc/tx_food_safety.go:246`, `locallife/db/sqlc/tx_food_safety.go:249`, `locallife/db/sqlc/tx_food_safety.go:253`, `locallife/db/sqlc/tx_food_safety.go:261`, `locallife/db/sqlc/tx_food_safety.go:282`, `locallife/db/sqlc/tx_food_safety.go:297`, `locallife/db/sqlc/tx_food_safety.go:302`, `locallife/db/sqlc/tx_food_safety.go:307`, `locallife/db/sqlc/tx_food_safety.go:315`, `locallife/db/sqlc/tx_food_safety.go:320`, `locallife/db/sqlc/tx_food_safety.go:325`, `locallife/db/sqlc/tx_food_safety.go:385`.

12. Customer report flow is the upstream case source: it deduplicates open incidents, creates/reuses an open case on circuit break, links incidents, suspends merchant/profile/takeout state, pauses active takeout orders, and writes order status logs.
    Evidence: `locallife/db/sqlc/tx_food_safety.go:57`, `locallife/db/sqlc/tx_food_safety.go:65`, `locallife/db/sqlc/tx_food_safety.go:87`, `locallife/db/sqlc/tx_food_safety.go:123`, `locallife/db/sqlc/tx_food_safety.go:130`, `locallife/db/sqlc/tx_food_safety.go:159`, `locallife/db/sqlc/tx_food_safety.go:168`, `locallife/db/sqlc/tx_food_safety.go:179`, `locallife/db/sqlc/tx_food_safety.go:190`, `locallife/db/sqlc/tx_food_safety.go:206`, `locallife/db/sqlc/tx_food_safety.go:212`, `locallife/db/sqlc/tx_food_safety.go:221`.

13. Operator recovery-dispute list and summary resolve server-side managed regions and read counts/details from `recovery_disputes`, claims, orders, merchants, riders/users.
    Evidence: `locallife/api/recovery_dispute.go:1381`, `locallife/api/recovery_dispute.go:1404`, `locallife/api/recovery_dispute.go:1411`, `locallife/api/recovery_dispute.go:1431`, `locallife/api/recovery_dispute.go:1436`, `locallife/api/recovery_dispute.go:1448`, `locallife/api/recovery_dispute.go:1459`, `locallife/api/recovery_dispute.go:1477`, `locallife/api/recovery_dispute.go:1488`, `locallife/api/recovery_dispute.go:1539`, `locallife/api/recovery_dispute.go:1546`, `locallife/api/recovery_dispute.go:1552`.

14. Operator recovery-dispute detail loads the dispute, checks the dispute's stored region with `checkOperatorManagesRegion`, then reads operator detail by dispute id and region id.
    Evidence: `locallife/api/recovery_dispute.go:1611`, `locallife/api/recovery_dispute.go:1618`, `locallife/api/recovery_dispute.go:1627`, `locallife/api/recovery_dispute.go:1632`, `locallife/db/query/recovery_dispute.sql:410`.

15. Operator claim-recovery detail lists all managed operator region ids, then authorizes the recovery by claim recovery context before returning read-only recovery state.
    Evidence: `locallife/api/claim_recovery.go:195`, `locallife/api/claim_recovery.go:202`, `locallife/api/claim_recovery.go:208`, `locallife/logic/claim_recovery.go:68`, `locallife/logic/claim_recovery.go:74`, `locallife/logic/claim_recovery_context.go:11`, `locallife/db/query/claim_recovery.sql:117`.

16. Recovery-dispute write/post-process is not an operator UI route in current code. Merchant/rider dispute creation marks claim recovery disputed and invokes automatic resolution; automatic resolution updates review state and may waive/resume recovery, create release/compensation actions, enqueue retries, and process downstream effects.
    Evidence: `locallife/api/recovery_dispute.go:712`, `locallife/api/recovery_dispute.go:1219`, `locallife/api/recovery_dispute.go:1767`, `locallife/api/recovery_dispute.go:1785`, `locallife/db/sqlc/tx_create_recovery_dispute.go:23`, `locallife/db/sqlc/tx_create_recovery_dispute.go:54`, `locallife/logic/recovery_dispute_auto_resolution.go:35`, `locallife/db/sqlc/tx_recovery_dispute_review.go:27`, `locallife/db/sqlc/tx_recovery_dispute_review.go:51`, `locallife/db/sqlc/tx_recovery_dispute_review.go:77`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:52`, `locallife/worker/task_process_recovery_dispute_result.go:104`.

## SQL And Durable State Boundaries

- `food_safety_cases`: operator-visible case state; list/detail/investigate/resolve read or mutate this table.
- `food_safety_incidents`: customer report incidents linked to cases and resolved with the case.
- `merchant_profiles`: food-safety suspension/takeout suspension state; resolution releases only food-safety-owned reasons.
- `merchants`: merchant active/suspended projection; resolution returns to `active` only when the released suspension belongs to food safety.
- `orders`: active takeout orders get `food_safety_paused`; resolution clears only rows matching that exception state and active takeout statuses.
- `order_status_logs`: food-safety pause logs written by the upstream report/circuit-break transaction.
- `operator_regions`, `operators`, `regions`: server-side regional authority for food-safety and recovery visibility.
- `recovery_disputes`: operator-visible dispute list/detail/status source; current operator routes do not mutate it.
- `claim_recoveries`, `claim_recovery_events`: recovery payment/release lifecycle; operator can read recoveries by region, while merchant/rider/automatic recovery paths mutate them.
- `claims`, `deliveries`, `behavior_decisions`, `behavior_actions`, `behavior_blocklist`, `user_claim_warnings`: recovery-dispute post-process inputs/effects outside the safety page.

## Trust, Authorization, And Tenant Checks

- Food-safety and recovery routes are in the operator route group protected by operator role and loaded operator context.
- Food-safety list/detail/write uses `getOperatorRegionID`, so a query `region_id` must be parseable and managed; otherwise the helper falls back to primary/single assigned region or fails closed.
- The current Mini Program safety list does not send `region_id`, so multi-region operators without a usable primary/single region cannot use the page without backend returning a region-required/forbidden error.
- Food-safety detail, investigation, and resolution re-read the stored case and compare `caseRecord.RegionID` to the server-resolved region before exposing or mutating it.
- Recovery-dispute detail first loads `recovery_disputes.region_id` and checks operator-region authority before reading joined sensitive detail.
- Claim-recovery detail checks the recovery's claim/order merchant region against server-resolved managed region ids.
- Recovery-dispute detail response includes merchant phone, user phone/name, rider id, claim/order status, and lookback result; this is intentionally a G3 visibility boundary.

## Idempotency And Duplicate-Submit Checks

- Food-safety list/detail are idempotent reads.
- Investigation update is repeatable while the case is not resolved; SQL rejects a concurrent resolved update by returning no row.
- Resolution is guarded in handler and transaction: already resolved cases return a stable bad request and the transaction locks the case before changing state.
- Customer report source flow deduplicates open incidents by order/user and handles unique constraints for open case/incident races.
- Recovery-dispute creation uses uniqueness/existence checks and can reuse existing rider disputes; automatic resolution can be retried by worker.
- Operator recovery APIs in this slice are read-only, so duplicate operator requests do not mutate dispute/recovery state.

## Recovery And Async Convergence Paths

- Food-safety list refreshes on entry/show, pull refresh, status tab changes, and load more; current state rehydrates from SQL.
- Detail reloads after investigation save and resolution submit, so the page reflects backend truth after each write.
- Resolution restores only food-safety-owned suspension state; non-food-safety manual/compliance suspensions remain intact.
- Recovery-dispute automatic resolution logs failures and enqueues `recovery_dispute:automatic_resolution` retry when a task distributor exists.
- Recovery dispute result processing executes claim recovery release, claimant penalty, compensation action, resume-after-rejection, and notifications as downstream effects; those are not operator page actions.
- Claim recovery payment/provider callback and direct payment fact recovery are outside this slice and should remain owned by the claim-recovery/payment slices.

## Frontend Draft And Backend Rehydration

- Food-safety form text is local draft until submitted; backend trims and validates the effective fields again.
- Detail `is_active`/`is_resolved` are frontend projections from backend status, not permission truth.
- Resolve can reuse the persisted investigation report when the local investigation field is blank.
- The frontend status enum matches backend binding: `merchant-suspended`, `investigating`, and `resolved`.
- There is no frontend recovery-dispute/recovery service under the current operator pages; backend recovery APIs are documented here as API-level boundaries, not page-driven UI.

## Test Coverage Signals

Observed tests:

- `locallife/api/operator_food_safety_cases_test.go` covers list region usage, resolution transaction call, missing investigation rejection, cross-region denial for detail/investigate/resolve, resolved investigation race, and duplicate resolved rejection.
- `locallife/integration/food_safety_case_integration_test.go` covers the customer-report -> circuit-break -> operator list/detail -> investigate -> resolve closed loop, incident resolution, merchant/profile recovery, duplicate resolve rejection, and the guard that non-food-safety suspension is not cleared.
- `locallife/api/casbin_enforcer_test.go` covers production Casbin policy entries for operator food-safety list/detail/investigate/resolve.
- `locallife/api/recovery_dispute_test.go` covers removed operator review/waive routes, operator recovery-dispute list, summary, and detail API behavior.
- `locallife/db/sqlc/recovery_dispute_tx_test.go` covers operator recovery-dispute list/count/detail SQL and wrong-region detail denial.
- `locallife/worker/task_automatic_recovery_dispute_resolution_test.go` and `locallife/worker/task_process_recovery_dispute_result_test.go` cover automatic recovery dispute resolution and post-process recovery effects.

Missing high-value tests:

- Mini Program page/service tests for food-safety list/detail re-entry, duplicate-submit button disabling, resolved read-only rendering, and region-required failure messaging.
- API test for explicit `region_id` query on food-safety list/detail/write, including multi-region operator behavior.
- API test proving food-safety list status filter uses `ListFoodSafetyCasesByRegionAndStatus` and rejects unsupported statuses.
- API or integration test for operator `GET /v1/operator/recoveries/:id` success/forbidden/not-found.
- Contract test proving no operator recovery-dispute review/waive route exists and no Mini Program page accidentally promises that capability.
- End-to-end test that automatic recovery dispute retry remains idempotent across retry after partial post-process completion.

## Gaps And Refactor Notes

- The safety Mini Program page has no region selector and does not pass `region_id`; backend food-safety helpers can require a region when an operator has multiple regions and no valid primary/single fallback.
- Food-safety operator APIs use single-region `getOperatorRegionID`, while recovery-dispute APIs use multi-region `resolveOperatorRegionSelection`/`listManagedOperatorRegionIDs`. Decide whether safety should adopt the managed-region picker pattern used by dashboard/region slices.
- `listOperatorRecoveryDisputesRequest` has `RegionID *int64` and Swagger documents `region_id`, but the handlers read the actual region from `ctx.Query("region_id")` through `resolveOperatorRegionSelection`; the bound `req.RegionID` is effectively unused.
- Operator recovery-dispute routes are currently GET-only. The durable artifact should keep treating review/waive/payment as non-entry until a real route and UI capability are added.
- Food-safety investigation save is a direct conditional SQL update, not the same transaction as final resolution. This is acceptable for the current field update, but audit/notification requirements would need a dedicated transaction or outbox boundary if added.
- Recovery-dispute detail exposes sensitive user/merchant/order data; any future Mini Program page must design masking and least-privilege display deliberately rather than mirroring the DTO.

## Branch Exhaustion

- Entry branches checked: safety list open/show, pull refresh, status tab switch, load more, detail navigation, invalid detail id, detail retry, investigation save, resolution submit, resolved read-only guard.
- Request branches checked: food-safety list/detail/investigate/resolve, recovery-dispute list/summary/detail, claim-recovery detail.
- Backend state branches checked: missing/invalid region, unmanaged region, cross-region case/dispute/recovery, not-found case/dispute/recovery, unsupported status, resolved case, missing investigation, blank resolution fields, concurrent resolved investigation update.
- Durable-state branches checked: case and incident resolution, food-safety-owned merchant/profile/takeout release, active paused order clear, non-food-safety suspension preservation, recovery disputed/waived/resumed, release/compensation action creation.
- Dead/orphan branches checked: no operator Mini Program recovery-dispute page/API caller found; removed operator review/waive routes are tested as 404; operator recovery APIs remain backend-only visibility boundaries in current code.
