# Merchant Manual Open Status Slice

Status: second merchant-state flow slice
Risk class: G2 - merchant availability is a user-triggered status transition, has multiple writers, and affects ordering availability
Scope: merchant dashboard/App switch -> manual status API -> durable `is_open`/`auto_close_at`/`manual_open_status_until` state -> scheduler convergence -> websocket refresh

## Variant Coverage

This slice covers:

- Merchant dashboard manual open/close switch.
- Flutter merchant App working-status switch and "立即上线营业" action.
- `GET /v1/merchants/me/status` dashboard status read.
- `PATCH /v1/merchants/me/status` manual status mutation.
- Food-safety suspension, Baofoo readiness, and local merchant payment-config gates before opening.
- Websocket publish after manual status mutation.
- The optional `auto_close_at` request/response contract.
- The 2026-06-10 manual-overrides-business-hours precedence through `manual_open_status_until`.

This slice does not cover:

- Business-hours editor UX beyond same-day window validation.
- Order creation rejection when the merchant is closed.
- Public merchant search/list display of `is_open`.

## Product Invariant

When a merchant manually opens or closes the store, the backend must decide whether the transition is allowed and then write durable merchant state. Opening must be blocked when the merchant is suspended or cannot receive payments. Closing should always be allowed for the resolved merchant. Dashboard state should refresh from backend truth or websocket-triggered reloads.

If `auto_close_at` is accepted as a contract, a runtime scheduler must eventually close the merchant at that time. Fixed 2026-06-07: the existing merchant open-status scheduler now calls `AutoCloseMerchants`, reads back changed rows, and publishes `merchant_status_change` with source `auto_close`.

Fixed 2026-06-10: a manual open/close operation temporarily overrides automatic business-hours control until the next business-hours switching point. The manual API persists `merchants.manual_open_status_until` from the database-local business-hours clock; automatic mode resumes at that boundary, expired markers are cleared, and payment-readiness invalidation can still force the merchant closed during the override window. Current business-hours semantics are same-day only; reverse/cross-midnight windows are rejected by backend validation and a database CHECK constraint.

## Primary Forward Chain

1. Dashboard loads with merchant console access, initializes websocket listeners, and calls `loadDashboard`.
   Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:168`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:179`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:209`.

2. `loadDashboard` reads profile and open status concurrently. It prefers `GET /status` for `isOpen`, falls back to previous trusted data or profile `is_open`, then builds the business state view.
   Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:213`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:234`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:264`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:274`.

3. The dashboard WXML renders a TDesign switch bound to `isOpen` and `onOpenStatusSwitchChange`.
   Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.wxml:43`, `weapp/miniprogram/pages/merchant/dashboard/index.wxml:47`, `weapp/miniprogram/pages/merchant/dashboard/index.wxml:52`.

4. `onOpenStatusSwitchChange` blocks when access is not ready or submission is active, asks for confirmation, then calls `updateMerchantStorefrontOpenStatus`.
   Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:471`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:488`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:505`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:508`.

5. Frontend service maps reads and writes to shared API wrappers.
   Evidence: `weapp/miniprogram/pages/merchant/_services/merchant-open-status.ts:14`, `weapp/miniprogram/pages/merchant/_services/merchant-open-status.ts:18`.

6. Frontend API wrappers call `GET /v1/merchants/me/status` and `PATCH /v1/merchants/me/status`.
   Evidence: `weapp/miniprogram/api/merchant.ts:556`, `weapp/miniprogram/api/merchant.ts:567`.

7. Backend routes register status GET and PATCH. PATCH lives under the merchant profile write group protected for owner/manager.
   Evidence: `locallife/api/server.go:669`, `locallife/api/server.go:674`, `locallife/api/server.go:682`, `locallife/api/server.go:684`.

8. `updateMerchantOpenStatus` binds `is_open` and optional `auto_close_at`, resolves the current merchant, blocks suspended merchants, and requires both Baofoo readiness and an active local `merchant_payment_configs.sub_mch_id` before opening.
   Evidence: `locallife/api/merchant_status.go:42`, `locallife/api/merchant_status.go:50`, `locallife/api/merchant_status.go:63`, `locallife/api/merchant_status.go:73`, `locallife/api/merchant_status.go:82`, `locallife/api/merchant_baofu_readiness.go:32`.

