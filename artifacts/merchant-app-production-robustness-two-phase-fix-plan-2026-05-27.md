# Merchant App Production Robustness Two-Phase Fix Plan

> **For agentic workers:** REQUIRED FIRST READS: start with `AGENTS.md`, `.github/copilot-instructions.md`, `.github/README.md`, `.github/instructions/flutter-merchant-app.instructions.md`, then use `.github/prompts/flutter-bugfix.prompt.md` or `.github/prompts/flutter-review.prompt.md` for the active work. This plan is the task handoff source for the merchant app robustness repair and must be updated when scope, evidence, or validation changes.

**Goal:** Bring `merchant_app/` to the production robustness baseline for G3 order-alert reliability so new orders do not disappear across WebSocket, native push, polling, cold start, background re-entry, weak network, and duplicate delivery paths.

**Architecture:** Treat the Flutter app as a lossy edge client. All inbound order signals must converge into one durable alert orchestration path with explicit state ownership, persistent checkpoints, retryable pending alerts, and backend-confirmed action outcomes. Native push, foreground service, device registration, auth refresh, and UI degraded states must expose observable status instead of only logging failures.

**Tech Stack:** Flutter, Riverpod, Dio, sqflite, flutter_local_notifications, flutter_foreground_task, native Android Kotlin push integrations, Android notification/foreground service permissions.

---

## 1. Why This Plan Exists

On 2026-05-27 the merchant Android app was reviewed after fixing three immediate defects:

- First app open works, but re-entering showed a global spinner.
- New orders could arrive without voice broadcast.
- Some orders lacked the "制作完成" action.

Those defects were fixed and a release APK was built, but a broader G3 production robustness review found that the app still does not meet the "cannot miss orders" standard. Static checks and unit tests passed, but several production-only paths remain unsafe.

This document preserves the repair context so another engineer or agent can continue without relying on chat memory.

## 2. Current Evidence

### Already Fixed Before This Plan

Modified merchant app files at the time of review:

- `merchant_app/lib/features/auth/auth_provider.dart`
- `merchant_app/test/auth_notifier_test.dart`
- `merchant_app/lib/features/order/order_alert_coordinator.dart`
- `merchant_app/lib/features/order/order_provider.dart`
- `merchant_app/lib/models/order.dart`
- `merchant_app/test/order_alert_coordinator_test.dart`

Fixes already landed in the working tree:

- Auth refresh notification now clears `isLoading` during startup token refresh.
- Local notification display failure no longer blocks sound/TTS/alert flow.
- `fulfillment_status` is parsed and `courier_accepted + preparing` can show "制作完成".

### Verification Already Run

Run from `merchant_app/`:

- `flutter analyze`: passed, no issues found.
- `flutter test`: passed, 52 tests passed.
- `flutter build apk --release`: passed.

Release artifact built before this plan:

- `merchant_app/build/app/outputs/flutter-apk/app-release.apk`
- SHA256: `ea9dddab92cc15727d0c1266b70066dd29cc94ae66c0c86209e0a6419ca1981f`

### Not Yet Verified

No real-device validation has been completed for:

- Xiaomi, OPPO, vivo, Honor/Huawei native push.
- Killed-process order delivery.
- Lock-screen full-screen notification.
- TTS and `STREAM_ALARM` behavior under silent mode.
- Battery optimization and vendor background restrictions.
- Notification tap cold-start navigation.

### Phase 1 Implementation Status On 2026-05-27

Code-side Phase 1 repairs now in the working tree:

- Native push credentials are no longer hardcoded sample values; release builds read provider credentials from Gradle properties or environment variables and default to failing when credentials are missing. Internal APK builds must explicitly set `ALLOW_INCOMPLETE_PUSH_CONFIG=true`.
- Android manifest metadata now covers Xiaomi, OPPO, vivo, and Honor push placeholders used by native code.
- Device registration and heartbeat payloads include the native push `provider`.
- WebSocket and native push parsing no longer permanently commit message dedup before `OrderAlertCoordinator` accepts recoverable alert responsibility.
- `OrderAlertCoordinator` now owns alert dedup/checkpoint commitment after successful auto-accept or alert presentation, queues pending alerts when navigator is unavailable, drains pending alerts after first frame and app resume, and checks backend order status before replaying a stale pending alert.
- Pending alerts are keyed/rechecked by message and order identity so WebSocket, native push, and polling duplicates do not repeat notification/voice side effects while an order is waiting for presentation retry.
- Polling backfill now alerts current awaiting-acceptance orders based on durable alert checkpoints rather than only comparing against the in-memory order list.

