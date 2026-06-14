# Operator Flow Variant Index

Status: created 2026-06-14
Purpose: cross-slice branch, drift, no-entry, and legacy-path checklist for the operator-side codegraph. Detailed evidence remains in the individual `*.slice.md` files.
Boundary note: this index is scoped to operator-visible pages, operator APIs, and operator-adjacent background paths. It does not replace merchant, rider, platform, customer, provider, or payment-domain source slices.

## How To Use This Index

Use this file as the handoff queue after reading the relevant slice:

- `Contract drift`: a frontend type, parameter, copy, or service promise does not match current backend behavior.
- `Authority asymmetry`: two operator pages use different region-selection semantics.
- `No page caller`: backend/API path exists and is reviewed as a boundary, but no current operator Mini Program entry invokes it.
- `Legacy candidate`: code or SQL remains present but is not the current operator path.
- `Operational closure`: the operator can see state but does not own the write action or real-world action loop.

For implementation, start from the owning slice and source evidence instead of changing code from this index alone.

## Region Authority Variants

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| OP-AUTH-001 | Authority asymmetry | Dashboard analytics and finance overview use managed-region/all-region selection, while several older pages still rely on single-region fallback. | Multi-region operators can get a complete dashboard/overview but narrower or failing secondary pages. | Decide a single product rule for multi-region operator pages: explicit region picker, aggregate all regions, or single-region only with clear UX. | `operator-dashboard-analytics-notifications.slice.md`, `operator-finance-and-baofu-withdrawal.slice.md` |
| OP-AUTH-002 | Authority asymmetry | Merchant list aggregates all managed regions when no `region_id` is supplied. Rider list defaults missing `region_id` to `operator.region_id`. | Merchant and rider management feel similar in UI but do not have the same backend scope. | Align rider list with merchant list or make the primary-region-only behavior explicit in page UX and API docs. | `operator-merchant-management.slice.md`, `operator-rider-management.slice.md` |
| OP-AUTH-003 | Authority asymmetry | Safety food-safety APIs use `getOperatorRegionID`; safety Mini Program does not pass `region_id` or expose a region selector. | Multi-region operators without a primary/single fallback may be blocked from safety case list/detail/write. | Add managed-region selector to safety pages or move safety APIs to the multi-region selection pattern. | `operator-safety-and-recovery.slice.md` |
| OP-AUTH-004 | Authority asymmetry | Finance overview aggregates all managed regions, but commission bills and profit-sharing configs use `getOperatorRegionID`; current overview/bills pages do not pass `region_id`. | Operators can see aggregate income summary but fail or get a different region for bill/config detail. | Add region selection to finance pages or make commission/config aggregate consistently with overview. | `operator-finance-and-baofu-withdrawal.slice.md` |
| OP-AUTH-005 | Trust-boundary gap | `listPeakHourConfigs` reads by path `region_id` under operator routes but does not explicitly call `checkOperatorManagesRegion`; create/delete do check region authority. | Peak-hour read permission is weaker than write/delete for the same feature. | Add explicit region authority check to the list handler or document why the route group is sufficient. | `operator-region-rules-and-expansion.slice.md` |
| OP-AUTH-006 | Contract drift | `listOperatorRecoveryDisputesRequest.RegionID` is bound and Swagger documents `region_id`, but handlers use `ctx.Query("region_id")` through `resolveOperatorRegionSelection`. | Request struct field is effectively unused and can mislead future maintainers/tests. | Remove the unused bound field or route the handler through the bound request value consistently. | `operator-safety-and-recovery.slice.md` |

