# Rider Workbench Status Location Slice

Status: rider-state flow slice created 2026-06-08
Risk class: G3 - rider eligibility, online/offline acceptance, current region, deposit threshold, Baofu readiness, live GPS, active-delivery guard, geofence auto transitions
Scope: Mini Program rider dashboard/order hall/navigation live runtime -> rider me/status/workbench/location APIs -> rider/delivery/location SQL -> geofence event and auto-advance logic

## Variant Coverage

This slice covers:

- Rider dashboard and order-hall runtime loading of workbench, rider status, current region, active deliveries, available orders, and latest claim hints.
- Rider online/offline switch and current-region sync.
- Live delivery location session, queued GPS uploads, active delivery id binding, and navigation refresh.
- Backend `/v1/rider/me`, `/status`, `/online`, `/offline`, `/current-region`, `/location`, and `/workbench/summary`, with `GET /v1/rider/me` modeled as the profile truth reader rather than just an overview side call.
- Dashboard notification entry into `/pages/notification/index?mode=rider`, generic notification list/unread/read-all/delete APIs, and rider-mode frontend category filtering.
- Geofence arrive/dwell event creation and optional auto confirm pickup/delivery.

This slice does not fully cover:

- Delivery grab and delivery state APIs themselves; they are covered by `rider-delivery-lifecycle`.
- Deposit recharge/withdrawal ledger changes; they are covered by `rider-deposit`.
- Baofu settlement onboarding internals; they are covered by `rider-income-and-baofu-withdrawal`.

## Product Invariant

Rider workbench state must be derived from durable rider and delivery truth:

- The rider may go online only with a valid current region, eligible rider status, available deposit above the region threshold, and Baofu settlement payment readiness.
- The rider may go offline only when there are no active deliveries.
- Location uploads are accepted only from online riders; supplied `delivery_id` must be the current active delivery.
- Latest rider location updates the rider profile and appends immutable location samples.
- Geofence automation is best-effort and must not bypass the normal delivery status transition guards.

## Primary Forward Chain

1. Rider Mini Program entries include dashboard, order hall, navigation, task detail, and tasks pages in the rider subpackage.
   Evidence: `weapp/miniprogram/app.json:12`, `weapp/miniprogram/app.json:16`, `weapp/miniprogram/app.json:24`, `weapp/miniprogram/app.json:25`, `weapp/miniprogram/app.json:26`.

2. Dashboard/order-hall pages share `rider-dashboard-runtime`, which loads dashboard overview, status, active deliveries, recommended orders, and latest claim hints.
   Evidence: `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:294`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:344`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:364`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:383`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:402`.

3. Frontend rider API wrappers call `GET /v1/rider/me`, `GET /status`, `PATCH /current-region`, `POST /online`, `POST /offline`, and `POST /location`.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:122`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:126`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:130`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:138`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:146`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:186`.

4. `GET /v1/rider/me` resolves the authenticated user to a rider and returns rider profile/status/deposit/location/stat fields from `riders`; missing rider is a 404 and does not create a rider record.
   Evidence: `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:299`, `locallife/api/server.go:1132`, `locallife/api/rider.go:274`, `locallife/api/rider.go:277`, `locallife/api/rider.go:280`, `locallife/api/rider.go:287`.

5. Workbench wrapper calls `/v1/rider/workbench/summary` and maps degraded backend components into dashboard view state.
   Evidence: `weapp/miniprogram/pages/rider/_api/rider-workbench.ts:119`, `weapp/miniprogram/pages/rider/_services/rider-workbench.ts:72`, `weapp/miniprogram/pages/rider/_services/rider-workbench.ts:109`, `weapp/miniprogram/pages/rider/_services/rider-workbench.ts:130`.

