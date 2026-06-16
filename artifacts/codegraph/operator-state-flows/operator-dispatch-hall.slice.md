# Operator Dispatch Hall Slice

Status: operator-state flow slice created 2026-06-14
Risk class: G2/G3 boundary - regional dispatch visibility, pending-delivery timeout alerts, operator notification, cross-role cancellation/refund handoff
Scope: Mini Program operator dispatch hall -> managed-region selector -> pending delivery summary/list APIs -> SQL pending delivery pool reads -> 3-minute timeout alert scheduler/worker -> operator notification source -> cross-role delivery timeout boundary

## Variant Coverage

This slice covers:

- Operator dispatch hall page entry, optional `region_id` deep link from notification detail, managed-region picker, pull refresh, pagination, and notification-list navigation.
- Frontend dispatch monitor service and API wrappers for `/v1/operator/regions/:region_id/delivery-pool/summary` and `/delivery-pool`.
- Backend operator pending-dispatch summary/list routes, including operator-region authorization and pagination.
- SQL reads over `deliveries`, `orders`, `merchants`, and `regions` for pending deliveries and timeout-over-3-minute flags.
- Scheduler source for 3-minute pending-dispatch alerts, its `delivery_timeout_alerts` dedupe ledger, and worker creation of operator-audience notifications.

This slice does not fully cover:

- Rider order hall recommendation/grab and delivery state transitions; covered by `artifacts/codegraph/rider-state-flows/rider-delivery-lifecycle.slice.md`.
- Merchant/customer/order cancellation, refund recovery, and rider-visible pool cleanup after 60-minute timeout; covered by rider delivery lifecycle and platform operations backlog.
- A direct operator assign/cancel/force-dispatch action. Current operator dispatch hall is read/notify/handoff only.
- Notification read/list/detail behavior after alerts are created; covered by `operator-dashboard-analytics-notifications.slice.md`.

## Product Invariant

Operator dispatch hall is a region-scoped monitor, not a second delivery state machine:

- The selected region must be one of the current operator's managed regions.
- Pending-dispatch rows are read from durable delivery/order state where `deliveries.status='pending'`.
- The 3-minute timeout alert ledger dedupes notification enqueue; it does not mutate the delivery or order.
- Alert worker rechecks the delivery is still pending and old enough before notifying active operators in the merchant's region.
- Operator notification read-side can lead the operator back into dispatch hall by region id, but the operator cannot grab, assign, cancel, or refund from this slice.

## Primary Forward Chain

1. Operator Mini Program declares dispatch hall and notification detail entries; notification detail can navigate to dispatch hall with `region_id`.
   Evidence: `weapp/miniprogram/app.json:38`, `weapp/miniprogram/app.json:40`, `weapp/miniprogram/pages/operator/notifications/detail/index.ts:63`, `weapp/miniprogram/pages/operator/notifications/detail/index.ts:69`.

2. Dispatch hall loads managed regions, applies an optional preferred `region_id`, and loads the selected region's summary/list.
   Evidence: `weapp/miniprogram/pages/operator/dispatch-hall/index.ts:56`, `weapp/miniprogram/pages/operator/dispatch-hall/index.ts:63`, `weapp/miniprogram/pages/operator/dispatch-hall/index.ts:95`, `weapp/miniprogram/pages/operator/dispatch-hall/index.ts:124`.

3. Frontend dispatch monitor service calls summary and list concurrently, then adapts wait time, delivery fee, expected pickup time, and pagination state into operator-readable view models.
   Evidence: `weapp/miniprogram/pages/operator/_services/operator-dispatch-monitor.ts:95`, `weapp/miniprogram/pages/operator/_services/operator-dispatch-monitor.ts:107`, `weapp/miniprogram/pages/operator/_services/operator-dispatch-monitor.ts:117`, `weapp/miniprogram/pages/operator/_services/operator-dispatch-monitor.ts:124`, `weapp/miniprogram/pages/operator/_services/operator-dispatch-monitor.ts:130`.

