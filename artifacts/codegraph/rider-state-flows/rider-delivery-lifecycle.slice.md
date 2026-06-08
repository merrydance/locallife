# Rider Delivery Lifecycle Slice

Status: rider-state flow slice created 2026-06-08
Risk class: G3 - order assignment, delivery state machine, rider deposit freeze/unfreeze, Baofu rider profit-sharing bill, food-safety/fulfillment guards, rider-facing delivery visibility and notifications
Scope: Mini Program rider dashboard/order hall/task/navigation pages -> delivery APIs -> delivery grab/status logic -> delivery timeout schedulers -> delivery transactions -> orders/deliveries/rider deposit/profit-sharing SQL truth

## Variant Coverage

This slice covers:

- Rider order hall/dashboard available orders and active delivery cards.
- Delivery API wrappers for recommend, grab, active/history, status transitions, order detail, latest rider location, and track.
- Rider navigation page route planning through authenticated `/v1/location/direction/bicycling` and provider map route lookup.
- Backend delivery routes under `/v1/delivery/**`.
- Delivery grab validation and transaction, including delivery pool locking, deposit freeze, Baofu rider profit-sharing bill assignment, order status sync, and broadcast removal.
- New delivery visibility side paths: order payment creates/keeps a `delivery_pool` row, API/worker broadcast `delivery_pool_new` as nearby-candidate new-order reminders, `delivery_pool_gone` broadcasts removal to online active riders inside the recommendation-visible radius, dashboard increments the new-order counter, and manual refresh rehydrates from recommend.
- Pending-dispatch backend schedulers: 3-minute operator alert, 20-minute delayed merchant alert, 60-minute auto-cancel/refund path, and post-cancel pool cleanup.
- Rider delivery transitions `assigned -> picking -> picked -> delivering -> delivered`, including merchant-ready and location-radius validation.
- Rider history/active list and track/latest-location readers.

This slice does not fully cover:

- Customer order creation/payment internals before a `delivery_pool` row exists; this slice covers the rider-visible pool row and realtime broadcast boundary.
- Merchant fulfillment internals before rider pickup confirmation, except the fulfillment-ready guard.
- Claim/recovery after a delivered order; that is covered by `rider-claims-and-recovery`.
- Baofu profit-sharing provider callbacks that settle rider income after delivery; they are covered by `rider-income-and-baofu-withdrawal`.
- Platform/operator/merchant action closure after dispatch timeout; cross-role backlog is parked in `artifacts/codegraph/platform-operations-closed-loop/`.

## Product Invariant

Rider delivery state must be one canonical delivery/order chain:

- A rider can grab only from an unexpired pool row for an order whose status allows delivery action.
- Order-hall visibility may arrive by read-side recommend or by realtime pool-new notification, but the source of truth remains `delivery_pool`; realtime notifications do not assign the order.
- Grab must atomically assign delivery, remove pool row, freeze rider deposit, update the order to `courier_accepted`, and bind the rider profit-sharing bill.
- Pending-dispatch timeout paths must reconcile `orders`, `deliveries`, rider-visible pool rows, notifications, and refund recovery; `CancelOrderTx` now removes rider-visible pool rows during successful cancellation.
- Only the assigned rider can move a delivery through pickup/delivery states.
- Pickup confirmation must not bypass merchant readiness or food-safety pause rules.
- Delivery confirmation must validate fresh rider location near dropoff, then atomically set delivered state, unfreeze deposit, update rider stats, and sync order status to `rider_delivered`.

## Primary Forward Chain

1. Rider dashboard/order-hall entries show available orders and active deliveries, and the button flow calls grab/status transition handlers.
   Evidence: `weapp/miniprogram/pages/rider/dashboard/index.ts:60`, `weapp/miniprogram/pages/rider/order-hall/index.ts:59`, `weapp/miniprogram/pages/rider/order-hall/index.wxml:77`, `weapp/miniprogram/pages/rider/order-hall/index.wxml:122`, `weapp/miniprogram/pages/rider/order-hall/index.wxml:154`.