9. If `auto_close_at` is provided while opening, the handler parses it as RFC3339 and rejects past times.
   Evidence: `locallife/api/merchant_status.go:84`, `locallife/api/merchant_status.go:86`, `locallife/api/merchant_status.go:91`.

10. The handler computes `manual_open_status_until` from database-local business-hours truth when `auto_open_by_business_hours` is enabled, then writes `merchants.is_open`, `auto_close_at`, and `manual_open_status_until` through `UpdateMerchantIsOpen`.
    Evidence: `locallife/api/merchant_status.go:98`, `locallife/api/merchant_status.go:104`, `locallife/api/merchant_business_hours_override.go:14`, `locallife/db/query/merchant.sql:510`.

11. After the write, the handler publishes a websocket merchant status change with source `manual` and returns the updated state.
    Evidence: `locallife/api/merchant_status.go:115`, `locallife/api/merchant_status.go:117`, `locallife/api/merchant_status.go:132`.

12. Dashboard updates local state from the PATCH response and reinitializes websocket listeners.
    Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:514`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:521`.

13. Dashboard websocket listener responds to merchant status changes by refreshing dashboard data from backend APIs.
    Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:139`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:143`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:152`.

14. Flutter merchant App also reads and writes the same status contract through `WorkingStatusNotifier.syncFromBackend` and `setStatus`.
    Evidence: `merchant_app/lib/features/order/working_status_provider.dart:54`, `merchant_app/lib/features/order/working_status_provider.dart:68`, `merchant_app/lib/features/order/working_status_provider.dart:84`, `merchant_app/lib/features/order/working_status_provider.dart:98`.

15. The App order list switch and offline empty-state button both call `setStatus`; successful open triggers order fetch, while close clears orders through the status listener.
    Evidence: `merchant_app/lib/features/order/order_list_page.dart:38`, `merchant_app/lib/features/order/order_list_page.dart:43`, `merchant_app/lib/features/order/order_list_page.dart:45`, `merchant_app/lib/features/order/order_list_page.dart:106`, `merchant_app/lib/features/order/order_list_page.dart:110`, `merchant_app/lib/features/order/order_list_page.dart:405`, `merchant_app/lib/features/order/order_list_page.dart:411`.

## Reverse-Reference Findings

- `merchants.is_open` is also written by `SyncMerchantOpenStatusByBusinessHours`. Fixed 2026-06-10: the scheduler skips active `manual_open_status_until` rows until the next business-hours switch, except payment-config invalidation still fails closed.
- Fixed 2026-06-07: `AutoCloseMerchants` mutates `is_open=false` where `auto_close_at <= now()` and is now called by `MerchantOpenStatusScheduler` before business-hours sync.
- `GET /status` includes settlement readiness, while `PATCH /status` only returns open status fields and message.
- Kitchen page subscribes to merchant status websocket and also reads `GET /status`, so it is an important downstream reader.
- Flutter merchant App uses the same backend truth to gate websocket connection, polling, foreground service, and order list visibility. Its copy calls the closed state "离线打烊" and "当前处于打烊状态".

## SQL And Durable State Boundaries

- `merchants.is_open`: durable current merchant availability.
- `merchants.auto_close_at`: optional future close time accepted by PATCH.
- `merchants.manual_open_status_until`: durable temporary manual override boundary for automatic business-hours mode.
- `merchants.auto_open_by_business_hours`: read by `GetMerchantIsOpen`; automatic scheduler uses it to decide whether to control `is_open`.
- `GetDatabaseLocalClock`: database-local date/time truth used by the manual API to calculate the next business-hours switching point consistently with scheduler SQL.
- `UpdateMerchantIsOpen`: manual writer for `is_open`, `auto_close_at`, and `manual_open_status_until`.
- `AutoCloseMerchants`: timed manual close writer, called by `MerchantOpenStatusScheduler`.
- `SyncMerchantOpenStatusByBusinessHours`: automatic writer for `is_open`; skips active manual override rows, clears `auto_close_at`/`manual_open_status_until` when automatic truth resumes, and can still force closed when payment config is invalid.
- `ClearExpiredMerchantManualOpenStatusOverrides`: internal cleanup for expired override markers when no visible status transition is needed.
- `merchant_business_hours_same_day_window_chk`: database constraint rejecting non-closed `open_time >= close_time` windows; backend `PUT /business-hours` rejects the same shape before persistence.

## Trust, Authorization, And Tenant Checks

- Frontend dashboard uses `ensureMerchantConsoleAccess`.
- Backend PATCH is under merchant profile write roles `owner` and `manager`.
- Handler resolves merchant from authenticated user; it does not accept merchant id from the client.
- Opening checks food-safety suspension through merchant profile and payment readiness through both Baofoo account/report state and the local active `merchant_payment_configs.sub_mch_id` record.
- Closing does not require payment readiness, which is appropriate because closing reduces exposure.

## Idempotency And Duplicate-Submit Checks

- Frontend blocks duplicate switch writes with `openStatusSubmitting`.
- Flutter App blocks duplicate status writes with `_updateFuture`.
- Backend update is last-write-wins for `is_open` and `auto_close_at`.
- Repeating the same PATCH is effectively idempotent for final state.
- There is no idempotency key or conditional version check.
- Automatic scheduler no longer overwrites a manual write before `manual_open_status_until`; repeated scheduler ticks converge through `IS DISTINCT FROM` plus idempotent expired-marker cleanup.

## Recovery And Async Convergence Paths

- Dashboard handles partial refresh failure by retaining previous trusted status and rendering the stale/partial-sync message in a TDesign warning notice with retry.
- Flutter App collapses status sync requests through `_syncFuture`, ignores stale generations, and resets local working status on logout.
- Manual mutation publishes websocket source `manual`; dashboard reloads on matching merchant id.
- Fixed 2026-06-07: expired `auto_close_at` rows are closed by `MerchantOpenStatusScheduler`, then websocket events are published with source `auto_close`.
- Fixed 2026-06-10: business-hours scheduler still runs after timed auto-close, but active `manual_open_status_until` prevents automatic business-hours overwrite until the next switching point. Payment-config invalidation is deliberately not protected by the override and can close the merchant immediately.

## Frontend Draft And Backend Rehydration

- There is no long-lived draft. The switch asks confirmation and immediately writes.
- On success, dashboard uses PATCH response for local `isOpen`.
- In the Flutter App, order receiving is gated by confirmed backend `is_open`; opening fetches orders and closing clears local orders.
- Websocket refresh provides a backend rehydration path after local update.
- On PATCH failure, dashboard shows an action-level toast and leaves existing `isOpen` intact because local truth is only updated from the backend PATCH response.

## Test Coverage Signals

Observed tests:

- API tests block opening when Baofoo account/payment channel is not ready and when local merchant payment config is inactive.
- API test verifies GET status includes settlement readiness without leaking contract details.
- API tests verify websocket publish for manual close and manual open with `auto_close_at`.
- API tests verify `manual_open_status_until` is persisted until the next business-hours switch, infinity is used when no future switch exists, special dates are treated as calendar dates under non-local time zones, local payment-config inactive opening returns stable 400 without leaking contract/sub-merchant identifiers, and reverse business-hour windows are rejected before persistence.
- Scheduler tests cover expired manual `auto_close_at` auto-close publish and business-hours publish.
- Scheduler tests cover expired manual `auto_close_at` auto-close publish, business-hours publish, and expired override cleanup invocation.
- SQLC tests cover `AutoCloseMerchants` closing expired rows, clearing `auto_close_at`, preserving future auto-close rows, preserving manual override until the business-hours boundary, closing despite manual override when payment config becomes invalid, clearing expired override markers, deleting dirty historical reverse/cross-midnight business-hour rows before adding the CHECK constraint, and rejecting new non-closed reverse business-hour windows.
- Authorization test denies unauthorized PATCH.
- `check:merchant-open-status-cross-client-contract` covers the cross-client Mini Program/App convergence contract: backend `GET/PATCH /v1/merchants/me/status`, shared `is_open` payload/response, Mini Program dashboard wrapper/switch/readback, Flutter `WorkingStatusNotifier.syncFromBackend/setStatus`, Flutter order-list status actions, and Flutter login sync all point at the same backend truth.
- `check:merchant-open-status-dashboard-failure-state` covers the Mini Program dashboard failure-state contract: refresh/partial-sync failures set a visible in-page warning with retry, PATCH failures use Chinese action toast, `openStatusSubmitting` is visible and cleared in `finally`, and `isOpen` is only changed from the backend PATCH response.
- Flutter `working_status_provider_test.dart` and `working_status_sync_manager_test.dart` cover route/payload/readback behavior for App status sync, status writes, stale in-flight responses, and login-time status sync.

Missing high-value tests:

- No remaining high-value backend proof gap for the 2026-06-10 manual-overrides-business-hours precedence. Manager-facing readiness precheck copy remains optional because backend readiness enforcement is authoritative.

## Gaps And Refactor Notes

- Fixed 2026-06-07: `AutoCloseMerchants` is no longer zombie; it is wired into `MerchantOpenStatusScheduler`.
- Fixed 2026-06-09: dashboard stale/partial-refresh failure state is visible in-page, and the status switch failure contract is proof-covered without changing manual/automatic status semantics.
- Fixed 2026-06-10: manual open/close temporarily overrides automatic mode until the next business-hours switching point through durable `manual_open_status_until`; scheduler cleanup, API and scheduler payment-invalid fail-closed behavior, DB-clock alignment, dirty-row migration cleanup, and same-day business-hour validation are proof-covered.
- Dashboard precheck for payment readiness only runs when `canManageMerchantApplyment` is true; backend still enforces readiness for all opens. This is safe but can lead to a backend-only toast for some roles.
- The Flutter App's "上线营业后才能接收新订单和断线补单" copy is correct only if backend readiness gates, automatic-business-hours overrides, and App polling/websocket state all remain aligned.

## Branch Exhaustion

- Entry branches checked: Mini Program dashboard switch, dashboard refresh/websocket status handling, kitchen status reader, Flutter App order-list status switch, Flutter offline empty-state open button, App status sync on startup/order receiving, and backend automatic business-hours writer adjacency.
- Request branches checked: `GET/PATCH /v1/merchants/me/status`, frontend Mini Program wrappers, Flutter `WorkingStatusNotifier.syncFromBackend/setStatus`, backend readiness checks, websocket publish, and scheduler-called `AutoCloseMerchants` SQL.
- Backend state branches checked: manual `is_open` write, optional `auto_close_at`, durable `manual_open_status_until`, closing path, opening path with food-safety suspension, Baofoo readiness, and local payment-config readiness, automatic scheduler overwrite after boundary, payment-invalid fail-closed branch, `auto_open_by_business_hours` reader field, and status response readiness fields.
- Async branches checked: manual websocket source `manual`, timed auto-close scheduler source `auto_close`, dashboard reload on matching event, Flutter sync generation suppression, App order fetch on open, App order clear on close, automatic scheduler source `business_hours`, and internal-only expired override cleanup without websocket publish.
- Failure/retry branches checked: duplicate dashboard switch guard, visible dashboard refresh/partial-sync failure notice with retry, PATCH failure preserving existing `isOpen`, Flutter `_updateFuture` guard, last-write-wins PATCH, failed opening readiness precheck/backend error, local payment-config inactive rejection without sensitive identifier leakage, partial status refresh retaining trusted data, stale Flutter sync generation, logout reset, automatic scheduler race after manual write, DB-clock/special-date calendar handling, dirty historical reverse-row cleanup, reverse business-hour window rejection, and payment-readiness invalidation during manual override.
- Reader/consumer branches checked: dashboard, kitchen, Flutter order receiving/polling/websocket, public/order availability readers, business-hours scheduler, and settlement readiness prechecks.
- Authorization/tenant branches checked: Mini Program console access, backend owner/manager profile-write middleware, server-side merchant resolution, readiness enforced on backend opening for all roles, and close allowed without payment readiness.
- Zombie/unreachable branches checked: `AutoCloseMerchants` was repaired on 2026-06-07 and is now called by the scheduler; dashboard readiness precheck remains role-gated while backend enforces universally.
- Test-proof gaps checked: existing tests cover readiness gates, local payment-config inactive rejection, status GET readiness fields, websocket publish, auth denial, timed `auto_close_at` scheduler publish, `AutoCloseMerchants` SQL semantics, manual override until next switch, no-future-switch infinity fallback, expired marker cleanup, payment-invalid fail-closed behavior, dirty historical reverse-row migration cleanup, same-day business-hour constraint, cross-client Mini Program/App status-route convergence, and dashboard failure-state UI. No remaining high-value backend proof gap for the 2026-06-10 manual-override-until-next-switching-point semantics.
