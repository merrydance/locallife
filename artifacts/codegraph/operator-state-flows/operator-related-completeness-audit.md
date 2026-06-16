# Operator Related Completeness Audit

Status: created 2026-06-14
Purpose: answer whether the operator codegraph artifacts exhaust operator-facing and operator-related flows, including Mini Program entrypoints, backend/API-only boundaries, background workers, and known stale or legacy paths.

## Verdict

The seven operator slices exhaust the current operator-facing and operator-actionable closure as of 2026-06-14: every operator Mini Program entrypoint declared in `weapp/miniprogram/app.json:36` through `weapp/miniprogram/app.json:59` has direct `frontend_page` coverage in an operator `*.edges.json`, and each page's backend route, main logic, SQL truth, async worker/scheduler, provider callback, or explicit non-entry boundary is represented in a reviewed slice.

The stricter phrase "all operator-related flows" is broader than operator-side closure. Under that definition, customer food-safety reports, merchant/rider lifecycle mutations, platform/admin approvals, provider callbacks, payment facts, and background schedulers must also be counted. This audit indexes those touchpoints so they are not silently omitted, while keeping their operational closure owned by the relevant role/domain artifacts.

Remaining issues are implementation or product-alignment backlog, not missing codegraph coverage. They are tracked in `flow-variant-index.md`.

## Operator-Facing Closure Covered

1. Dashboard, analytics, managed region picker, finance summary, notification center, and dispatch handoff: `operator-dashboard-analytics-notifications.slice.md`.
2. Dispatch hall, pending delivery pool reads, 3-minute timeout alert scheduler/worker, and notification handoff: `operator-dispatch-hall.slice.md`.
3. Region list/config, delivery fee, peak hours, rules, rule-engine proxy boundary, and region expansion: `operator-region-rules-and-expansion.slice.md`.
4. Merchant list/detail/summary/stats/capabilities and merchant-management non-entry boundaries: `operator-merchant-management.slice.md`.
5. Rider list/detail/summary/stats and rider-management non-entry boundaries: `operator-rider-management.slice.md`.
6. Food-safety investigation/resolution, merchant/order release transaction, recovery-dispute/recovery read boundary, and automatic recovery-dispute post-process: `operator-safety-and-recovery.slice.md`.
7. Finance overview, commission bills, Baofu settlement account, Baofu withdrawal, callbacks, command/fact workers, recovery schedulers, and legacy withdrawal boundary: `operator-finance-and-baofu-withdrawal.slice.md`.

## Mini Program Entrypoint Matrix

| app.json entry | Primary slice | Edge node | Verdict |
| --- | --- | --- | --- |
| `pages/operator/analytics/index` | `operator-dashboard-analytics-notifications` | `weapp.analytics` | Covered |
| `pages/operator/dashboard/index` | `operator-dashboard-analytics-notifications` | `weapp.dashboard` | Covered |
| `pages/operator/dispatch-hall/index` | `operator-dispatch-hall` | `weapp.dispatchHall` | Covered |
| `pages/operator/notifications/index` | `operator-dashboard-analytics-notifications` | `weapp.notifications` | Covered |
| `pages/operator/notifications/detail/index` | `operator-dashboard-analytics-notifications` | `weapp.notificationDetail` | Covered; dispatch handoff is also modeled in `operator-dispatch-hall` |
| `pages/operator/merchants/index` | `operator-merchant-management` | `weapp.merchantList` | Covered |
| `pages/operator/merchants/detail/index` | `operator-merchant-management` | `weapp.merchantDetail` | Covered |
| `pages/operator/riders/index` | `operator-rider-management` | `weapp.riderList` | Covered |
| `pages/operator/riders/detail/index` | `operator-rider-management` | `weapp.riderDetail` | Covered |
| `pages/operator/rules/index` | `operator-region-rules-and-expansion` | `weapp.rulesPage` | Covered |
| `pages/operator/region/index` | `operator-region-rules-and-expansion` | `weapp.regionList` | Covered |
| `pages/operator/region/config` | `operator-region-rules-and-expansion` | `weapp.regionConfig` | Covered |
| `pages/operator/timeslot/index` | `operator-region-rules-and-expansion` | `weapp.timeslotPage` | Covered |
| `pages/operator/delivery-fee/index` | `operator-region-rules-and-expansion` | `weapp.deliveryFeePage` | Covered |
| `pages/operator/safety/report/index` | `operator-safety-and-recovery` | `weapp.safetyList` | Covered |
| `pages/operator/safety/detail/index` | `operator-safety-and-recovery` | `weapp.safetyDetail` | Covered |
| `pages/operator/finance/withdraw/index` | `operator-finance-and-baofu-withdrawal` | `weapp.financeOverview` | Covered |
| `pages/operator/finance/bills/index` | `operator-finance-and-baofu-withdrawal` | `weapp.financeBills` | Covered |
| `pages/operator/finance/withdrawals/index` | `operator-finance-and-baofu-withdrawal` | `weapp.withdrawalList` | Covered |
| `pages/operator/finance/withdrawals/create/index` | `operator-finance-and-baofu-withdrawal` | `weapp.withdrawalCreate` | Covered |
| `pages/operator/finance/withdrawals/detail/index` | `operator-finance-and-baofu-withdrawal` | `weapp.withdrawalDetail` | Covered |
| `pages/operator/finance/settlement-account/index` | `operator-finance-and-baofu-withdrawal` | `weapp.settlementStatus` | Covered |
| `pages/operator/finance/settlement-account/submit/index` | `operator-finance-and-baofu-withdrawal` | `weapp.settlementSubmit` | Covered |
| `pages/operator/region-expansion/index` | `operator-region-rules-and-expansion` | `weapp.regionExpansionPage` | Covered |

