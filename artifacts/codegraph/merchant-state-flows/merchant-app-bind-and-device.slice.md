# Merchant App Bind And Device Slice

Status: merchant-state flow slice created; App bind merchant-id recheck and one-time verification proof repaired 2026-06-08; consume-after-recheckable-preconditions, platform payload, logout unregister, stale-device cleanup, and native-push terminal-failure degradation boundaries repaired 2026-06-11
Risk class: G3 - binding code exchanges a public one-time credential for long-lived merchant App tokens and later binds push devices used for order alerts
Scope: Mini Program dashboard bind-code popup -> public App bind verification -> App auth token/session storage -> merchant App device registration/heartbeat -> native-push device readers

## Variant Coverage

This slice covers:

- Merchant Mini Program bind-code entry from the dashboard device area.
- Backend authenticated bind-code generation and public bind-code verification.
- Flutter merchant App bind-code login, secure token storage, and refresh behavior.
- Merchant App push device registration, heartbeat, unregister, and downstream push-target reader.

This slice does not fully cover:

- Staff invite/bind codes that use `merchants.bind_code`; those belong to the staff flow.
- Cloud printer device management; that is covered by `merchant-device-display-config`.
- Provider-specific native push SDK delivery internals beyond the LocalLife device registry and dispatcher boundary.

## Product Invariant

The merchant App binding workflow has two distinct truths:

- Binding code truth is short-lived Redis state that should be one-time and should only mint tokens for a still-authorized merchant user.
- Device truth is durable `merchant_app_devices` state created after App login and used later for native order push.
- Fixed/current 2026-06-11: App bind code consumption happens after the recheckable role/user/token/profile preconditions, so late infrastructure failure before the final consume boundary does not burn a valid code. The final Redis consume is an atomic compare-and-delete, and session-insert failure attempts to restore the code with the remaining millisecond TTL without overwriting a newer user-index code or reviving an already-expired code, preserving strict one-time success semantics.

Successful bind-code verification alone does not prove the device can receive native push; device registration and heartbeat must converge separately.

## Primary Forward Chain