6. The dashboard notification card navigates to the shared notification page in rider mode; that page pages through `/v1/notifications`, filters rider categories on the client, reads unread count, marks a single notification read, marks all read, and deletes owned notifications.
   Evidence: `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:839`, `weapp/miniprogram/pages/notification/index.ts:191`, `weapp/miniprogram/pages/notification/index.ts:198`, `weapp/miniprogram/pages/notification/index.ts:202`, `weapp/miniprogram/pages/notification/index.ts:259`, `weapp/miniprogram/pages/notification/index.ts:294`, `weapp/miniprogram/pages/notification/index.ts:320`, `weapp/miniprogram/api/notification.ts:50`, `weapp/miniprogram/api/notification.ts:74`, `weapp/miniprogram/api/notification.ts:81`, `weapp/miniprogram/api/notification.ts:95`.

7. Backend notification routes are generic authenticated user routes; SQL scopes list/read/delete operations by `notifications.user_id`, while rider-mode routing to claims/deposit/income/tasks is frontend-only category mapping.
   Evidence: `locallife/api/server.go:1262`, `locallife/api/server.go:1264`, `locallife/api/server.go:1265`, `locallife/api/server.go:1266`, `locallife/api/server.go:1267`, `locallife/api/server.go:1268`, `locallife/api/notification.go:182`, `locallife/api/notification.go:189`, `locallife/api/notification.go:207`, `locallife/api/notification.go:268`, `locallife/api/notification.go:305`, `locallife/api/notification.go:345`, `locallife/api/notification.go:382`, `locallife/db/query/notification.sql:19`, `locallife/db/query/notification.sql:33`, `locallife/db/query/notification.sql:60`.