2. The shared delivery API wrapper calls registered routes for recommend, grab, start pickup, confirm pickup, start delivery, confirm delivery, order detail, latest rider location, and track.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:173`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:182`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:194`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:204`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:214`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:224`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:234`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:244`, `weapp/miniprogram/pages/rider/_main_shared/api/delivery.ts:254`.

3. The older delivery-task-management wrapper still calls the same live routes for recommend/grab/active/history/transitions, and its generic delivery detail call is now backed by a registered `GET /v1/delivery/:delivery_id` route.
   Evidence: `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:109`, `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:119`, `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:131`, `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:160`, `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:172`, `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:184`.

4. Backend delivery rider routes are registered under `/v1/delivery`.
   Evidence: `locallife/api/server.go:1181`, `locallife/api/server.go:1184`, `locallife/api/server.go:1187`, `locallife/api/server.go:1188`, `locallife/api/server.go:1191`, `locallife/api/server.go:1192`, `locallife/api/server.go:1193`, `locallife/api/server.go:1194`, `locallife/api/server.go:1197`, `locallife/api/server.go:1198`, `locallife/api/server.go:1199`.

5. New order visibility has two rider-facing entry paths: the API service publishes `delivery_pool_new` when an order is pooled, and the payment worker publishes nearby-rider new-order notifications after payment processing; both target `notification:rider:<id>` channels and rely on `delivery_pool` as the durable source.
   Evidence: `locallife/api/logic_adapters.go:112`, `locallife/api/logic_adapters.go:123`, `locallife/worker/task_process_payment.go:636`, `locallife/worker/task_process_payment.go:646`, `locallife/logic/delivery_broadcast.go:64`, `locallife/logic/delivery_broadcast.go:99`, `locallife/logic/delivery_broadcast.go:167`, `locallife/main.go:224`, `locallife/main.go:484`.

6. Dashboard/order-hall WebSocket runtime listens for `delivery_pool_new` and `delivery_pool_gone`; new increments a local badge and gone removes a matching recommended order. On reconnect, backend replay filters stale `delivery_pool_new` messages by checking that the pool row still exists.
   Evidence: `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:862`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:872`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:885`, `weapp/miniprogram/pages/rider/_main_shared/utils/websocket.ts:14`, `weapp/miniprogram/pages/rider/_main_shared/utils/websocket.ts:17`, `locallife/websocket/message_types.go:13`, `locallife/websocket/message_types.go:14`, `locallife/api/server.go:218`, `locallife/api/server.go:223`.

7. Recommend route resolves the current rider by user id through logic, scores delivery pool rows, enriches merchant/order/delivery/item data, and returns real distance/duration when available.
   Evidence: `locallife/api/delivery.go:94`, `locallife/api/delivery.go:103`, `locallife/api/delivery.go:116`, `locallife/api/delivery.go:122`, `locallife/api/delivery.go:144`, `locallife/api/delivery.go:148`, `locallife/api/delivery.go:151`, `locallife/api/delivery.go:156`.

8. A 3-minute dispatch scheduler scans pending deliveries not yet represented in `delivery_timeout_alerts`, writes the dedupe row, and enqueues an operator alert task; this is recorded here only because it shares the rider-visible pending-delivery timeout chain.
   Evidence: `locallife/scheduler/data_cleanup.go:89`, `locallife/scheduler/operator_dispatch_alert.go:19`, `locallife/scheduler/operator_dispatch_alert.go:29`, `locallife/scheduler/operator_dispatch_alert.go:47`, `locallife/scheduler/operator_dispatch_alert.go:59`, `locallife/scheduler/operator_dispatch_alert.go:66`, `locallife/db/query/delivery_timeout_alert.sql:1`, `locallife/db/query/delivery_timeout_alert.sql:15`.

