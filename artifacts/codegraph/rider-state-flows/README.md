# Rider State Flows Codegraph

Status: created 2026-06-08
Risk class: G3 - identity documents, OCR, rider eligibility, delivery state machine, deposit/refund, Baofu settlement/withdrawal, claim recovery, behavior sanctions, provider callbacks
Scope: WeChat Mini Program rider/register pages -> rider/delivery backend routes -> logic/transactions -> SQL tables -> async callbacks/workers/recovery schedulers -> dead/orphan paths

## Slice Map

- `rider-application-onboarding.slice.md`: rider registration, document OCR, application submit/review, rider activation, credential lifecycle/restore/suspension.
- `rider-workbench-status-location.slice.md`: dashboard/workbench, rider profile, current region, status, online/offline, live location, notification center, geofence auto transitions.
- `rider-delivery-lifecycle.slice.md`: order hall, realtime pool notifications, recommend/grab, pending-dispatch timeout schedulers, navigation route planning, pickup/delivery state machine, deposit freeze/unfreeze, order status sync.
- `rider-deposit.slice.md`: rider deposit recharge, balance, deposit ledger, deposit withdrawal/refund, payment facts and recovery.
- `rider-income-and-baofu-withdrawal.slice.md`: income ledger, Baofu settlement account onboarding, Baofu rider income withdrawal and provider callbacks/recovery.
- `rider-claims-and-recovery.slice.md`: rider claim list/detail/decision, recovery disputes, claim recovery payment, overdue sanctions, compensation/release.
- `flow-variant-index.md`: compact branch/dead-code index across all rider-side slices.

Each `*.edges.json` uses the same compact edge schema as the existing merchant slices: only core page/API/logic/transaction/table/provider edges are modeled, while branch detail stays in the Markdown slices.

## Mini Program Entrypoints

The rider-facing Mini Program package is declared in `weapp/miniprogram/app.json:12` through `weapp/miniprogram/app.json:29`:

- `pages/rider/dashboard/index`
- `pages/rider/order-hall/index`
- `pages/rider/income/index`
- `pages/rider/income/withdrawals/index`
- `pages/rider/income/withdrawals/create/index`
- `pages/rider/income/withdrawals/detail/index`
- `pages/rider/settlement-account/index`
- `pages/rider/settlement-account/submit/index`
- `pages/rider/claims/index`
- `pages/rider/claims/detail/index`
- `pages/rider/deposit/index`
- `pages/rider/navigation/index`
- `pages/rider/task-detail/index`
- `pages/rider/tasks/index`

The rider registration entry is declared under the register package at `weapp/miniprogram/app.json:64` through `weapp/miniprogram/app.json:72` as `pages/register/rider/index`.

## Backend Route Surface

Rider application routes are registered at `locallife/api/server.go:1122` through `locallife/api/server.go:1131`.

Rider profile, settlement, deposit, income, workbench, status, location, claims, recovery, and disputes are registered at `locallife/api/server.go:1132` through `locallife/api/server.go:1174`.

Delivery rider routes are registered at `locallife/api/server.go:1178` through `locallife/api/server.go:1199`.

Shared authenticated routes used by rider pages include generic media routes at `locallife/api/server.go:658` through `locallife/api/server.go:664`, location route planning at `locallife/api/server.go:688`, payment query/detail recovery at `locallife/api/server.go:1107` through `locallife/api/server.go:1108`, and notification routes at `locallife/api/server.go:1262` through `locallife/api/server.go:1268`.

Provider callbacks used by rider money/settlement flows are registered at `locallife/api/server.go:542` through `locallife/api/server.go:557`.

## Cross-Slice Invariants

- A user becomes a rider only after rider application approval creates `riders` and `user_roles(role='rider')` in one transaction.
- Credential expiry governance can suspend riders through `rider_profiles`; renewed approved credentials can restore only the credential-owned suspension.
- A rider can go online only when status is eligible, current region exists, available deposit meets the region threshold, and Baofu settlement readiness is payment-ready.
- Rider private identity/health document previews and deletes must go through media ownership checks and application draft-state checks before touching `media_assets` or `rider_applications`.
- Rider notification center is a generic `notifications.user_id` view with rider-mode client categorization; it is not a separate authorization boundary.
- Rider order hall can discover new orders through recommend reads or realtime `delivery_pool_new`, but grab remains the only writer that assigns a rider.
- A rider can grab only from a live delivery pool row; grab atomically locks order, rider, pool, delivery, deposit, and rider profit-sharing bill state.
- Pending-dispatch timeout schedulers can alert operations/merchants and cancel stale pending deliveries; cancellation now removes rider-visible `delivery_pool` rows in `CancelOrderTx`.
- Navigation route planning uses authenticated map-provider access and never writes delivery state.
- Rider locations are accepted only for online riders and only for the current active delivery when a delivery id is supplied.
- Delivery terminal rider completion unfreezes the per-order deposit and updates rider stats; payment income visibility comes from Baofu profit-sharing orders, not from rider deposit logs.
- Rider deposit recharge, Baofu verify-fee payment, and claim recovery payment can converge through callback facts or Mini Program-triggered payment query facts; terminal truth flows through payment fact application, not through client `requestPayment` result alone.
- Rider income ledger status is written by Baofu profit-sharing command/callback/query fact application and read from `profit_sharing_orders`.
- Rider income withdrawal uses Baofu account balance and `baofu_withdrawal_orders`; it does not freeze local rider deposit or local income ledger rows.
- Claim recovery can suspend riders through behavior actions; dispute approval waives/release recovery and may create payout compensation, while rejection resumes pending/overdue recovery.

## Dead And Orphan Summary

- Resolved: older delivery-task-management detail wrapper is now backed by `GET /v1/delivery/:delivery_id` with order-owner/assigned-rider authorization.
- Resolved: worker fallback rider new-order push emits `delivery_pool_new`, matching Mini Program listeners.
- Resolved: stale pending-delivery cancellation removes the matching `delivery_pool` row through `CancelOrderTx`.
- Resolved: rider appeal-compatible wrappers now call `/v1/rider/recovery-disputes` list/detail/create and adapt the list response for existing pages.
- `locallife/worker/task_risk_management.go:64` through `locallife/worker/task_risk_management.go:72` keep the legacy `risk:check_rider_damage` task consumer but intentionally ignore it because claim behavior trace is now the main adjudication path.
- `locallife/worker/task_process_payment.go:794` through `locallife/worker/task_process_payment.go:797` intentionally skips rider deposit refund result application in the legacy refund worker; rider deposit refund facts must be applied through payment fact application.

## Operations Gaps

- Scheduler auto-cancel removes SQL pool truth but does not emit an observed `delivery_pool_gone` realtime invalidation, so already-open rider clients may need refresh/reconnect to remove a cancelled recommendation card immediately.

## Validation

This directory is documentation/artifact-only. Validate edge JSON with:

```bash
jq empty artifacts/codegraph/rider-state-flows/*.edges.json
```