Remaining Phase 1 caveats:

- Native push runtime failures are now represented in Phase 2 through `DeviceSyncState`, but provider behavior still requires real-device validation.
- Local provider credentials were added for Honor, vivo, and OPPO. Xiaomi credentials are still missing: `XIAOMI_APP_ID` and `XIAOMI_APP_KEY`.
- No vendor real-device validation has been completed.

### Phase 1 Verification Run On 2026-05-27

Run from `merchant_app/` after Phase 1 code-side repairs:

- `flutter test test/device_sync_service_test.dart test/ws_client_test.dart test/order_alert_coordinator_test.dart test/order_poller_test.dart`: passed, 35 tests passed.
- `flutter analyze`: passed, no issues found.
- `flutter test`: passed, 62 tests passed.
- `ALLOW_INCOMPLETE_PUSH_CONFIG=true flutter build apk --release`: passed.

Release artifact from this run:

- `merchant_app/build/app/outputs/flutter-apk/app-release.apk`
- Size: 55M (`flutter build` reported 57.1MB)
- SHA256: `b9926a472ea70588dfa4d37cf7c80f8fc1f3887a6072b5d123d92570f331fdac`

Build note:

- The APK above is a signed release-mode internal validation artifact. It was built with `ALLOW_INCOMPLETE_PUSH_CONFIG=true` because Xiaomi push credentials are not present in this workspace.
- A rollout production APK must be rebuilt without that flag and with all native push credentials provided.

### Phase 2 Implementation Status On 2026-05-27

Code-side Phase 2 repairs now in the working tree:

- Auth refresh now distinguishes recoverable transient refresh uncertainty from invalid-session refresh failures. Timeout, connection error, cancellation, and refresh 5xx keep stored tokens and set `AuthState.isSessionDegraded`; 401/403 still clears the session and returns to binding-required state.
- `ApiClient` no longer clears the secure token store for recoverable refresh failures, and refresh failure classification is covered by focused tests.
- Local notification launch payloads are captured on app start and queued until `onNotificationTap` is attached. Notification taps are then queued in `OrderAlertCoordinator` until the root navigator is ready.
- Native Xiaomi/vivo notification click callbacks now call the notification-opened path instead of treating clicks as ordinary message receipt.
- Foreground service policy is explicit: an authenticated online merchant keeps the foreground service running through network loss and WebSocket reconnect; the persistent notification text reflects waiting, network unavailable, reconnecting, or notification-permission-missing states.
- Device registration, native push token/provider availability, and heartbeat failures are represented by `DeviceSyncState` and surfaced on home/settings as degraded Chinese business messages instead of staying debug-only.
- Native push initialization failures are cached in `NativePushManager`, returned through `getInitializationFailure`, and propagated into `DeviceSyncState.failed` so the UI does not misreport SDK/config failures as only a missing token.
- `acceptOrder`, `rejectOrder`, and `markOrderReady` no longer declare success from a 200 response without a usable order snapshot. They immediately read back order detail and only return success when backend state confirms the requested transition. Otherwise they return the recoverable message `结果确认中，请刷新订单`.
- Android foreground-service permissions now include data sync, remote messaging, battery optimization request, and overlay permission declarations required by the keep-alive guide.

Remaining Phase 2 caveats:

- Strict production release build is blocked until Xiaomi push credentials are supplied. The build fails as intended with missing `XIAOMI_APP_ID` and `XIAOMI_APP_KEY`.
- The APK built after Phase 2 is a signed release-mode internal validation artifact only, because it was built with `ALLOW_INCOMPLETE_PUSH_CONFIG=true`.
- No Xiaomi, OPPO, vivo, Honor/Huawei real-device validation has been completed in this session.
- Notification tap behavior is covered by widget/provider tests for foreground/router-not-ready behavior, but lock-screen and killed-process notification taps still require real Android device validation.

### Phase 2 Verification Run On 2026-05-27

Run from `merchant_app/` after Phase 2 code-side repairs:

- `flutter test test/auth_notifier_test.dart test/api_client_response_test.dart test/order_provider_test.dart test/order_alert_coordinator_test.dart test/order_poller_test.dart test/device_sync_service_test.dart test/foreground_service_policy_test.dart`: passed, 53 tests passed.
- `flutter analyze`: passed, no issues found.
- `flutter test`: passed, 78 tests passed.
- `flutter build apk --release`: failed as expected on the release gate because `XIAOMI_APP_ID` and `XIAOMI_APP_KEY` are missing.
- `ALLOW_INCOMPLETE_PUSH_CONFIG=true flutter build apk --release`: passed.

Internal validation artifact from this run:

- `merchant_app/build/app/outputs/flutter-apk/app-release.apk`
- Size: 55M (`flutter build` reported 57.2MB)
- SHA256: `e41e2a595c04acc91ca8d0b7816381ca2e3d7f060bf10ffd1e01afeac07dbff6`

Build note:

- The APK above is not a production rollout artifact because Xiaomi push credentials are missing. Rebuild without `ALLOW_INCOMPLETE_PUSH_CONFIG=true` after adding `XIAOMI_APP_ID` and `XIAOMI_APP_KEY`.

## 3. Risk Classification

Risk level: `G3`.

Reason:

- Missing a new order can directly cause business loss.
- Duplicate or swallowed message handling can cause repeated or missing alerts.
- Background, cold-start, weak-network, and vendor-push behavior are mainline production paths, not edge cases.
- Client must be designed for at-least-once delivery and eventual consistency.

Primary standards:

- `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md`
- `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`
- `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`
- `.github/standards/flutter/APP_AUTH_BINDING.md`
- `.github/standards/flutter/REVIEW_CHECKLIST.md`

## 4. Root Findings To Fix

### P0. Native Push Is Not Production-Configured

Evidence:

- `merchant_app/android/app/build.gradle.kts` still contains sample or placeholder push values such as `1000000`, `5000000000000`, and `xxx`.
- `PushManager.kt` reads Xiaomi and OPPO metadata keys, but `AndroidManifest.xml` does not declare all required provider metadata consistently.
- `DeviceSyncService` does not upload the native push provider to the backend, so backend provider routing cannot be trusted.
- Push registration failures mostly stay in native/debug logs.

Impact:

- If the app process is killed, WebSocket and polling cannot help. Native push becomes the only delivery path, and it may not be registered or routable.

### P0. Message Dedup Can Mark A Message Processed Before The Alert Is Recoverable

Evidence:

- `ws_client.dart` and `native_push_manager.dart` call `MessageDeduplicator.tryAcceptGroup(...)` before `OrderAlertCoordinator.handleIncomingOrder(...)`.
- `message_dedup.dart` persists processed keys immediately.
- `OrderAlertCoordinator._presentAlert(...)` returns when `rootNavigatorKey.currentState` is null.

Impact:

- If a message is accepted into dedup but notification, audio, hydration, or navigator presentation fails, the same message/order may be suppressed for 24 hours.

### P0. Cold-Start Or Polling Backfill Can Put Paid Orders In The List Without Alerting

Evidence:

- `OrderListPage.initState` calls `fetchOrders()` when the merchant is online.
- `OrderPoller.pollOnce()` snapshots current provider orders, fetches latest paid orders, then alerts only orders missing from the previous in-memory list.
- If a paid order is already loaded by the list before the poller diff runs, it is not considered "new" and may not voice/popup.

Impact:

- Orders created while the app was killed or offline can silently appear in the list without the high-priority alert experience.

### P1. Foreground Service Is Not A Real Recovery Owner

Evidence:

- `ws_provider.dart` stops the foreground service whenever connectivity becomes false.
- `foreground_service.dart` only logs heartbeat events and does not trigger reconnect, polling, heartbeat, or notification updates.

Impact:

- The service does not currently provide the resilience promised by the keep-alive standard.

### P1. Auth Refresh Clears Tokens On Transient Network Failure

Evidence:

- `auth_service.dart` clears tokens for any exception in startup refresh.
- `api_client.dart` clears session for any `DioException` during refresh.

Impact:

- Weak network, DNS issues, or backend 5xx can force re-binding and interrupt order receiving.

### P1. Notification Tap And Cold-Start Navigation Are Incomplete

Evidence:

- `local_notification_service.dart` has an empty background notification response callback.
- There is no confirmed launch-payload consumption path before app routing.
- Native notification-click callbacks are treated similarly to normal message receipt in Xiaomi/vivo receivers.

Impact:

- A merchant tapping a notification after the process was killed may not reach the correct order flow.