9. A 10-minute cleanup scheduler handles pending deliveries older than 20 minutes by marking `is_delayed`, and older than 60 minutes by cancelling the order/delivery and enqueueing a refund task for successful external payments. For rider-side closure, `CancelOrderTx` removes the matching `delivery_pool` row in the same transaction as a successful order cancellation, and the scheduler now publishes `delivery_pool_gone` to online active riders inside the recommendation-visible radius after cancellation so already-open clients can remove the card without waiting for refresh.
   Evidence: `locallife/scheduler/data_cleanup.go:83`, `locallife/scheduler/data_cleanup.go:1293`, `locallife/scheduler/data_cleanup.go:1299`, `locallife/scheduler/data_cleanup.go:1322`, `locallife/scheduler/data_cleanup.go:1334`, `locallife/scheduler/data_cleanup.go:1342`, `locallife/scheduler/data_cleanup.go:1358`, `locallife/scheduler/data_cleanup.go:1369`, `locallife/scheduler/data_cleanup.go:1392`, `locallife/scheduler/data_cleanup.go:1410`, `locallife/scheduler/data_cleanup.go:1478`, `locallife/scheduler/data_cleanup.go:1525`, `locallife/db/sqlc/tx_order_status.go:197`, `locallife/db/query/delivery_pool.sql:37`, `locallife/logic/delivery_recommendation.go:50`, `locallife/api/delivery.go:116`, `locallife/db/sqlc/tx_order_status_test.go:600`, `locallife/scheduler/stale_delivery_cleanup_test.go:24`.

10. Grab route parses `order_id`, delegates to `logic.GrabDeliveryOrder`, then sends merchant notification, broadcasts order removal, reloads delivery, and returns backend delivery truth.
   Evidence: `locallife/api/delivery.go:436`, `locallife/api/delivery.go:445`, `locallife/api/delivery.go:483`, `locallife/api/delivery.go:499`, `locallife/api/delivery.go:513`.

11. Grab logic rejects non-rider, offline rider, non-active rider, suspended rider, missing Baofu settlement readiness, missing/expired pool row, too-far rider, missing delivery/order, disallowed order status, insufficient deposit, and missing/non-pending rider profit-sharing bill.
   Evidence: `locallife/logic/delivery_grab.go:54`, `locallife/logic/delivery_grab.go:62`, `locallife/logic/delivery_grab.go:65`, `locallife/logic/delivery_grab.go:69`, `locallife/logic/delivery_grab.go:75`, `locallife/logic/delivery_grab.go:83`, `locallife/logic/delivery_grab.go:90`, `locallife/logic/delivery_grab.go:113`, `locallife/logic/delivery_grab.go:127`, `locallife/logic/delivery_grab.go:133`, `locallife/logic/delivery_grab.go:138`, `locallife/logic/delivery_grab.go:141`.

12. Grab transaction locks food-safety/order/rider/pool state, assigns delivery, updates the rider profit-sharing bill, removes the pool row, freezes rider deposit, writes a freeze log, updates order status, and writes order status log.
   Evidence: `locallife/db/sqlc/tx_delivery.go:184`, `locallife/db/sqlc/tx_delivery.go:194`, `locallife/db/sqlc/tx_delivery.go:202`, `locallife/db/sqlc/tx_delivery.go:209`, `locallife/db/sqlc/tx_delivery.go:219`, `locallife/db/sqlc/tx_delivery.go:226`, `locallife/db/sqlc/tx_delivery.go:238`, `locallife/db/sqlc/tx_delivery.go:249`, `locallife/db/sqlc/tx_delivery.go:260`, `locallife/db/sqlc/tx_delivery.go:270`.

13. `start-pickup` requires the authenticated user to be the assigned rider and delivery status `assigned`, then moves delivery/order through the pickup transaction and notifies the customer.
   Evidence: `locallife/api/delivery.go:543`, `locallife/logic/delivery_status.go:115`, `locallife/logic/delivery_status.go:137`, `locallife/logic/delivery_status.go:141`, `locallife/logic/delivery_status.go:153`, `locallife/api/delivery.go:561`.

14. `confirm-pickup` requires assigned rider and status `picking`; it blocks when the takeout order is still `courier_accepted` and merchant fulfillment is not ready.
    Evidence: `locallife/api/delivery.go:596`, `locallife/logic/delivery_status.go:171`, `locallife/logic/delivery_status.go:195`, `locallife/logic/delivery_status.go:199`, `locallife/logic/delivery_status.go:211`, `locallife/logic/delivery_status.go:225`.