## Operator-Related Backend And Background Touchpoints

These touch operator-visible state but are not necessarily operator page entrypoints.

1. `/v1/operator` route group.
   Covered by dashboard, dispatch, region/rules, merchant, rider, and safety slices. It owns region-scoped operational reads and food-safety writes.

2. `/v1/operators/me` route group.
   Covered by dashboard/notifications, finance/Baofu, and region rules proxy slices. It owns operator finance, notifications, Baofu settlement/withdrawal, and API-only rule proxy paths.

3. Dispatch timeout background flow.
   Covered by `operator-dispatch-hall.slice.md`: scheduler scans pending deliveries, writes `delivery_timeout_alerts`, enqueues operator notification tasks, and dispatch hall reads the pending pool. Operator write/action closure is intentionally outside current code.

4. Food-safety upstream and downstream state.
   Covered by `operator-safety-and-recovery.slice.md`: customer reports create cases/incidents and suspend merchant/takeout state; operator investigate/resolve releases only food-safety-owned suspension and paused order state.

5. Recovery-dispute and claim-recovery visibility.
   Covered by `operator-safety-and-recovery.slice.md`: operator can read recovery-dispute/recovery state, while merchant/rider dispute creation and automatic review/post-process own mutation.

6. Platform/admin region-expansion approval.
   Covered as a boundary in `operator-region-rules-and-expansion.slice.md`: operator submits and reads applications; platform/admin approves or rejects.

7. Merchant and rider management visibility.
   Covered by merchant/rider slices: operator can read and, for merchants, update capability labels. Suspension/resume, deposit, onboarding, approval, online eligibility, and payment closure remain outside operator management.

8. Baofu account and withdrawal callbacks/workers.
   Covered by `operator-finance-and-baofu-withdrawal.slice.md`: callbacks, command dispatch, fact application, and recovery schedulers converge settlement/withdrawal state outside Mini Program request timing.

9. Baofu and direct-payment provider evidence boundaries.
   Covered by finance/Baofu slice and Baofu domain standards. Local tests do not equal real funds-action C4 evidence.

10. Rule-engine proxy.
    Covered as API-only in `operator-region-rules-and-expansion.slice.md`; the current Mini Program rules page uses `/v1/operator/rules`, not `/v1/operators/me/rules/**`.

## Dead, Stale, Or Non-Current Operator-Related Code

1. Merchant list `keyword` and rider list `keyword`/`online_status`.
   Status: covered as contract drift in `flow-variant-index.md`; current backend does not bind or filter these values.

2. Merchant/rider suspend/resume request types.
   Status: stale frontend API types, covered in merchant/rider slices and `flow-variant-index.md`; no live operator route or page action was found.

3. Rider `offline` status type.
   Status: covered as contract drift; `offline` is display/online-state vocabulary, not a current backend rider status filter.

4. `/v1/operators/me/profit-sharing/configs`.
   Status: backend route with tests, covered as API-only/no-page-caller in finance/Baofu slice and `flow-variant-index.md`.

5. Operator recovery-dispute/recovery APIs.
   Status: backend read-only/API-only boundary, covered in safety/recovery slice and `flow-variant-index.md`; no current operator Mini Program page caller was found.

6. `/v1/operators/me/rules/**` proxy routes.
   Status: backend API-only proxy surface, covered in region/rules slice and `flow-variant-index.md`; current Mini Program rules page does not call it.

7. `operator_finance.sql` and `withdrawal_records`.
   Status: legacy/non-current withdrawal candidate, covered in finance/Baofu slice and `flow-variant-index.md`; current operator Baofu withdrawal path uses `baofu_withdrawal_orders` and external payment command/fact tables.

## Residual Non-Closure

No additional operator Mini Program entrypoint was found missing from the seven operator slices after this pass.

Residual work remains, but it is not missing codegraph coverage:

- Product decisions for multi-region behavior across dashboard, merchant, rider, safety, finance, and configs.
- Frontend/backend parameter alignment for merchant/rider search, summary region filters, rider online/offline vocabulary, and analytics DTO comments.
- Trust-boundary cleanup such as peak-hour list region authorization and recovery-dispute request binding.
- Operational action-loop design for dispatch timeout escalation.
- Real Baofu funds-action evidence before claiming provider C4 withdrawal success.

Those items are deliberately durable in `flow-variant-index.md` so they survive handoff, model context switching, and future implementation planning.