### P2. Degraded States Are Mostly Debug Logs

Evidence:

- Device registration and heartbeat failures are debug-only in `device_sync_service.dart`.
- Notification permission result is not modeled as app state.
- Push token/provider state is not visible in the home screen.

Impact:

- The UI may show "在线营业" while a production-critical channel is broken.

## 5. Two-Phase Execution Model

Do not try to fix everything at once. Finish Phase 1, build an internal APK, and run real-device order-delivery validation before starting Phase 2.

### Phase 1: Stop The Order-Loss Paths

Objective:

- Make inbound order delivery recoverable and alerting durable across WebSocket, native push, polling, cold start, and process re-entry.

Exit criteria:

- A new paid order can never be marked processed before at least one recoverable alert path is recorded.
- A paid order discovered after app start, cold start, or poller backfill still triggers voice/popup once.
- Native push registration uses real configurable credentials and uploads provider-aware device records.
- Release APK builds and passes targeted plus full Flutter tests.

### Phase 2: Production Hardening And Release Gate

Objective:

- Make the app honestly report degraded states, recover auth/foreground-service/network failures, and pass real-device release validation.

Exit criteria:

- Foreground service remains an active recovery owner during weak network.
- Auth refresh distinguishes transient network failure from invalid session.
- Notification tap works from foreground, background, lock screen, and killed-process starts.
- Home/settings UI surfaces notification, push, heartbeat, and connection degradation.
- Vendor-device validation matrix is complete or residual risk is explicitly signed off.

## 6. Phase 1 Task Plan

### Task 1. Native Push Production Configuration

Files:

- Modify: `merchant_app/android/app/build.gradle.kts`
- Modify: `merchant_app/android/app/src/main/AndroidManifest.xml`
- Modify: `merchant_app/android/app/src/main/kotlin/com/merrydance/locallife/merchant/push/PushManager.kt`
- Modify: `merchant_app/lib/core/push/native_push_manager.dart`
- Modify: `merchant_app/lib/core/push/device_sync_service.dart`
- Add tests if provider parsing or device payload builder becomes testable.

Implementation requirements:

- Replace hardcoded sample push credentials with Gradle properties or environment-derived values.
- Fail release builds when native push provider credentials required for production are still sample/empty values, unless an explicit non-production build flag is set.
- Ensure Android manifest declares metadata for every provider read by `PushManager`.
- Include `provider` in the device registration and heartbeat payload.
- Expose native push init/token registration failures to Flutter via MethodChannel instead of debug-only logs.

Suggested checks:

```bash
cd /home/sam/locallife/merchant_app
flutter analyze
flutter test
flutter build apk --release
```

Manual validation:

- Install release build on at least one configured provider device.
- Confirm native token is obtained.
- Confirm `/merchant/device/register` receives `push_token` and `provider`.
- Kill app, send test order, confirm OS-level push arrives.

### Task 2. Move Dedup Ownership To Recoverable Alert Orchestration

Files:

- Modify: `merchant_app/lib/core/service/message_dedup.dart`
- Modify: `merchant_app/lib/core/network/ws_client.dart`
- Modify: `merchant_app/lib/core/push/native_push_manager.dart`
- Modify: `merchant_app/lib/features/order/order_alert_coordinator.dart`
- Test: `merchant_app/test/order_alert_coordinator_test.dart`
- Test: `merchant_app/test/ws_client_test.dart`

Implementation requirements:

- `WsClient` and `NativePushManager` should parse inbound messages but should not permanently mark messages processed before the alert coordinator accepts responsibility.
- Introduce one of these designs:
  - Preferred: `OrderAlertCoordinator` owns dedup after hydration and after a pending alert record is written.
  - Acceptable: `MessageDeduplicator` supports `beginProcessing(keys)`, `commitProcessing(keys)`, and `releaseProcessing(keys)` semantics.
- Persist enough pending state to retry an alert if navigator, notification, audio, or hydration fails before presentation.
- Preserve duplicate suppression for simultaneous WebSocket/native/polling delivery.

Regression tests to add:

- Same `message_id` from WebSocket and native push only presents once.
- Navigator unavailable does not permanently suppress the order.
- Notification display failure still allows voice/TTS and pending alert commitment.
- Hydration failure does not create a false "processed forever" state.

Suggested commands:

```bash
cd /home/sam/locallife/merchant_app
flutter test test/order_alert_coordinator_test.dart test/ws_client_test.dart
flutter test
flutter analyze
```

### Task 3. Durable Pending Alert Queue

Files:

- Create: `merchant_app/lib/core/service/pending_order_alert_store.dart`
- Modify: `merchant_app/lib/features/order/order_alert_coordinator.dart`
- Modify: `merchant_app/lib/main.dart`
- Test: `merchant_app/test/order_alert_coordinator_test.dart`

Implementation requirements:

- Persist pending alerts by stable keys:
  - `message_id` when available.
  - `order_id` plus alert type when message ID is absent.
- Pending alerts must store enough payload to retry:
  - order id
  - message id
  - order number or pickup code if available
  - amount
  - timestamp
  - source channel
- Drain pending alerts after app startup, route readiness, and app resume.
- Expire stale pending alerts only after backend confirms the order is no longer awaiting acceptance, or after a clearly documented retention window with a backend reconciliation attempt.

Regression tests to add:

- Pending alert is written when navigator is unavailable.
- Pending alert is drained once navigator becomes available.
- Pending alert is removed after order is no longer `paid`.
- Duplicate pending records do not trigger duplicate voice/popup.

Suggested commands:

```bash
cd /home/sam/locallife/merchant_app
flutter test test/order_alert_coordinator_test.dart
flutter test
flutter analyze
```

### Task 4. Polling And Cold-Start Backfill Alerts

Files:

- Create: `merchant_app/lib/core/service/order_alert_checkpoint_store.dart`
- Modify: `merchant_app/lib/core/service/order_poller.dart`
- Modify: `merchant_app/lib/features/order/order_alert_coordinator.dart`
- Modify: `merchant_app/lib/features/order/order_provider.dart`
- Test: `merchant_app/test/order_poller_test.dart`
- Test: `merchant_app/test/order_alert_coordinator_test.dart`

Implementation requirements:

- Do not use current in-memory order list as the only "already alerted" source.
- Track paid-order alert checkpoints durably by `order_id` and current backend status.
- On startup and on polling:
  - fetch current awaiting-acceptance orders;
  - compare against durable alert checkpoints;
  - alert any paid order that has not been alerted in its current paid lifecycle.
- Clear or ignore checkpoints after backend status leaves `paid`.

Regression tests to add:

- A paid order already present in `orderProvider.orders` still alerts if no durable checkpoint exists.
- Repeated polling of the same paid order does not repeat alert after checkpoint.
- Status transition away from paid clears or bypasses pending alert.
- Cold start with existing paid order triggers alert once.

Suggested commands:

```bash
cd /home/sam/locallife/merchant_app
flutter test test/order_poller_test.dart test/order_alert_coordinator_test.dart
flutter test
flutter analyze
```

### Phase 1 Integration Gate

Run:

```bash
cd /home/sam/locallife/merchant_app
flutter analyze
flutter test
flutter build apk --release
```

If provider credentials are not available and the goal is only an internal validation APK, run the release build with `ALLOW_INCOMPLETE_PUSH_CONFIG=true`. Do not use that flag for production rollout.

Manual validation before Phase 2:

- Foreground WebSocket new order triggers one voice/popup.
- Native push new order triggers one voice/popup.
- Polling-discovered new order triggers one voice/popup.
- Same order delivered by all three channels triggers only one alert.
- App killed before order, then opened: paid order alerts once.
- App starts while navigator is not ready: alert is not lost and is retried.

## 7. Phase 2 Task Plan

### Task 5. Foreground Service Recovery Owner

Files:

- Modify: `merchant_app/lib/core/service/foreground_service.dart`
- Modify: `merchant_app/lib/core/network/ws_provider.dart`
- Modify: `merchant_app/lib/core/service/order_poller.dart`
- Modify: `merchant_app/android/app/src/main/AndroidManifest.xml`
- Test: `merchant_app/test/order_poller_test.dart`

Implementation requirements:

- Do not stop the foreground service just because network is temporarily unavailable.
- Foreground service should represent app recovery state:
  - online and waiting for orders
  - network unavailable
  - WebSocket reconnecting
  - polling fallback active
  - notification permission missing
- Heartbeat should coordinate with an app-side owner that can trigger device heartbeat, status sync, reconnect, or poll fallback when the app process is alive.
- Add missing Android service permissions/types required by the keep-alive standard and current target SDK.

Validation:

```bash
cd /home/sam/locallife/merchant_app
flutter analyze
flutter test test/order_poller_test.dart
flutter test
```

Manual validation:

- Start online, background app, confirm persistent foreground notification remains.
- Turn off network for one minute, confirm service stays active and UI/notification shows degradation.
- Restore network, confirm reconnect and backfill.

### Task 6. Auth Refresh Degraded State

Files:

- Modify: `merchant_app/lib/core/network/api_client.dart`
- Modify: `merchant_app/lib/features/auth/auth_service.dart`
- Modify: `merchant_app/lib/features/auth/auth_provider.dart`
- Modify: `merchant_app/lib/features/auth/auth_state.dart`
- Test: `merchant_app/test/auth_notifier_test.dart`
- Test: `merchant_app/test/api_client_response_test.dart`

Implementation requirements:

- Clear tokens only on explicit invalid-session signals such as 401/403 from refresh.
- Do not clear tokens on timeout, DNS, connection error, cancellation, or backend 5xx.
- Represent auth as recoverable degraded state when refresh outcome is unknown.
- Block critical write actions while auth is unresolved, but keep automatic recovery running.
- Keep all user-facing messages Chinese.

Regression tests to add:

- Refresh timeout does not clear stored tokens.
- Refresh 500 does not clear stored tokens.
- Refresh 401 clears tokens and returns to binding-required state.
- Concurrent refresh remains single-flight.

Validation:

```bash
cd /home/sam/locallife/merchant_app
flutter test test/auth_notifier_test.dart test/api_client_response_test.dart
flutter test
flutter analyze
```

### Task 7. Notification Tap And Cold-Start Routing

Files:

- Modify: `merchant_app/lib/core/push/local_notification_service.dart`
- Modify: `merchant_app/lib/core/push/native_push_manager.dart`
- Modify: `merchant_app/android/app/src/main/kotlin/com/merrydance/locallife/merchant/push/XiaomiPushReceiver.kt`
- Modify: `merchant_app/android/app/src/main/kotlin/com/merrydance/locallife/merchant/push/VivoPushReceiver.kt`
- Modify: `merchant_app/android/app/src/main/kotlin/com/merrydance/locallife/merchant/push/HonorPushReceiver.kt`
- Modify: `merchant_app/lib/main.dart`
- Modify: `merchant_app/lib/features/order/order_alert_coordinator.dart`
- Test: `merchant_app/test/order_alert_coordinator_test.dart`

Implementation requirements:

- Capture local notification launch payload on app start.
- Persist or queue background notification tap payload until Flutter and router are ready.
- Native notification clicks must invoke a notification-opened path, not the same path as foreground message receipt.
- Tap behavior:
  - if backend says order is still awaiting acceptance, show order alert page;
  - otherwise open order detail;
  - if order detail fetch fails, show recoverable Chinese error and retry option.

Validation:

```bash
cd /home/sam/locallife/merchant_app
flutter test test/order_alert_coordinator_test.dart
flutter test
flutter analyze
```

Manual validation:

- Foreground notification tap.
- Background notification tap.
- Lock-screen notification tap.
- Killed-process notification tap.

### Task 8. Device, Push, And Permission Degraded ViewState

Files:

- Modify: `merchant_app/lib/core/push/device_sync_service.dart`
- Modify: `merchant_app/lib/core/push/push_provider.dart`
- Modify: `merchant_app/lib/features/order/order_list_page.dart`
- Modify: `merchant_app/lib/features/settings/settings_page.dart`
- Modify: `merchant_app/lib/features/settings/permission_guide_page.dart`
- Test: add or extend focused widget/provider tests.

Implementation requirements:

- Model these states in Riverpod:
  - notification permission granted/missing/unknown
  - native push initialized/failed/no-provider
  - push token registered/missing
  - device registration success/failure/retrying
  - heartbeat success/failure/stale
- Home screen must not show a clean "在线营业" impression when critical channels are degraded.
- Settings page must provide clear Chinese next steps and retry actions.
- Permission guide remains vendor-specific and points to system settings.

Validation:

```bash
cd /home/sam/locallife/merchant_app
flutter analyze
flutter test
```

Manual validation:

- Disable notification permission and confirm home/settings warnings.
- Simulate missing push token and confirm degraded state.
- Simulate heartbeat failure and confirm warning does not block local UI but is visible.

### Task 9. Backend-Confirmed Order Actions