15. `start-delivery` requires assigned rider and status `picked`, writes delivery/order status to `delivering`, and logs the rider transition.
    Evidence: `locallife/api/delivery.go:654`, `locallife/logic/delivery_status.go:261`, `locallife/logic/delivery_status.go:284`, `locallife/logic/delivery_status.go:288`, `locallife/logic/delivery_status.go:301`, `locallife/logic/delivery_status.go:308`.

16. `confirm-delivery` requires assigned rider and status `delivering`, validates rider latest location freshness and dropoff radius, then calls `CompleteDeliveryTx`.
    Evidence: `locallife/api/delivery.go:707`, `locallife/logic/delivery_status.go:320`, `locallife/logic/delivery_status.go:340`, `locallife/logic/delivery_status.go:344`, `locallife/logic/delivery_status.go:348`, `locallife/logic/delivery_status.go:355`.

17. Complete delivery transaction updates delivery to `delivered`, syncs order to `rider_delivered`, unfreezes the per-order deposit, writes an unfreeze log, updates rider stats, and may auto-offline a no-longer-eligible rider.
    Evidence: `locallife/db/sqlc/tx_delivery.go:317`, `locallife/db/sqlc/tx_delivery.go:339`, `locallife/db/sqlc/tx_delivery.go:349`, `locallife/db/sqlc/tx_delivery.go:358`, `locallife/db/sqlc/tx_delivery.go:368`, `locallife/db/sqlc/tx_delivery.go:382`, `locallife/db/sqlc/tx_delivery.go:389`.

18. Active list resolves current rider by user id and returns `ListRiderActiveDeliveries`; history supports status/date pagination and returns completed count and total earnings.
    Evidence: `locallife/api/delivery.go:1017`, `locallife/api/delivery.go:1068`, `locallife/api/delivery.go:1076`, `locallife/api/delivery.go:1091`, `locallife/api/delivery.go:1099`, `locallife/api/delivery.go:1132`, `locallife/api/delivery.go:1150`.

19. Track and latest rider location routes are registered and used by navigation/task pages for route display and live tracking.
   Evidence: `locallife/api/delivery.go:833`, `locallife/api/delivery.go:923`, `weapp/miniprogram/pages/rider/navigation/index.ts:127`, `weapp/miniprogram/pages/rider/navigation/index.ts:151`, `weapp/miniprogram/pages/rider/task-detail/index.ts:4`.

20. Navigation page also plans a pickup-to-dropoff route through shared `mapService.planRoute`, which calls authenticated `GET /v1/location/direction/bicycling`; the backend validates `from/to`, calls the configured map client, and returns distance/duration/points without writing SQL.
    Evidence: `weapp/miniprogram/pages/rider/navigation/index.ts:130`, `weapp/miniprogram/pages/rider/_main_shared/services/map.ts:87`, `weapp/miniprogram/pages/rider/_main_shared/services/map.ts:106`, `locallife/api/server.go:688`, `locallife/api/location.go:211`, `locallife/api/location.go:236`, `locallife/logic/route_service.go:57`, `locallife/logic/route_service.go:72`.

21. Worker fallback path now emits the same `delivery_pool_new` event type as the primary `DeliveryBroadcastLogic` path, so current rider Mini Program WebSocket listeners consume both paths.
    Evidence: `locallife/worker/task_process_payment.go:53`, `locallife/worker/task_process_payment.go:54`, `locallife/worker/task_process_payment.go:646`, `locallife/worker/task_process_payment.go:724`, `locallife/worker/task_process_payment.go:728`, `locallife/worker/task_process_payment_notify_rider_test.go:121`, `weapp/miniprogram/pages/rider/_main_shared/utils/websocket.ts:14`, `weapp/miniprogram/pages/rider/_main_shared/utils/websocket.ts:17`.

## SQL And Durable State Boundaries