4. Frontend API wrappers call registered operator delivery-pool routes.
   Evidence: `weapp/miniprogram/pages/operator/_api/operator-dispatch-monitor.ts:37`, `weapp/miniprogram/pages/operator/_api/operator-dispatch-monitor.ts:48`, `locallife/api/server.go:1363`, `locallife/api/server.go:1364`.

5. Backend summary/list handlers bind `region_id`, verify `checkOperatorManagesRegion`, use `status='pending'`, and calculate timeout threshold from server time.
   Evidence: `locallife/api/operator_dispatch_monitor.go:84`, `locallife/api/operator_dispatch_monitor.go:91`, `locallife/api/operator_dispatch_monitor.go:96`, `locallife/api/operator_dispatch_monitor.go:137`, `locallife/api/operator_dispatch_monitor.go:150`, `locallife/api/operator_dispatch_monitor.go:156`.

6. Summary SQL counts pending deliveries and pending deliveries older than threshold for a region; list SQL returns delivery/order/merchant/region context ordered with timed-out rows first.
   Evidence: `locallife/db/query/operator_dispatch_monitor.sql:1`, `locallife/db/query/operator_dispatch_monitor.sql:16`, `locallife/db/query/operator_dispatch_monitor.sql:38`.

7. The data cleanup scheduler runs the operator pending-dispatch alert scan every minute, lists pending deliveries older than 3 minutes without the dedupe ledger, writes `delivery_timeout_alerts`, then enqueues `operator:pending_dispatch_alert`.
   Evidence: `locallife/scheduler/data_cleanup.go:93`, `locallife/scheduler/operator_dispatch_alert.go:14`, `locallife/scheduler/operator_dispatch_alert.go:19`, `locallife/scheduler/operator_dispatch_alert.go:28`, `locallife/scheduler/operator_dispatch_alert.go:43`, `locallife/scheduler/operator_dispatch_alert.go:58`, `locallife/db/query/delivery_timeout_alert.sql:1`, `locallife/db/query/delivery_timeout_alert.sql:15`.

8. If enqueue fails after the dedupe ledger is created, the scheduler deletes the ledger so a later scan can retry.
   Evidence: `locallife/scheduler/operator_dispatch_alert.go:64`, `locallife/scheduler/operator_dispatch_alert.go:66`.

9. Alert worker reloads the delivery, skips missing/non-pending/not-old-enough rows, loads order and merchant, finds active operator notification recipients by merchant region, and sends operator-audience `dispatch_timeout` notifications.
   Evidence: `locallife/worker/task_operator_pending_dispatch_alert.go:50`, `locallife/worker/task_operator_pending_dispatch_alert.go:56`, `locallife/worker/task_operator_pending_dispatch_alert.go:65`, `locallife/worker/task_operator_pending_dispatch_alert.go:74`, `locallife/worker/task_operator_pending_dispatch_alert.go:82`, `locallife/worker/task_operator_pending_dispatch_alert.go:97`, `locallife/worker/task_operator_pending_dispatch_alert.go:112`, `locallife/worker/task_operator_pending_dispatch_alert.go:121`, `locallife/db/query/operator_region.sql:30`.

10. Longer pending-delivery timeout cancellation and refund recovery are outside the operator hall page. The 60-minute scheduler path calls `CancelOrderTx` and handles delivery/refund cleanup independently of any operator page action.
    Evidence: `locallife/scheduler/data_cleanup.go:1339`, `locallife/scheduler/data_cleanup.go:1347`, `locallife/db/sqlc/tx_order_status.go:145`.

## SQL And Durable State Boundaries

- `deliveries`: pending status, created time, delivery fee, estimated pickup time, and cancellation boundary.
- `orders`: order number and merchant association for pending dispatch rows.
- `merchants`: merchant name and region id used for operator scoping and alert recipients.
- `regions`: region name and selected-region summary context.
- `operator_regions`: current active operator-region authority and active alert recipients.
- `delivery_timeout_alerts`: dedupe ledger for 3-minute operator pending-dispatch alerts.
- `notifications`: operator-audience dispatch timeout notifications created by the worker.

