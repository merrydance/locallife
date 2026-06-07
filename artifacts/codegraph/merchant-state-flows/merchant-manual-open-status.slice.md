# Merchant Manual Open Status Slice

Status: second merchant-state flow slice
Risk class: G2 - merchant availability is a user-triggered status transition, has multiple writers, and affects ordering availability
Scope: merchant dashboard switch -> manual status API -> durable `is_open`/`auto_close_at` state -> websocket refresh

## Variant Coverage

This slice covers:

- Merchant dashboard manual open/close switch.
- Flutter merchant App working-status switch and "立即上线营业" action.
- `GET /v1/merchants/me/status` dashboard status read.
- `PATCH /v1/merchants/me/status` manual status mutation.
- Food-safety suspension and Baofoo readiness gates before opening.
- Websocket publish after manual status mutation.
- The optional `auto_close_at` request/response contract.

This slice does not cover:

- Business-hours automatic open/close scheduler, except as a second writer to `merchants.is_open`.
- Order creation rejection when the merchant is closed.
- Public merchant search/list display of `is_open`.
- Product precedence between manual status changes and `auto_open_by_business_hours=true`.

## Product Invariant

When a merchant manually opens or closes the store, the backend must decide whether the transition is allowed and then write durable merchant state. Opening must be blocked when the merchant is suspended or cannot receive payments. Closing should always be allowed for the resolved merchant. Dashboard state should refresh from backend truth or websocket-triggered reloads.

If `auto_close_at` is accepted as a contract, a runtime scheduler must eventually close the merchant at that time. Fixed 2026-06-07: the existing merchant open-status scheduler now calls `AutoCloseMerchants`, reads back changed rows, and publishes `merchant_status_change` with source `auto_close`.

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

8. `updateMerchantOpenStatus` binds `is_open` and optional `auto_close_at`, resolves the current merchant, blocks suspended merchants, and requires Baofoo payment readiness before opening.
   Evidence: `locallife/api/merchant.go:488`, `locallife/api/merchant.go:514`, `locallife/api/merchant.go:525`, `locallife/api/merchant.go:538`, `locallife/api/merchant.go:549`.

9. If `auto_close_at` is provided while opening, the handler parses it as RFC3339 and rejects past times.
   Evidence: `locallife/api/merchant.go:560`, `locallife/api/merchant.go:563`, `locallife/api/merchant.go:568`.

10. The handler writes `merchants.is_open` and `merchants.auto_close_at` through `UpdateMerchantIsOpen`.
    Evidence: `locallife/api/merchant.go:576`, `locallife/db/query/merchant.sql:459`.

11. After the write, the handler publishes a websocket merchant status change with source `manual` and returns the updated state.
    Evidence: `locallife/api/merchant.go:586`, `locallife/api/merchant.go:588`, `locallife/api/merchant.go:604`.

12. Dashboard updates local state from the PATCH response and reinitializes websocket listeners.
    Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:514`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:521`.