- `delivery_pool`: available order pool, expiration, pickup/dropoff geometry, fee data, and concurrency lock for grab.
- Redis/WebSocket channels `notification:rider:<id>`: realtime pool-new/pool-gone delivery visibility and replay, not durable assignment truth.
- `deliveries`: rider assignment and state machine truth (`pending/assigned/picking/picked/delivering/delivered/cancelled` style states).
- `orders`: customer order status, fulfillment status, food-safety progression guard, and `courier_accepted/picked/delivering/rider_delivered` sync.
- `order_status_logs`: durable audit of rider delivery transitions.
- `riders`: online/status/deposit/frozen deposit and completion stats.
- `rider_deposits`: freeze/unfreeze logs tied to delivery/order deposit movement.
- `profit_sharing_orders`: rider bill row updated during grab to bind rider Baofu sharing member.
- `rider_locations`: latest/fresh location source for delivery confirmation and track/latest-location readers.
- `delivery_timeout_alerts`: pending-dispatch alert dedupe ledger for the 3-minute scheduler; operator action closure is outside this rider slice.
- `payment_orders` and refund tasks: stale pending delivery cancellation can enqueue external payment refund recovery after `CancelOrderTx`.

## Trust, Authorization, And Tenant Checks

- All rider actions resolve the rider by authenticated user id.
- Status transitions reload delivery and require `delivery.rider_id == rider.id`.
- Detail/list APIs for active/history scope to the current rider id.
- Customer-facing `GET /v1/delivery/order/:order_id` has separate order ownership semantics; rider pages should prefer rider-scoped active/history/detail data where possible.
- Broadcast and notification side effects are derived from the order/delivery loaded server-side, not client-supplied actors.
- New-order push recipients are looked up from nearby online rider SQL; replay filtering rechecks `delivery_pool` for grab removals and auto-cancel pool cleanup.
- Navigation route planning is authenticated but not rider-scoped to a delivery id; it accepts coordinates and should be treated as map-provider access, not delivery authorization.

## Idempotency And Duplicate-Submit Checks

- Grab is concurrency-safe through `GetDeliveryPoolByOrderIDForUpdate` and pool row removal.
- Realtime `delivery_pool_new` is read-side only; duplicate messages increment the local badge but do not mutate backend state. Manual refresh/recommend is the recovery path.
- Delivery state transitions are conditional on current status and assigned rider.
- Confirm pickup/delivery are not idempotent from a stale state; repeated calls after transition return state errors.
- Order status log writes skip duplicate final target in some branches, while delivery status updates remain the main guard.
- Frontend has local action loading flags for grab and delivery buttons.
- Pending-dispatch alerts dedupe through `delivery_timeout_alerts`; 20-minute delayed-delivery marking dedupes through `deliveries.is_delayed`.
- Auto-cancel timeout uses `CancelOrderTx` plus `UpdateDeliveryToCancelled`; `CancelOrderTx` removes the pool row so recommend no longer exposes a cancelled pending order after transaction commit.

## Recovery And Async Convergence Paths

- Order pooled/new broadcast and grab order-gone broadcast are best-effort; delivery pool SQL remains canonical.
- Grab sends merchant/customer notifications and order-gone broadcast best-effort after the core transaction.
- Delivery estimate recalculation after grab is best-effort and does not roll back assignment.
- Map route planning can fail independently; navigation page falls back to straight-line/default route display while track/latest-location remain available.
- Geofence auto transitions call the same delivery logic and can be recovered by manual buttons.
- Pending-dispatch timeout side effects are best-effort; the rider-facing source of truth remains `delivery_pool` plus delivery/order status.
- Auto-cancel removes the SQL pool row and publishes a best-effort `delivery_pool_gone` event to online active riders inside the recommendation-visible radius; history/active/recommend refresh remains the durable recovery path if Pub/Sub or the client connection is unavailable.
- History/active list refresh is the frontend recovery path after network ambiguity.

## Frontend Draft And Backend Rehydration