## Frontend Promise Versus Backend Support

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| OP-CONTRACT-001 | Contract drift | Operator merchant list sends `keyword`, but backend request/SQL do not bind or filter by keyword. | Search UI is local/promise drift unless data already happened to be filtered elsewhere. | Either add backend keyword filtering or remove the API/UI promise. | `operator-merchant-management.slice.md` |
| OP-CONTRACT-002 | Contract drift | `getMerchantSummary(regionId?: number)` sends `region_id`, but backend summary aggregates via server-resolved managed regions. | Region-scoped merchant summary parameter is ignored. | Implement scoped summary or remove the parameter from frontend service. | `operator-merchant-management.slice.md` |
| OP-CONTRACT-003 | Stale type | `SuspendOperatorMerchantRequest` and `ResumeOperatorMerchantRequest` exist in frontend API types, but no page action, service method, or backend route uses them. | Future work may assume merchant suspend/resume is an operator capability when it is not. | Delete stale types or keep them only behind a real route/page design. | `operator-merchant-management.slice.md` |
| OP-CONTRACT-004 | Contract drift | Operator rider list sends `keyword`; API type includes `online_status`; backend request binds neither field. | Search and online filtering are not backend-supported despite frontend API shape. | Add backend filtering or remove/rename unsupported params. | `operator-rider-management.slice.md` |
| OP-CONTRACT-005 | Contract drift | `getRiderSummary(regionId?: number)` sends `region_id`, but backend summary aggregates via server-resolved managed regions. | Region-scoped rider summary parameter is ignored. | Implement scoped summary or remove the parameter from frontend service. | `operator-rider-management.slice.md` |
| OP-CONTRACT-006 | Contract drift | Rider API type includes `offline` as a status, but backend status binding accepts only approval/active/suspended style statuses. | `offline` is an online-state display concept, not a valid rider status filter. | Split rider lifecycle status from online status in frontend types and page filters. | `operator-rider-management.slice.md` |
| OP-CONTRACT-007 | Stale type | `SuspendOperatorRiderRequest` and `ResumeOperatorRiderRequest` exist in frontend API types, but no operator page action or backend route uses them. | Future work may assume rider pause/resume is an operator capability when it is not. | Delete stale types or add a real rule/platform-owned action path. | `operator-rider-management.slice.md` |
| OP-CONTRACT-008 | Contract drift | Mini Program `OperatorRegionStatsResponse` and analytics service describe richer nested region stats than backend `regionStatsResponse` currently returns. | Analytics code tolerates missing data with fallback summary, but type/comments overpromise. | Align frontend type/comment with backend response or expand backend response deliberately. | `operator-dashboard-analytics-notifications.slice.md` |
| OP-CONTRACT-009 | UX promise drift | Dispatch timeout notification copy says the operator should remind the rider, but dispatch hall has no contact/action channel. | UI copy implies an action loop that the product does not currently provide. | Add explicit contact/escalation action or adjust copy to the real read-only behavior. | `operator-dispatch-hall.slice.md` |
| OP-CONTRACT-010 | Error-path drift | Delivery-fee wrapper catches any PATCH failure and tries POST. | Missing-config convenience can mask non-404 PATCH failures until POST returns a second error. | Retry POST only for a verified not-found/missing-config branch. | `operator-region-rules-and-expansion.slice.md` |

## Backend API-Only Or No-Page-Caller Surfaces

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| OP-NOUI-001 | No page caller | Operator recovery-dispute list/summary/detail and claim-recovery detail routes are registered and reviewed, but no current operator Mini Program page/API caller was found. | These are API visibility boundaries, not live operator UI closure. | Keep read-only boundary in docs; do not add review/waive/payment claims without UI and route design. | `operator-safety-and-recovery.slice.md` |
| OP-NOUI-002 | No page caller | Operator recovery-dispute routes are GET-only; removed review/waive routes are tested as absent. | Operator cannot mutate recovery disputes through current UI/API. | Treat review/waive/payment as non-entry until a real capability lands. | `operator-safety-and-recovery.slice.md` |
| OP-NOUI-003 | No page caller | `/v1/operators/me/profit-sharing/configs` exists and has tests, but no current operator Mini Program page/service caller was found. | It is backend/API surface, not current page capability. | Either wire it to a real finance/rules page or mark it backend-only in API docs. | `operator-finance-and-baofu-withdrawal.slice.md` |
| OP-NOUI-004 | No page caller | Rule-engine proxy routes under `/v1/operators/me/rules/**` exist, while current Mini Program rules page uses `/v1/operator/rules`. | Proxy coverage is API-only for current operator Mini Program. | Keep proxy slice as backend surface or migrate UI intentionally. | `operator-region-rules-and-expansion.slice.md` |

