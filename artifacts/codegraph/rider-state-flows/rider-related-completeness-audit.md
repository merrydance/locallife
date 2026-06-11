# Rider Related Completeness Audit

Status: updated 2026-06-09
Purpose: answer whether the rider audit has exhausted every rider-related flow, including cross-role/background paths and dead code.

## Verdict

The six rider slices exhaust the rider-facing and rider-actionable closure: from Mini Program rider/register entry, through backend routes, logic/transactions, SQL truth, provider callbacks, workers/schedulers, recovery paths, and known dead/orphan branches.

The stricter phrase "all rider-related flows" is broader than rider-side closure. Under that definition, cross-role and background touchpoints must also be counted. This file indexes those touchpoints so they are not silently omitted. Some of their operational closure questions remain owned by platform/operator/merchant/user audits and are parked under `artifacts/codegraph/platform-operations-closed-loop/`.

## Rider-Facing Closure Covered

1. Application and identity: `rider-application-onboarding.slice.md`
2. Workbench, status, region, location, geofence, notifications: `rider-workbench-status-location.slice.md`
3. Order hall, delivery lifecycle, timeout cancellation, navigation, pool realtime: `rider-delivery-lifecycle.slice.md`
4. Deposit recharge, balance, withdrawal/refund, credits, abnormal refund recovery: `rider-deposit.slice.md`
5. Income ledger, Baofu settlement account, verify fee, profit sharing, Baofu income withdrawal: `rider-income-and-baofu-withdrawal.slice.md`
6. Claims, recoveries, disputes, overdue sanctions, release/compensation: `rider-claims-and-recovery.slice.md`

## Cross-Role Rider Touchpoints

These are rider-related but not rider-side entry flows. They must be considered in a full-system audit.

1. Platform/admin rider management is a rider state write path.
   Evidence: Mini Program platform rider pages are declared in `weapp/miniprogram/app.json:88` through `weapp/miniprogram/app.json:90`; backend routes `/v1/admin/riders`, `/pause-accepting`, and `/resume-accepting` are registered at `locallife/api/server.go:1204` through `locallife/api/server.go:1210`.
   Status: counted as cross-role rider-related. Deeper platform workflow closure is outside rider-side closure.

2. Operator rider management is rider-related read/operations visibility.
   Evidence: Mini Program operator rider pages are declared at `weapp/miniprogram/app.json:42` through `weapp/miniprogram/app.json:44`; backend operator rider list/summary/detail/stats/ranking routes are registered at `locallife/api/server.go:1369` and `locallife/api/server.go:1383` through `locallife/api/server.go:1387`.
   Status: counted as cross-role rider-related read surface; operator pause/resume is intentionally absent.

3. Operator pending-dispatch monitoring affects rider delivery-pool closure.
   Evidence: operator dispatch hall reads `/v1/operator/regions/:region_id/delivery-pool/summary` and `/delivery-pool`; backend routes are registered at `locallife/api/server.go:1359` through `locallife/api/server.go:1360`; alert scheduler writes `delivery_timeout_alerts` and enqueues operator alert tasks.
   Status: rider-visible pool truth is handled in `rider-delivery-lifecycle`; operator action-loop closure is parked in `platform-operations-closed-loop`.

4. Merchant and customer order paths create, block, complete, or cancel rider-visible delivery state.
   Evidence: merchant order accept/ready routes are registered under `/v1/merchant/orders` at `locallife/api/server.go:1040` through `locallife/api/server.go:1044`; customer order cancel/confirm routes are registered at `locallife/api/server.go:1025` through `locallife/api/server.go:1030`; delivery pool/order status effects are covered in `rider-delivery-lifecycle.slice.md`.
   Status: counted as cross-role delivery inputs; rider slice follows them from the delivery-pool/status boundary forward.

5. User completion and auto-completion trigger rider income settlement.
   Evidence: order SQL has `rider_delivered -> user_delivered/completed` branches in `locallife/db/query/order.sql`; auto-complete scheduler is in `locallife/scheduler/takeout_auto_complete.go`; rider income convergence is covered in `rider-income-and-baofu-withdrawal.slice.md`.
   Status: counted as cross-role rider-income trigger.