- Order hall and dashboard reload active deliveries after grab and switch to the "my" tab.
- New-order realtime only changes dashboard local badge; user refreshes the hall to rehydrate recommended orders from backend pool truth.
- Task detail/navigation rehydrate delivery status and location/track data from backend routes.
- History page uses request sequence guards to ignore stale paginated responses.

## Test Coverage Signals

Observed tests:

- `locallife/logic/delivery_grab_test.go` covers rider eligibility, distance, expired pool, suspended rider, Baofu readiness, order status, deposit, and rider bill branches.
- `locallife/logic/delivery_status_test.go` covers pickup/delivery transitions, merchant-not-ready block, location missing/stale/radius validation, and success paths.
- `locallife/db/sqlc/delivery_test.go` covers delivery transactions including complete delivery and auto-offline behavior.
- `locallife/api/delivery_test.go` covers API transition and list/detail behavior.
- `locallife/api/location_test.go` covers `/v1/location/direction/bicycling` response mapping.
- `locallife/logic/delivery_broadcast_test.go` and `locallife/worker/task_process_payment_notify_rider_test.go` cover nearby rider broadcast/new-order payload paths.

Missing high-value tests:

- Broader Mini Program contract coverage ensuring all old `DeliveryTaskManagementService` methods stay backed by registered routes.
- Broader Mini Program/WebSocket contract coverage that backend new-order event type remains `delivery_pool_new` across primary and fallback paths.
- End-to-end grab -> active delivery -> location upload -> confirm delivery -> deposit unfreeze.
- Notification/broadcast failure does not affect delivery transaction response.
- Broader scheduler contract test for pending delivery timeout: 3-minute alert dedupe, 20-minute `is_delayed` marking, 60-minute cancel/refund, and rider delivery-pool removal/broadcast semantics.

## Gaps And Refactor Notes

- Consider naming cleanup around the older delivery-task-management wrapper now that its detail method is backed by `GET /v1/delivery/:delivery_id`.
- Keep scheduler auto-cancel `delivery_pool_gone` best-effort and transaction-external; do not let realtime publish failure roll back cancellation/refund recovery.

## Dead And Orphan Paths

- Resolved: `weapp/miniprogram/pages/rider/_api/delivery-task-management.ts:172` now has a backend-compatible `GET /v1/delivery/:delivery_id` route with order-owner/assigned-rider authorization.
- Resolved: worker fallback new-order push now emits `delivery_pool_new`, matching current Mini Program listeners.
- Resolved: `CancelOrderTx` now removes the matching `delivery_pool` row, so scheduler cancellation no longer leaves a cancelled order readable by recommend after transaction commit.
- Resolved: scheduler auto-cancel emits best-effort `delivery_pool_gone` realtime invalidation for already-open rider clients after successful cancellation.

## Branch Exhaustion

- Entry branches checked: dashboard/order hall hall tab, realtime new/gone order events, active delivery tab, task detail, navigation, delivery history, grab action, status transition buttons, old/new delivery API wrappers.
- Request branches checked: recommend, grab, active, history, start-pickup, confirm-pickup, start-delivery, confirm-delivery, order detail by order id, order detail by delivery id, track, latest rider location, and authenticated map route planning.
- Backend state branches checked: not rider, offline, not active, suspended, missing Baofu account, missing/expired pool, too far, missing delivery/order, disallowed order status, cancelled order cleanup, insufficient deposit, rider bill missing/non-pending, no nearby riders for push, stale replay pool row gone, invalid map coordinates/provider unavailable, food-safety pause, merchant not ready, wrong assigned rider, wrong status, missing/stale location, too far from dropoff, successful completion.
- Async branches checked: order-pooled broadcast, payment-worker new-order notify, pending-dispatch alert scheduler, delayed-delivery marking scheduler, 60-minute stale-delivery cancel/refund scheduler, estimate recalculation, notifications, broadcast order removal, geofence auto-advance via separate slice, frontend refresh/reconciliation.
- Dead/orphan branches checked: previously missing generic delivery detail route, previously unconsumed fallback event type, previously stale `delivery_pool` row risk, and scheduler auto-cancel realtime invalidation are now resolved.