Files:

- Modify: `merchant_app/lib/features/order/order_provider.dart`
- Modify: `merchant_app/lib/features/order/order_alert_page.dart`
- Modify: `merchant_app/lib/features/order/order_detail_page.dart`
- Test: `merchant_app/test/order_provider_test.dart`
- Test: `merchant_app/test/order_alert_coordinator_test.dart`

Implementation requirements:

- `acceptOrder`, `rejectOrder`, and `markOrderReady` must not declare success solely because the action endpoint returned 200 without a parseable order state.
- If action response lacks a usable order snapshot, immediately fetch order detail.
- If detail readback confirms the expected backend state, then return success.
- If detail readback fails or returns unknown state, return a recoverable "结果确认中，请刷新订单" state instead of optimistic success.
- Preserve single-flight duplicate-tap protection.

Validation:

```bash
cd /home/sam/locallife/merchant_app
flutter test test/order_provider_test.dart test/order_alert_coordinator_test.dart
flutter test
flutter analyze
```

## 8. Release Validation Matrix

Before compiling a production release for rollout, complete this matrix or write an explicit residual-risk note.

| Scenario | Required Result | Evidence |
|---|---|---|
| First launch after install | Binding/login route stable, no permanent spinner | Screenshot or test note |
| Exit and re-enter | No global spinner lock, auth restored or degraded clearly | Test note |
| Foreground WebSocket order | One voice alert, one popup, order appears | Test note |
| Native push order while background | Notification, voice/popup if process alive | Device/provider note |
| Native push order while killed | OS notification arrives and tap opens correct order | Device/provider note |
| Polling backfill | Paid order found by polling alerts once | Test note |
| Duplicate delivery across channels | Only one alert | Test note |
| No network then restore | Degraded state visible, backfill after restore | Test note |
| Notification permission denied | Home/settings warning visible | Screenshot or note |
| Silent mode | Order alert still audible via alarm stream | Device note |
| Lock screen | Full-screen or high-priority notification visible | Device note |
| Accept duplicate tap | Single backend action and stable UI | Test note |
| Ready duplicate tap | Single backend action and stable UI | Test note |

Provider/device minimum:

- Xiaomi or Redmi
- vivo
- OPPO or Realme
- Honor or Huawei-compatible path

## 9. Commands To Run At Major Checkpoints

From `merchant_app/`:

```bash
flutter pub get
flutter analyze
flutter test
flutter build apk --release
```

Focused checks:

```bash
flutter test test/order_alert_coordinator_test.dart
flutter test test/order_poller_test.dart
flutter test test/ws_client_test.dart
flutter test test/auth_notifier_test.dart
flutter test test/order_provider_test.dart
```

Diff hygiene from repo root:

```bash
git diff --check -- merchant_app
git status --short
```

## 10. Handoff Notes

- Work from `/home/sam/locallife`.
- Do not touch unrelated `weapp/` dirty files unless the user explicitly expands scope.
- Do not treat `flutter analyze` and `flutter test` as proof of production push reliability; real-device validation is mandatory for Phase 2 release.
- If backend contracts are needed, use Go backend Swagger/source as truth. Do not invent client-only status semantics.
- Keep all user-facing strings in Chinese.
- Keep state ownership in providers/services; Widgets render state and dispatch intent only.
- Update this document whenever a task is completed, deferred, or invalidated by new evidence.

## 11. Completion Checklist

Phase 1:

- [x] Native push credentials and metadata are production-configurable.
- [x] Device registration includes push provider.
- [x] Push failures are visible to Flutter state.
- [x] Dedup no longer permanently suppresses unpresented alerts.
- [x] Pending alert store exists and drains after startup/resume.
- [x] Polling and cold-start paid orders alert once.
- [x] Phase 1 APK builds.
- [ ] Phase 1 manual delivery validation completed.

Phase 2:

- [x] Foreground service stays active as recovery owner.
- [x] Auth refresh distinguishes transient failure from invalid session.
- [ ] Notification tap works across foreground/background/killed starts.
- [x] Degraded push/permission/heartbeat states are visible.
- [x] Order actions require backend-confirmed state or explicit confirmation-needed state.
- [x] Full Flutter validation passes.
- [ ] Vendor device validation matrix is complete.
- [ ] Production release APK built and SHA256 recorded.
- [x] Internal release-mode validation APK built and SHA256 recorded.