1. Merchant dashboard exposes `绑定商户端App` as a device-management entry.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:178`, `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:204`.

2. Tapping the entry opens a center popup, resets local bind-code state, and calls `createMerchantAppBindCode`.
   Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:379`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:391`, `weapp/miniprogram/pages/merchant/dashboard/index.wxml:94`, `weapp/miniprogram/pages/merchant/dashboard/index.wxml:110`.

3. The Mini Program wrapper maps bind-code generation to `POST /v1/auth/app-bind/code`.
   Evidence: `weapp/miniprogram/pages/merchant/_services/merchant-app-bind.ts:1`, `weapp/miniprogram/pages/merchant/_services/merchant-app-bind.ts:3`, `weapp/miniprogram/api/auth.ts:214`, `weapp/miniprogram/api/auth.ts:216`.

4. Backend registers bind-code generation under authenticated routes and bind-code verification under the public auth group.
   Evidence: `locallife/api/server.go:521`, `locallife/api/server.go:626`.

5. Code generation requires Redis, rate-limits per user to three attempts per minute, and reads user roles to find a merchant-related role.
   Evidence: `locallife/api/app_bind.go:92`, `locallife/api/app_bind.go:102`, `locallife/api/app_bind.go:115`, `locallife/api/app_bind.go:121`, `locallife/api/app_bind.go:128`.

6. Generated code state is Redis-only: `app_bind:<code>` stores `userID:merchantID`, and `app_bind:user:<userID>` allows reuse of a still-valid code.
   Evidence: `locallife/api/app_bind.go:30`, `locallife/api/app_bind.go:38`, `locallife/api/app_bind.go:49`, `locallife/api/app_bind.go:59`, `locallife/api/app_bind.go:60`, `locallife/api/app_bind.go:133`.

7. Mini Program popup displays the code, starts a local countdown from backend `expires_in`, and can copy or regenerate the code.
   Evidence: `weapp/miniprogram/pages/merchant/dashboard/index.ts:392`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:398`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:418`, `weapp/miniprogram/pages/merchant/dashboard/index.ts:422`, `weapp/miniprogram/pages/merchant/dashboard/index.wxml:111`, `weapp/miniprogram/pages/merchant/dashboard/index.wxml:115`.

8. Flutter App login page accepts a six-digit code and calls `AuthNotifier.loginWithBindingCode`; duplicate in-flight submits are collapsed in the notifier.
   Evidence: `merchant_app/lib/features/auth/bind_code_page.dart:40`, `merchant_app/lib/features/auth/bind_code_page.dart:43`, `merchant_app/lib/features/auth/auth_provider.dart:185`, `merchant_app/lib/features/auth/auth_provider.dart:190`, `merchant_app/test/auth_notifier_test.dart:9`.

9. Flutter `AuthService.verifyBindingCode` reuses a local `device_uuid`, collects device metadata, and calls public `POST /auth/app-bind/verify`.
   Evidence: `merchant_app/lib/features/auth/auth_service.dart:50`, `merchant_app/lib/features/auth/auth_service.dart:60`, `merchant_app/lib/features/auth/auth_service.dart:93`, `merchant_app/lib/features/auth/auth_service.dart:97`, `merchant_app/lib/features/auth/auth_service.dart:98`.

10. Backend verification reads Redis by code, rejects missing/expired code, parses `userID:merchantID`, and completes role/user/token/profile preconditions before final consumption.
    Evidence: `locallife/api/app_bind.go:355`, `locallife/api/app_bind.go:356`, `locallife/api/app_bind.go:372`, `locallife/api/app_bind.go:374`, `locallife/api/app_bind.go:382`, `locallife/api/app_bind.go:405`, `locallife/api/app_bind.go:412`, `locallife/api/app_bind.go:445`.

11. Verification rechecks that the user still has an active merchant role for the same `merchantID` embedded in the Redis code payload, consumes the code with an atomic Redis compare-and-delete only after recheckable preconditions, and preserves one-time behavior for duplicate successful consumption.
    Evidence: `locallife/api/app_bind.go:382`, `locallife/api/app_bind.go:388`, `locallife/api/app_bind.go:395`, `locallife/api/app_bind.go:451`, `locallife/api/app_bind.go:456`, `locallife/api/app_bind.go:482`.

12. Backend stores a normal session row but prefixes the session user agent with `app-bind:` so refresh keeps the long-lived App refresh duration. If session creation fails after final consume, the handler attempts to restore the Redis code with the consumed key's remaining TTL, skips restoration when the user index already points at a newer code, and logs restore failure separately.
    Evidence: `locallife/api/app_bind.go:461`, `locallife/api/app_bind.go:468`, `locallife/api/app_bind.go:471`, `locallife/api/app_bind.go:472`, `locallife/api/token.go:71`, `locallife/api/token.go:72`.

13. Flutter saves access/refresh tokens and merchant display name into secure storage and routes into the authenticated App shell.
    Evidence: `merchant_app/lib/features/auth/auth_provider.dart:203`, `merchant_app/lib/features/auth/auth_provider.dart:208`, `merchant_app/lib/features/auth/auth_provider.dart:209`, `merchant_app/lib/features/auth/auth_service.dart:32`, `merchant_app/lib/features/auth/auth_service.dart:37`.

14. After authentication, the App attempts native push device registration through `DeviceSyncService.ensureRegistered`.
    Evidence: `merchant_app/lib/core/push/push_provider.dart:40`, `merchant_app/lib/core/push/push_provider.dart:42`, `merchant_app/lib/core/network/ws_provider.dart:48`, `merchant_app/lib/core/network/ws_provider.dart:53`.

15. Device registration payload includes the same local `device_uuid`, push token, provider, model, OS, version, and platform, then calls `POST /merchant/device/register`.
    Evidence: `merchant_app/lib/core/push/device_sync_service.dart:127`, `merchant_app/lib/core/push/device_sync_service.dart:173`, `merchant_app/lib/core/push/device_sync_service.dart:176`, `merchant_app/lib/core/push/device_sync_service.dart:324`, `merchant_app/lib/core/push/device_sync_service.dart:327`.

16. Backend merchant App device routes require merchant staff access for owner, manager, cashier, or chef.
    Evidence: `locallife/api/server.go:1007`, `locallife/api/server.go:1008`, `locallife/api/server.go:1010`, `locallife/api/server.go:1011`, `locallife/api/server.go:1012`.

17. Device registration validates merchant/user principal, normalizes platform/provider, and writes `merchant_app_devices` through a transaction that deactivates other active rows with the same push token before upserting by active `device_id`.
    Evidence: `locallife/logic/merchant_app_device.go:62`, `locallife/logic/merchant_app_device.go:64`, `locallife/logic/merchant_app_device.go:76`, `locallife/logic/merchant_app_device.go:80`, `locallife/logic/merchant_app_device.go:97`, `locallife/db/sqlc/tx_merchant_app_device.go:13`, `locallife/db/sqlc/tx_merchant_app_device.go:17`, `locallife/db/query/merchant_app_device.sql:54`.

18. Heartbeat is sent during order polling and through the settings tile; backend updates active device metadata and `last_active_at`.
    Evidence: `merchant_app/lib/core/service/order_poller.dart:87`, `merchant_app/lib/features/settings/settings_page.dart:81`, `merchant_app/lib/core/push/device_sync_service.dart:205`, `merchant_app/lib/core/push/device_sync_service.dart:237`, `locallife/logic/merchant_app_device.go:115`, `locallife/db/query/merchant_app_device.sql:120`, `locallife/db/query/merchant_app_device.sql:127`.

19. Active App logout from the order drawer or settings page goes through `AuthLogoutController`, which attempts to unregister the current backend device before clearing the local auth session. Device unregister failure is best-effort and does not block logout; the local last-registered push-token marker is cleared so a later rebind can re-register the same token.
    Evidence: `merchant_app/lib/features/order/order_list_page.dart:216`, `merchant_app/lib/features/settings/settings_page.dart:199`, `merchant_app/lib/features/auth/auth_logout_controller.dart:6`, `merchant_app/lib/features/auth/auth_logout_controller.dart:25`, `merchant_app/lib/features/auth/auth_logout_controller.dart:35`, `merchant_app/lib/core/push/device_sync_service.dart:312`, `merchant_app/lib/core/push/device_sync_service.dart:323`.

20. `DataCleanupScheduler` deactivates active merchant App devices whose `last_active_at` is more than 90 days stale, so uninstall, storage-wipe, token-expiry, or no-logout cases eventually stop remaining active push targets.
    Evidence: `locallife/scheduler/data_cleanup.go:34`, `locallife/scheduler/data_cleanup.go:175`, `locallife/scheduler/data_cleanup.go:213`, `locallife/db/query/merchant_app_device.sql:10`, `locallife/db/query/merchant_app_device.sql:15`, `locallife/db/sqlc/merchant_app_device_test.go:13`, `locallife/scheduler/merchant_app_device_cleanup_test.go:13`.

21. Native push dispatch reads active merchant App devices by merchant, groups by provider, and sends to configured providers; unconfigured providers are skipped rather than deactivating devices. Retryable provider failures remain in-memory retry signals, while terminal provider failures record sanitized reason/count on only the affected active `merchant_app_devices` row and deactivate that row only after three consecutive terminal failures.
    Evidence: `locallife/logic/merchant_app_push_gateway.go:120`, `locallife/logic/merchant_app_push_gateway.go:135`, `locallife/logic/merchant_app_push_gateway.go:145`, `locallife/logic/merchant_app_push_gateway.go:161`, `locallife/logic/merchant_app_push_gateway.go:168`, `locallife/db/query/merchant_app_device.sql:18`, `locallife/db/query/merchant_app_device.sql:24`, `locallife/db/query/merchant_app_device.sql:33`.

22. Successful native-push send, device registration, and heartbeat clear previous per-device push degradation evidence, so recovered devices do not keep stale terminal-failure count.
    Evidence: `locallife/logic/merchant_app_push_gateway.go:149`, `locallife/db/query/merchant_app_device.sql:37`, `locallife/db/query/merchant_app_device.sql:105`, `locallife/db/query/merchant_app_device.sql:120`, `locallife/db/query/merchant_app_device.sql:128`.

## Reverse-Reference Findings

- App bind codes use Redis `app_bind:*` keys and do not use the legacy `merchants.bind_code` columns. Those columns are used by staff invite binding.
- The public verify endpoint receives `device_id`, model, OS, and App version, but only logs them. It does not persist a device row; device persistence happens later through `/v1/merchant/device/register`.
- Fixed/current 2026-06-11: bind-code verification preserves the Redis code through role recheck, user lookup, token/hash generation, and access-profile construction. The final consume uses atomic compare-and-delete; a session-insert failure attempts to restore the code with its remaining TTL without overriding a newer generated code for the same user.
- Fixed 2026-06-08: the role recheck proves the same `merchantID` embedded in the Redis code payload is still present in the user's current active merchant roles before tokens are minted.
- Fixed/current 2026-06-11: device registry constrains `platform` to `android`, and Flutter device registration/heartbeat now always report backend platform `android` even if a non-Android runtime branch is present for local metadata collection.
- Fixed/current 2026-06-11: active Flutter logout now attempts backend device unregister before clearing local auth, clears the local last-registered push-token marker even when unregister fails, and skips backend DELETE if no local device id exists.
- Fixed/current 2026-06-11: backend stale-device cleanup deactivates active merchant App devices after 90 days without heartbeat, covering uninstall/storage-wipe/no-logout cases without touching merchant, staff user, or App-wide push state.
- Fixed/current 2026-06-11: native push dispatcher persists per-device terminal provider failures. It records sanitized provider reason and consecutive failure count on only the affected active `merchant_app_devices` row/push token, keeps the row active while degraded for the first two terminal failures, and deactivates only that row after the third clear terminal failure. This does not deactivate the merchant account, staff user, or App-wide push capability.

## SQL And Durable State Boundaries

- Redis `app_bind:<code>` and `app_bind:user:<userID>` own short-lived bind-code truth.
- `sessions` owns issued access/refresh token hashes and App-specific refresh behavior through the prefixed user-agent marker.
- `merchant_app_devices` owns durable native-push device registration, active/inactive state, provider token, device metadata, `last_active_at`, and per-device native-push terminal-failure evidence; migration `000265` adds a partial active-device `last_active_at` index for stale cleanup scans, and migration `000266` adds `push_failure_count`, `last_push_failure_reason`, `last_push_failure_at`, and `push_degraded_at`.
- `merchants.bind_code` and `bind_code_expires_at` are not part of App bind truth; they belong to staff invite binding.

## Trust, Authorization, And Tenant Checks

- Bind-code generation requires an authenticated user token and any role in `merchant`, `merchant_owner`, or `merchant_manager`.
- Bind-code verification is public and protected by the public auth sensitive rate limiter plus code TTL.
- Verification rechecks same-merchant active-role presence before minting tokens.
- Device registration/heartbeat/unregister require authenticated merchant staff context for owner, manager, cashier, or chef.
- Device registry writes use the middleware-resolved merchant id, not a client-supplied merchant id.

## Idempotency And Duplicate-Submit Checks

- Generate reuses a still-valid code per user and rate-limits generation.
- Fixed/current 2026-06-11: verify is one-time by final Redis compare-and-delete. Recheckable backend failures before that boundary leave the code replayable, while duplicate successful consumption still rejects as expired/invalid. Session-insert failure after final consume attempts TTL-preserving restoration without replacing a newer user-index code.
- Flutter `AuthNotifier` collapses in-flight bind-code submits.
- Device registration is idempotent by active `device_id`, deactivates other active rows with the same push token in the same transaction, and clears prior native-push terminal-failure evidence for the refreshed device row.
- Heartbeat is repeatable and last-write-wins for mutable device metadata; it also clears prior native-push terminal-failure evidence because the device has proven recent client activity.
- Active logout unregister is best-effort: a successful backend delete deactivates the current `device_id`; a failed delete still clears the local registered-token marker so the next login can force registration instead of silently reusing a stale local marker.
- Stale-device cleanup is a status-only convergence update: rows stay in `merchant_app_devices`, but only `status='active'` rows with `last_active_at` before the scheduler cutoff are marked `inactive` with `unregistered_at` set.
- Native-push terminal failures are scoped by current device row id and push token. The first two terminal failures mark only that row as degraded; the third consecutive terminal failure deactivates only that row. Retryable failures and unconfigured providers do not mutate device status.

## Recovery And Async Convergence Paths

- Mini Program can regenerate a bind code if generation fails or code expires.
- Flutter can retry bind-code login after recheckable backend failures before final consume because the Redis code remains valid. If session creation fails after final consume, backend attempts to restore the code with the remaining TTL unless the user already generated a newer code; if that restore also fails, Mini Program regeneration remains the recovery path.
- Device registration failures surface as degraded state in the App settings/order list, but do not block token login.
- Missing native push token skips registration; polling and websocket still provide other order-reception channels.
- Active logout clears local auth even if device unregister fails. If the unregister request is unavailable or unauthenticated, backend stale-device cleanup now deactivates the active device row after the 90-day no-heartbeat grace window, while the client no longer suppresses the next registration for the same push token.
- Terminal native-push provider failures now converge through per-device degradation first. A later successful provider send, device registration, or heartbeat clears stale degradation evidence; repeated terminal failures deactivate only the affected device/token row.

## Frontend Draft And Backend Rehydration

- Mini Program bind popup state is local only; backend response provides code and countdown seconds.
- Flutter bind page keeps the entered code local and does not persist the code.
- Flutter secure storage persists tokens, merchant name, and local `device_uuid`.
- Device sync state is in-memory `ValueNotifier` state with degradation copy shown in settings/order surfaces.

## Test Coverage Signals

Observed tests:

- Backend tests cover Redis unavailable for generate/verify, regeneration when user index points to a missing code, full generate -> verify -> session creation -> one-time reuse denial, changed-merchant-role rejection before token minting, no code burn when user lookup fails, TTL-preserving code restoration when session creation fails, skipping old-code restoration when a newer code already owns the user index, and no revival for expired/non-positive consumed TTL.
- Backend token test covers App bind sessions keeping the long-lived refresh-token duration.
- Flutter auth notifier tests cover duplicate bind-code submit suppression, startup/manual refresh behavior, and logout-controller ordering/failure behavior. Flutter device-sync tests cover Android-only platform payloads for registration/heartbeat and logout unregister behavior including failed unregister and missing local device id.
- Backend API/logic tests cover merchant App device registration, heartbeat, unregister, unsupported provider, and auth denial.
- Backend DB/scheduler tests cover stale-device cleanup deactivating only old active devices, the daily cleanup scheduler using the 90-day cutoff, the 04:30 cleanup cron expression, migration `000265` clean/incremental index proof, per-device terminal push failure degradation/deactivation, success/heartbeat degradation clearing, and migration `000266` clean/incremental schema proof.
- Push dispatcher tests cover provider grouping, send success, retryable/permanent failures, skipped unconfigured providers, permanent-failure persistence, and clearing prior device degradation after a successful provider send.

Missing high-value tests:

- Fixed/current 2026-06-11: App bind verify deletion-order tests cover no code burn on user-lookup failure, session-creation failure restoration, no expired-code revival, and duplicate successful consumption rejection.
- Flutter/contract test for unsupported native-push provider copy if product wants client-side preflight before backend rejection.
- Fixed/current 2026-06-11: Flutter logout unregister coverage proves active logout attempts backend device unregister before clearing local auth, continues logout on unregister failure, clears the local registered-token marker for rebind, and skips backend DELETE when no local device id exists.
- Fixed/current 2026-06-11: stale-device cleanup proof covers active devices with old `last_active_at` becoming inactive while recent active and already inactive devices are left alone.
- Fixed/current 2026-06-11: native-push terminal-failure policy tests cover provider reason/failure count recording, the affected registered device/push token being marked degraded first, third clear terminal failure deactivating only that row, heartbeat clearing degradation, success clearing degradation, and migration `000266` clean/incremental schema proof.

## Gaps And Refactor Notes

- Decide whether managers should be allowed to generate App bind codes; generation currently accepts `merchant_manager` as well as owner.
- Fixed/current 2026-06-11: App bind code consumption happens after recheckable role/user/token/profile preconditions and uses atomic Redis compare-and-delete; session-insert failure attempts TTL-preserving restoration.
- Fixed 2026-06-08: verify rechecks the embedded `merchantID` against the user's current active merchant roles before minting tokens.
- Fixed/current 2026-06-11: Flutter device registration and heartbeat payloads align with the Android-only backend platform contract.
- Fixed/current 2026-06-11: active Flutter logout attempts device unregister before clearing local auth, with best-effort failure handling and rebind-safe local marker cleanup.
- Fixed/current 2026-06-11: backend stale-device cleanup deactivates 90-day no-heartbeat devices that never receive active logout, such as app uninstall, storage wipe, token expiry, or process loss.
- Fixed/current 2026-06-11: native-push terminal provider failures record sanitized reason/count on only the affected active device row, mark it degraded before deactivation, and deactivate only that row after the third consecutive terminal failure; successful send, registration, and heartbeat clear stale degradation evidence.

## Branch Exhaustion

- Entry branches checked: Mini Program App bind popup/code generation, Flutter bind-code login, secure token persistence, App startup refresh, device UUID creation, native push token sync, device registration, heartbeat from polling/settings, device unregister backend route, and native push dispatch for order alerts.
- Request branches checked: bind-code generate, public bind-code verify, token/session creation, merchant access profile read during verify, device register, heartbeat, unregister, push device query by merchant, and dispatcher provider sends. Legacy staff invite `merchants.bind_code` is tracked separately under staff flow.
- Backend state branches checked: Redis code/user index reuse and TTL, public verify rate limit, final Redis compare-and-delete consumption, role recheck, session insert, token minting, device platform/provider validation, active device upsert by device id, duplicate push-token deactivation, heartbeat metadata update plus degradation clearing, push provider grouping, skipped unconfigured providers, retryable send failure, permanent send failure degradation, and third terminal-failure device-row deactivation.
- Async branches checked: native push dispatch is called from order notification paths; device registration is independent of bind-code verification; missing push token leaves polling/websocket as recovery channels; stale-device cleanup runs through `DataCleanupScheduler`.
- Failure/retry branches checked: generation Redis unavailable, verify Redis unavailable, expired/missing code, consumed-code retry, user-lookup failure before final consume, session-insert failure after final consume with TTL-preserving restore attempt, duplicate Flutter submit, registration unsupported provider/platform, missing push token, heartbeat failure, logout unregister failure, missing local device id on logout, stale device cleanup after 90 days without heartbeat, unconfigured push provider, retryable push provider failure, permanent provider failure degradation, successful-send degradation clearing, and third terminal-failure device-row deactivation.
- Reader/consumer branches checked: Flutter auth state, settings device sync tile, order polling/alert delivery, backend push dispatcher, sessions table, and merchant App device list used by push.
- Authorization/tenant branches checked: code generation accepts merchant owner/manager roles, public verify now rechecks the same Redis-embedded merchant id against current active merchant roles, device routes require owner/manager/cashier/chef staff context, and device writes derive merchant id from middleware rather than client payload.
- Zombie/unreachable branches checked: `merchants.bind_code` is not App binding truth; verify logs but does not persist device metadata.
- Test-proof gaps checked: existing tests cover Redis failures, full one-time generate/verify/session flow, embedded merchant-id recheck, consume-after-recheckable-preconditions behavior, session-failure restoration, App refresh sessions, Flutter Android-only platform payload contract, active logout unregister, backend stale-device cleanup, device registration/heartbeat/unregister, dispatcher provider branches, per-device permanent provider-failure degradation/deactivation, and degradation clearing on success/heartbeat. No known App bind/device proof gap remains in this slice.