13. Dashboard websocket listener responds to merchant status changes by refreshing dashboard data from backend APIs.
    Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:139`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:143`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:152`.

14. Flutter merchant App also reads and writes the same status contract through `WorkingStatusNotifier.syncFromBackend` and `setStatus`.
    Evidence: `merchant_app/lib/features/order/working_status_provider.dart:54`, `merchant_app/lib/features/order/working_status_provider.dart:68`, `merchant_app/lib/features/order/working_status_provider.dart:84`, `merchant_app/lib/features/order/working_status_provider.dart:98`.

15. The App order list switch and offline empty-state button both call `setStatus`; successful open triggers order fetch, while close clears orders through the status listener.
    Evidence: `merchant_app/lib/features/order/order_list_page.dart:38`, `merchant_app/lib/features/order/order_list_page.dart:43`, `merchant_app/lib/features/order/order_list_page.dart:45`, `merchant_app/lib/features/order/order_list_page.dart:106`, `merchant_app/lib/features/order/order_list_page.dart:110`, `merchant_app/lib/features/order/order_list_page.dart:405`, `merchant_app/lib/features/order/order_list_page.dart:411`.

## Reverse-Reference Findings

- `merchants.is_open` is also written by `SyncMerchantOpenStatusByBusinessHours`, so manual open/close can be overwritten by automatic business-hours mode.
- Fixed 2026-06-07: `AutoCloseMerchants` mutates `is_open=false` where `auto_close_at <= now()` and is now called by `MerchantOpenStatusScheduler` before business-hours sync.
- `GET /status` includes settlement readiness, while `PATCH /status` only returns open status fields and message.
- Kitchen page subscribes to merchant status websocket and also reads `GET /status`, so it is an important downstream reader.
- Flutter merchant App uses the same backend truth to gate websocket connection, polling, foreground service, and order list visibility. Its copy calls the closed state "离线打烊" and "当前处于打烊状态".

## SQL And Durable State Boundaries

- `merchants.is_open`: durable current merchant availability.
- `merchants.auto_close_at`: optional future close time accepted by PATCH.
- `merchants.auto_open_by_business_hours`: read by `GetMerchantIsOpen`; automatic scheduler uses it to decide whether to control `is_open`.
- `UpdateMerchantIsOpen`: manual writer for `is_open` and `auto_close_at`.
- `AutoCloseMerchants`: timed manual close writer, called by `MerchantOpenStatusScheduler`.
- `SyncMerchantOpenStatusByBusinessHours`: automatic writer for `is_open` and clears `auto_close_at`.

## Trust, Authorization, And Tenant Checks

- Frontend dashboard uses `ensureMerchantConsoleAccess`.
- Backend PATCH is under merchant profile write roles `owner` and `manager`.
- Handler resolves merchant from authenticated user; it does not accept merchant id from the client.
- Opening checks food-safety suspension through merchant profile and payment readiness through Baofoo account/report state.
- Closing does not require payment readiness, which is appropriate because closing reduces exposure.

## Idempotency And Duplicate-Submit Checks

- Frontend blocks duplicate switch writes with `openStatusSubmitting`.
- Flutter App blocks duplicate status writes with `_updateFuture`.
- Backend update is last-write-wins for `is_open` and `auto_close_at`.
- Repeating the same PATCH is effectively idempotent for final state.
- There is no idempotency key or conditional version check.
- Automatic scheduler may race after manual write if automatic mode is enabled.

## Recovery And Async Convergence Paths

- Dashboard handles partial refresh failure by retaining previous trusted status.
- Flutter App collapses status sync requests through `_syncFuture`, ignores stale generations, and resets local working status on logout.
- Manual mutation publishes websocket source `manual`; dashboard reloads on matching merchant id.
- Fixed 2026-06-07: expired `auto_close_at` rows are closed by `MerchantOpenStatusScheduler`, then websocket events are published with source `auto_close`.
- Business-hours scheduler still runs after timed auto-close in the same scheduler pass and can clear `auto_close_at` while syncing automatic state.

## Frontend Draft And Backend Rehydration

- There is no long-lived draft. The switch asks confirmation and immediately writes.
- On success, dashboard uses PATCH response for local `isOpen`.
- In the Flutter App, order receiving is gated by confirmed backend `is_open`; opening fetches orders and closing clears local orders.
- Websocket refresh provides a backend rehydration path after local update.
- On failure, dashboard shows toast and leaves existing state intact.

## Test Coverage Signals

Observed tests:

- API tests block opening when Baofoo account/payment channel is not ready.
- API test verifies GET status includes settlement readiness without leaking contract details.
- API tests verify websocket publish for manual close and manual open with `auto_close_at`.
- Scheduler tests cover expired manual `auto_close_at` auto-close publish and business-hours publish.
- SQLC tests cover `AutoCloseMerchants` closing expired rows, clearing `auto_close_at`, and preserving future auto-close rows.
- Authorization test denies unauthorized PATCH.
- `check:merchant-open-status-cross-client-contract` covers the cross-client Mini Program/App convergence contract: backend `GET/PATCH /v1/merchants/me/status`, shared `is_open` payload/response, Mini Program dashboard wrapper/switch/readback, Flutter `WorkingStatusNotifier.syncFromBackend/setStatus`, Flutter order-list status actions, and Flutter login sync all point at the same backend truth.
- Flutter `working_status_provider_test.dart` and `working_status_sync_manager_test.dart` cover route/payload/readback behavior for App status sync, status writes, stale in-flight responses, and login-time status sync.

Missing high-value tests:

- Test for manual open while `auto_open_by_business_hours` is enabled and scheduler later disagrees.
- Frontend/unit coverage for dashboard switch reverting or preserving state on failed PATCH.

## Gaps And Refactor Notes

- Fixed 2026-06-07: `AutoCloseMerchants` is no longer zombie; it is wired into `MerchantOpenStatusScheduler`.
- Dashboard precheck for payment readiness only runs when `canManageMerchantApplyment` is true; backend still enforces readiness for all opens. This is safe but can lead to a backend-only toast for some roles.
- Manual and automatic writers should have explicit product semantics: does manual override automatic mode, or does automatic mode always win within the next minute?
- The Flutter App's "上线营业后才能接收新订单和断线补单" copy is correct only if backend readiness gates, automatic-business-hours overrides, and App polling/websocket state all remain aligned.

## Branch Exhaustion

- Entry branches checked: Mini Program dashboard switch, dashboard refresh/websocket status handling, kitchen status reader, Flutter App order-list status switch, Flutter offline empty-state open button, App status sync on startup/order receiving, and backend automatic business-hours writer adjacency.
- Request branches checked: `GET/PATCH /v1/merchants/me/status`, frontend Mini Program wrappers, Flutter `WorkingStatusNotifier.syncFromBackend/setStatus`, backend readiness checks, websocket publish, and scheduler-called `AutoCloseMerchants` SQL.
- Backend state branches checked: manual `is_open` write, optional `auto_close_at`, closing path, opening path with food-safety suspension and Baofoo payment readiness, automatic scheduler overwrite, `auto_open_by_business_hours` reader field, and status response readiness fields.
- Async branches checked: manual websocket source `manual`, timed auto-close scheduler source `auto_close`, dashboard reload on matching event, Flutter sync generation suppression, App order fetch on open, App order clear on close, and automatic scheduler source `business_hours`.
- Failure/retry branches checked: duplicate dashboard switch guard, Flutter `_updateFuture` guard, last-write-wins PATCH, failed opening readiness precheck/backend error, partial status refresh retaining trusted data, stale Flutter sync generation, logout reset, and automatic scheduler race after manual write.
- Reader/consumer branches checked: dashboard, kitchen, Flutter order receiving/polling/websocket, public/order availability readers, business-hours scheduler, and settlement readiness prechecks.
- Authorization/tenant branches checked: Mini Program console access, backend owner/manager profile-write middleware, server-side merchant resolution, readiness enforced on backend opening for all roles, and close allowed without payment readiness.
- Zombie/unreachable branches checked: `AutoCloseMerchants` was repaired on 2026-06-07 and is now called by the scheduler; dashboard readiness precheck remains role-gated while backend enforces universally.
- Test-proof gaps checked: existing tests cover readiness gates, status GET readiness fields, websocket publish, auth denial, timed `auto_close_at` scheduler publish, `AutoCloseMerchants` SQL semantics, and cross-client Mini Program/App status-route convergence. Missing proof remains for manual-open versus auto scheduler semantics and dashboard failure-state UI.