## Trust, Authorization, And Tenant Checks

- Dispatch hall routes require operator auth through the `/v1/operator` route group.
- Summary/list handlers verify the requested `region_id` with `checkOperatorManagesRegion` before reading pending deliveries.
- Alert worker recipients are looked up from active `operator_regions` and active `operators` for the merchant's region.
- Operator notification rows are scoped by user id and `extra_data.audience='operator'` in the notification slice.

## Idempotency And Duplicate-Submit Checks

- Summary/list reads are idempotent and server-time based.
- Scheduler dedupes 3-minute alerts by `(delivery_id, alert_key)` in `delivery_timeout_alerts`.
- Duplicate ledger insert conflicts are skipped.
- If task enqueue fails, the scheduler removes the ledger so the alert can be retried.
- Worker skips stale tasks when delivery is no longer pending or has not yet reached threshold.

## Recovery And Async Convergence Paths

- Dispatch hall can refresh manually, on pull-down, on region change, and on page show.
- Notification detail deep links back to dispatch hall by region id; dispatch hall revalidates that region through the managed-region picker and backend route auth.
- If alert task processing fails while sending notifications, Asynq retry handles the worker task.
- If Pub/Sub/page state is stale, dispatch hall reloads durable pending delivery SQL.
- Longer timeout cancellation/refund recovery is handled by scheduler/order/payment flows outside this operator page.

## Frontend Draft And Backend Rehydration

- Preferred `region_id` from notification detail is only a page hint. If it is not in the operator's managed-region list, dispatch hall falls back to the default region.
- Dispatch rows and timeout counts are always rehydrated from backend summary/list reads.
- The page does not keep local delivery action state because there is no operator-side delivery mutation action here.

## Test Coverage Signals

Observed tests:

- `locallife/api/operator_dispatch_monitor_test.go` covers summary/list happy paths and unmanaged-region denial.
- `locallife/scheduler/operator_dispatch_alert_test.go` covers alert enqueue, duplicate ledger skip, and ledger rollback on enqueue failure.
- `locallife/worker/task_operator_pending_dispatch_alert_test.go` covers operator notification creation, before-threshold skip, and no-recipient skip.

Missing high-value tests:

- Mini Program contract/page-level test for notification detail `region_id` deep link into dispatch hall.
- End-to-end pending delivery older than 3 minutes -> ledger -> worker notification -> operator notification list -> dispatch hall handoff.
- Regression that dispatch hall never exposes assign/cancel/refund actions unless a future operator action-loop design explicitly adds them.

## Gaps And Refactor Notes

- Dispatch hall is intentionally read-only today. Cross-role operational closure for who contacts riders/merchants, who cancels, and who owns refund/manual intervention remains tracked outside this slice.
- The notification copy says "提醒骑手接单", but the page does not show an action channel for contacting riders. A future operator action-loop design should either add an explicit action path or adjust copy to the real operational behavior.
- `delivery_timeout_alerts` dedupes only the 3-minute alert key; later timeout milestones should use separate keys if added.

## Branch Exhaustion

- Entry branches checked: dispatch hall direct open, notification-detail deep link, no managed region, preferred region not managed, region change, pull refresh, load more, retry, notification list navigation.
- Request branches checked: `/v1/operator/regions`, `/v1/operator/regions/:region_id/delivery-pool/summary`, `/v1/operator/regions/:region_id/delivery-pool`.
- Backend state branches checked: invalid region id, unmanaged region, empty pending list, timed out and not timed out pending rows, pagination, no active operator recipients, delivery missing, delivery no longer pending, threshold not reached.
- Async branches checked: one-minute scheduler scan, timeout ledger create, duplicate ledger skip, enqueue rollback, worker notification send, Asynq retry.
- Dead/orphan branches checked: no dead dispatch-hall route found; current operator dispatch hall has no write action by design.