6. Merchant delivery fee/promotion settings influence customer pricing and delivery-pool economics.
   Evidence: merchant delivery-promotion pages are declared at `weapp/miniprogram/app.json:347` through `weapp/miniprogram/app.json:348`; delivery-fee config/promotion/calculate routes are registered at `locallife/api/server.go:873` through `locallife/api/server.go:897`.
   Status: mostly covered by merchant marketing/order slices; rider delivery slice treats final `delivery_pool.delivery_fee` and rider bill rows as canonical.

7. Customer/order tracking reads rider location.
   Evidence: delivery track/latest-rider-location routes are registered at `locallife/api/server.go:1199` through `locallife/api/server.go:1200`; fallback current-rider-location exposure is implemented in `locallife/api/delivery_location_fallback.go`.
   Status: counted as rider data exposure surface; rider-side location write rules are covered in `rider-workbench-status-location`.

8. Claims may originate from customer-side claim submission before becoming rider-visible.
   Evidence: shared claim wrappers include customer claim submit/read routes, while rider-visible claim/recovery ownership starts at `/v1/rider/claims/**` and `/recoveries/**`.
   Status: original customer claim creation remains outside rider-side closure; rider ownership, payment, dispute, sanction, and release paths are covered.

9. Provider callbacks and payment-domain workers mutate rider money/status outside the Mini Program page that initiated the action.
   Evidence: WeChat callbacks/query facts cover rider deposit, claim recovery, and Baofu verify fee; Baofu share/account/withdraw callbacks and recovery workers cover income, settlement readiness, and withdrawals.
   Status: covered inside deposit, income/Baofu, and claims/recovery slices.

10. Schedulers and workers mutate rider-related truth without a user page entry.
    Evidence: credential governance, rider deposit credit expiry, stale delivery cleanup, pending-dispatch alerts, takeout auto-complete, claim recovery overdue, automatic dispute resolution, behavior actions, payment fact application, and Baofu recovery schedulers are each referenced in the relevant rider slice.
    Status: covered where the mutation is rider-visible; cross-role operational closure gaps are parked separately.

## Dead Or Stale Rider-Related Code

1. Stale copied frontend wrappers still post to `/onboarding/rider`.
   Evidence: `weapp/miniprogram/pages/register/_main_shared/api/onboarding.ts`, `weapp/miniprogram/pages/user_center/addresses/_main_shared/api/onboarding.ts`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts`, and `weapp/miniprogram/pages/operator/_main_shared/api/onboarding.ts` each preserve a `submitRiderApplication` helper using `/onboarding/rider`; no backend route is registered for that path. Current rider registration imports `weapp/miniprogram/pages/register/rider/_api/rider-application.ts` and submits to `/v1/rider/application/submit`.
   Status: dead/stale shared wrapper, not live rider registration flow.

2. Older delivery task management wrapper is live but no longer orphaned.
   Evidence: `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts` calls registered `/v1/delivery/**` routes including `GET /v1/delivery/:delivery_id`.
   Status: resolved.

3. Legacy rider damage risk task is intentionally no-op.
   Evidence: `locallife/worker/task_risk_management.go` keeps `risk:check_rider_damage` but logs that claim behavior trace handles adjudication.
   Status: intentional dead path.

4. Legacy refund-result worker refuses rider deposit refund application.
   Evidence: `locallife/worker/task_process_payment.go` skips rider deposit refund result application and routes terminal truth through payment fact application.
   Status: intentional dead path.

## Residual Non-Closure

No additional rider-facing branch was found missing from the six rider slices after this pass.

The remaining non-closure is operational, not rider-facing: platform/operator/merchant/user work-order handling, manual reconciliation queues, unified incident timelines, and action ownership for cross-role timeout/refund/credential/payment exceptions. Those are not safe to mark "exhausted" until their own role audits are completed.