8. The dashboard toggle first refreshes latest rider status, rejects blocked transitions in the UI, then calls online/offline routes and refreshes location/workbench state.
   Evidence: `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:612`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:616`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:624`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:655`, `weapp/miniprogram/pages/rider/_utils/rider-dashboard-runtime.ts:676`.

9. Backend status/location routes are registered under `/v1/rider`.
   Evidence: `locallife/api/server.go:1132`, `locallife/api/server.go:1133`, `locallife/api/server.go:1154`, `locallife/api/server.go:1157`, `locallife/api/server.go:1158`, `locallife/api/server.go:1159`, `locallife/api/server.go:1162`.

10. Current region sync validates the region and writes `riders.region_id`; status/deposit endpoints reject missing current region.
   Evidence: `locallife/api/rider.go:187`, `locallife/api/rider.go:230`, `locallife/api/rider.go:249`, `locallife/api/rider.go:759`, `locallife/api/rider.go:783`, `locallife/db/query/rider.sql:155`.

11. `GET /v1/rider/status` loads rider, active deliveries, region deposit threshold, Baofu readiness, location fields, and derives `offline/online/delivering`.
   Evidence: `locallife/api/rider.go:759`, `locallife/api/rider.go:790`, `locallife/api/rider.go:798`, `locallife/api/rider.go:803`, `locallife/api/rider.go:816`, `locallife/api/rider.go:830`.

12. Status blocks online when suspended, status is not online-eligible, deposit is below threshold, or Baofu settlement readiness is not payment-ready.
   Evidence: `locallife/api/rider.go:831`, `locallife/api/rider.go:834`, `locallife/api/rider.go:837`, `locallife/api/rider.go:840`, `locallife/api/rider_baofu_readiness.go:10`.

13. `POST /v1/rider/online` syncs the selected region, checks eligible status, threshold, and Baofu readiness, then idempotently writes `is_online=true`.
    Evidence: `locallife/api/rider.go:863`, `locallife/api/rider.go:882`, `locallife/api/rider.go:891`, `locallife/api/rider.go:897`, `locallife/api/rider.go:911`, `locallife/api/rider.go:922`.

14. `POST /v1/rider/offline` loads active deliveries, blocks if any active delivery exists, returns idempotently if already offline, then writes `is_online=false`.
    Evidence: `locallife/api/rider.go:948`, `locallife/api/rider.go:966`, `locallife/api/rider.go:973`, `locallife/api/rider.go:980`, `locallife/api/rider.go:984`.

15. Live location session tracks only a current active delivery, queues points, retries network failures, and uploads with `delivery_id` and source.
    Evidence: `weapp/miniprogram/pages/rider/_utils/rider-live-location.ts:124`, `weapp/miniprogram/pages/rider/_utils/rider-live-location.ts:150`, `weapp/miniprogram/pages/rider/_utils/rider-live-location.ts:158`, `weapp/miniprogram/pages/rider/_utils/rider-live-location.ts:321`, `weapp/miniprogram/pages/rider/_utils/rider-live-location.ts:345`.

16. Backend location upload clamps future timestamps to server time, rejects points older than one hour, requires online rider, and guards supplied delivery id against the current active delivery.
    Evidence: `locallife/api/rider.go:1028`, `locallife/api/rider.go:1040`, `locallife/api/rider.go:1045`, `locallife/api/rider.go:1062`, `locallife/api/rider.go:1070`, `locallife/api/rider.go:1080`.

17. Accepted locations are inserted into `rider_locations`, latest location is written back to `riders`, current region is synced, and geofence processing runs when an active delivery exists.
    Evidence: `locallife/api/rider.go:1075`, `locallife/api/rider.go:1119`, `locallife/api/rider.go:1127`, `locallife/api/rider.go:1136`, `locallife/api/rider.go:1145`, `locallife/db/query/rider_location.sql:2`, `locallife/db/query/rider.sql:47`.

18. Geofence processing loads the delivery, checks rider ownership, maps status to pickup/dropoff targets, skips bad accuracy/out-of-radius points, writes arrive/dwell events, and optionally auto-advances.
    Evidence: `locallife/api/rider_location_events.go:24`, `locallife/api/rider_location_events.go:31`, `locallife/api/rider_location_events.go:36`, `locallife/api/rider_location_events.go:40`, `locallife/api/rider_location_events.go:45`, `locallife/api/rider_location_events.go:49`, `locallife/api/rider_location_events.go:56`, `locallife/api/rider_location_events.go:65`.

19. Dwell detection requires enough recent samples in the configured radius and accuracy window before auto pickup/dropoff can fire.
    Evidence: `locallife/api/rider_location_events.go:162`, `locallife/api/rider_location_events.go:167`, `locallife/api/rider_location_events.go:177`, `locallife/api/rider_location_events.go:182`, `locallife/api/rider_location_events.go:210`.

20. Auto confirm pickup/delivery delegates to delivery logic, so food-readiness and confirm-radius guards still apply.
    Evidence: `locallife/api/rider_location_events.go:259`, `locallife/api/rider_location_events.go:287`, `locallife/logic/delivery_geofence.go:69`, `locallife/logic/delivery_geofence.go:118`, `locallife/logic/delivery_geofence.go:123`.

21. Workbench service degrades non-critical subqueries independently for active deliveries, deposit threshold, pool, daily summary, income, claims, and notifications.
    Evidence: `locallife/logic/rider_workbench.go:175`, `locallife/logic/rider_workbench.go:205`, `locallife/logic/rider_workbench.go:213`, `locallife/logic/rider_workbench.go:233`, `locallife/logic/rider_workbench.go:247`, `locallife/logic/rider_workbench.go:261`, `locallife/logic/rider_workbench.go:286`.

## SQL And Durable State Boundaries

- `riders`: online status, current region, current latitude/longitude, location update timestamp, status, deposit/frozen deposit, and stats.
- `rider_locations`: append-only GPS samples, optional delivery id, accuracy/speed/heading, and recorded timestamp.
- `deliveries`: active delivery status and rider assignment used for offline/location/geofence guards.
- Delivery location events table/query surface: durable arrive/dwell dedupe evidence created by geofence processing.
- `region_rule_configs`, `operators`, `platform_configs`: effective rider deposit threshold inputs.
- `baofu_account_bindings`: rider settlement account readiness consumed by online/grab gates.
- `notifications`: generic user notification rows used by rider-mode notification center for read/unread/delete and related-page navigation.

## Trust, Authorization, And Tenant Checks

- Rider routes use authenticated user id and server-side `GetRiderByUserID`.
- Rider middleware is required only for Baofu settlement/income money routes; status/location routes still resolve rider explicitly.
- `GET /v1/rider/me` and generic notification APIs both scope by authenticated user id; notification category routing is not an authorization boundary.
- Location `delivery_id` is optional, but when present it must match the first current active delivery assigned to the rider.
- Geofence processing rechecks delivery rider id before writing events or auto-advancing.

## Idempotency And Duplicate-Submit Checks

- Online is idempotent when `is_online=true`.
- Offline is idempotent when already offline, but blocked by any active delivery.
- Notification list/unread reads are idempotent; mark-read returns 404 for already-read or non-owned rows, mark-all-read is idempotent at user scope, delete is scoped by user id.
- Location upload appends each accepted point; it is not deduped by timestamp/client id.
- Geofence event creation is deduped per event type/source boundary in backend helper logic.
- Auto-advance calls are guarded by delivery status checks and will no-op/fail safely on stale statuses.

## Recovery And Async Convergence Paths

- Frontend retries live location uploads through a local queue while online/tracking.
- Workbench summary degrades non-critical sections rather than failing the entire dashboard.
- Notification center failures are isolated to the shared notification page; dashboard workbench only shows unread summary from backend workbench degraded components.
- Geofence auto-advance is best-effort; manual delivery buttons remain the recovery path.
- Region can be resynced by explicit current-region PATCH or by a successful location upload.

## Frontend Draft And Backend Rehydration

- The dashboard derives UI status from backend status/workbench responses and refreshes after online/offline/location operations.
- Rider notification center rehydrates list/unread state from `/v1/notifications`; rider category tabs are client-side filters over generic user notifications.
- Live location session starts only when a trackable active delivery exists and stops on no active delivery.
- Navigation page reads latest rider location, track points, and client route independently.

## Test Coverage Signals

Observed tests:

- `locallife/api/rider_test.go` covers rider status, online/offline, deposit threshold, and location/geofence branches.
- `locallife/api/rider_workbench_test.go` covers workbench summary/degraded component behavior.
- `locallife/api/rider_location_events_test.go` and `locallife/logic/delivery_geofence_test.go` cover dwell, accuracy, target mapping, and auto-confirm behavior.

Missing high-value tests:

- Client live-location queue replay after network recovery with active delivery switch.
- Multiple active deliveries: location upload currently binds to the first active delivery when no delivery id is supplied; product should confirm expected behavior.
- Notification center rider-mode pagination when a filtered category is sparse across many generic pages.
- End-to-end geofence auto-advance from real location samples through delivery state update and frontend refresh.

## Gaps And Refactor Notes

- Consider adding a client idempotency key for location batches if duplicate samples cause downstream analytics noise.
- Clarify product behavior for multiple active deliveries and which one should receive unsupplied location points.
- Consider a backend rider-notification category filter if client-side scanning across generic notifications becomes slow or misses sparse categories under pagination.
- `getRiderStatus` online block reason for non-eligible non-suspended status currently uses a deposit-like message; copy and status mapping should be reviewed.

## Branch Exhaustion

- Entry branches checked: dashboard load, order-hall load, pull refresh, online toggle, offline toggle, region sync, live tracking start/stop, upload retry, active delivery switch, navigation latest/track refresh, notification-center rider mode, rider notification tap routing.
- Request branches checked: `/v1/rider/me`, `/current-region`, `/workbench/summary`, `/status`, `/online`, `/offline`, `/location`, `/v1/notifications`, `/v1/notifications/unread/count`, `/v1/notifications/:id/read`, `/v1/notifications/read-all`, `/v1/notifications/:id`.
- Backend state branches checked: no rider, no region, suspended, non-eligible status, insufficient deposit, missing Baofu readiness, already online, already offline, active deliveries, notification row not owned/already read, future timestamp clamp, older-than-one-hour reject, offline location reject, wrong delivery id reject, geofence accuracy/radius/dwell gates.
- Async branches checked: live upload queue/retry, workbench degraded reads, notification push/list rehydration, geofence best-effort auto pickup/delivery.
- Dead/orphan branches checked: no dead backend status/location route found in this slice.