## Legacy Or Non-Current Paths

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| OP-LEGACY-001 | Legacy candidate | `operator_finance.sql` still defines generic `withdrawal_records` queries, but current operator Mini Program Baofu withdrawal uses `baofu_withdrawal_orders` plus external payment command/fact tables. | Old table/query names can be mistaken for current operator cash-out truth. | Confirm remaining call sites. If none are current, move to dead-code candidate cleanup after owner approval. | `operator-finance-and-baofu-withdrawal.slice.md` |
| OP-LEGACY-002 | Dual source | Operator rules update still writes `operators.rider_deposit` alongside `region_rule_configs`; current rider status logic uses region config as effective source. | Future readers may treat `operators.rider_deposit` as authoritative. | Document compatibility purpose or remove duplicate write after downstream reads are audited. | `operator-region-rules-and-expansion.slice.md` |

## Operational Closure Boundaries

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| OP-CLOSURE-001 | Operational closure | Dispatch hall is intentionally read-only; contact, cancellation, refund, and manual intervention are outside this slice. | Operator sees pending/timeout state but does not have a recorded action loop. | Design a separate dispatch escalation/action slice if operators should intervene. | `operator-dispatch-hall.slice.md` |
| OP-CLOSURE-002 | Operational closure | Merchant management does not own merchant suspension/recovery/refund/onboarding closure. | Merchant list/detail can show state but not resolve those domains. | Keep closures in food-safety, recovery, admin, and merchant-onboarding slices. | `operator-merchant-management.slice.md` |
| OP-CLOSURE-003 | Operational closure | Rider management does not own deposit payment, deposit withdrawal, online eligibility, or platform rider approval. | Rider list/detail are operational visibility, not rule/payment closure. | Keep closures in rider deposit, rider income/Baofu, rider workbench, and platform review slices. | `operator-rider-management.slice.md` |
| OP-CLOSURE-004 | Operational closure | Region expansion can be submitted by operator, but approval/rejection is platform/admin-owned. | Operator-side closure is status visibility and submit idempotency, not activation. | Keep activation under platform/admin region-application workflow. | `operator-region-rules-and-expansion.slice.md` |
| OP-CLOSURE-005 | Operational closure | Food-safety investigation save is direct conditional SQL update; final resolution is transactional. | Current field update is acceptable, but future audit/notification requirements need a stronger boundary. | If audit/notification is added, wrap investigation save in a transaction/outbox boundary. | `operator-safety-and-recovery.slice.md` |

## Provider And Finance Risk Boundaries

| ID | Type | Finding | Current impact | Suggested next action | Source |
| --- | --- | --- | --- | --- | --- |
| OP-RISK-001 | Provider evidence gap | Baofu withdrawal has parser, unit, worker, callback, and recovery coverage, but no real funds-action C4 evidence. | Do not claim end-to-end provider withdrawal success from local tests alone. | Only claim C4 after explicit funds-action approval, funding, masked callback/query evidence, and evidence-ledger update. | `operator-finance-and-baofu-withdrawal.slice.md` |
| OP-RISK-002 | Finance semantic boundary | Dashboard finance summary and finance overview are reads over finished profit-sharing stats, not Baofu withdrawal balance. | Reusing summary values as cash-out availability would be a money-state bug. | Keep withdrawal availability sourced from Baofu balance/account binding path. | `operator-dashboard-analytics-notifications.slice.md`, `operator-finance-and-baofu-withdrawal.slice.md` |
| OP-RISK-003 | Notification category boundary | Operator notification category filtering depends on `extra_data.category`; new categories need backend allow-list plus frontend tabs/copy. | Adding categories only on one side can hide or mislabel notifications. | Update backend notification category production and frontend filters together. | `operator-dashboard-analytics-notifications.slice.md` |
| OP-RISK-004 | Alert dedupe boundary | `delivery_timeout_alerts` dedupes the 3-minute timeout key only. | Later timeout milestones would collide if they reuse the same key shape. | Use milestone-specific dedupe keys for any new timeout alert stages. | `operator-dispatch-hall.slice.md` |

## Resolved Or Explicit Non-Issues

- No dead dashboard, analytics, notification, or dispatch-hall route was found in those slices.
- Merchant/rider suspend and resume are explicitly non-entry for current operator pages, not missing route wiring.
- Baofu withdrawal request handler intentionally does not call the provider directly; async command dispatch is the current production path.
- Operator recovery-dispute/recovery APIs are intentionally documented as backend visibility boundaries until a Mini Program page exists.
