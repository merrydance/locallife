# Merchant App (Flutter Android) Instructions

This file applies to `merchant_app/`.

## Scope

- Flutter Android application for merchant order notification, voice alerts, and receipt printing.
- Target: Chinese Android market, private APK distribution (no app store).
- Backend integration: Go monolith at `locallife/` via REST API + WebSocket.

## Read First

1. `.github/standards/flutter/README.md`
2. `.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md`
3. `.github/standards/flutter/APP_AUTH_BINDING.md`
4. `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`
5. `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`
6. `.github/standards/engineering/README.md`

## Architecture Rules

- **State Management**: Riverpod only. Use `StreamProvider` for WebSocket streams, `FutureProvider` for API calls, `StateNotifierProvider` for mutable UI state.
- **Directory Structure**: Feature-first (`lib/features/<feature>/`). Shared code in `lib/core/`.
- **Dependency Injection**: Via Riverpod providers. No service locators or singletons outside of Riverpod scope.
- **Navigation**: GoRouter for declarative routing.

## Authentication

The app uses a **binding code** authentication scheme. See `.github/standards/flutter/APP_AUTH_BINDING.md` for full details.

- **First login**: Merchant generates a 6-digit code in the WeChat Mini Program → enters it in the App → receives JWT tokens.
- **Subsequent launches**: Auto-refresh via `POST /v1/auth/refresh`. No re-binding needed unless app is reinstalled or token expires (365 days).
- **Token storage**: Use `flutter_secure_storage` (Android Keystore encryption). Never use `shared_preferences` for tokens.
- **Dio interceptor**: Automatically attach Bearer token to all requests; auto-refresh on 401.

## Message Delivery Contract

The app receives order notifications through three channels simultaneously:

1. **WebSocket** — real-time, unreliable (network dependent)
2. **JPush vendor push** — reliable even when app is killed, 1-3s latency
3. **Polling fallback** — every 30 seconds, GET `/v1/merchant/orders/pending`

All three channels may deliver the same message. **Deduplication by `message_id` is mandatory.** Use `MessageDeduplicator` in `lib/core/service/message_dedup.dart`.

## Order Acceptance Loop

1. Order arrives → deduplicate → voice alert + full-screen intent
2. Merchant must tap "接单" within 60 seconds
3. If not accepted: backend re-pushes + alerts operations team
4. After acceptance: trigger receipt printing (cloud or Bluetooth)

## Android-Specific Rules

- **Foreground Service**: Must run with a persistent notification "乐客来福正在运行". Use `flutter_foreground_task`.
- **Notification Channels**: 
  - `order_alert` (HIGH importance) — new orders
  - `merchant_fg_service` (LOW importance) — foreground service
  - `update_channel` (DEFAULT) — app updates
- **Full-Screen Intent**: Required for order alerts to wake locked screen.
- **Keep-Alive**: See `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`.

## Backend API Endpoints (merchant_app uses)

Auth:
- `POST /v1/auth/app-bind/verify` — verify binding code, get JWT tokens (public, no auth)
- `POST /v1/auth/refresh` — refresh token pair (existing endpoint)

Device:
- `POST /v1/merchant/device/register` — register push token
- `DELETE /v1/merchant/device/:device_id` — unregister device
- `PUT /v1/merchant/device/heartbeat` — device heartbeat

Orders:
- `GET /v1/merchant/orders/pending` — polling fallback
- `POST /v1/merchant/orders/:id/accept` — confirm order acceptance

Update:
- `GET /v1/app/version/latest` — check for updates

WebSocket:
- `GET /v1/ws` — WebSocket connection (existing)

## UI/UX Rules

- All user-facing text in Chinese.
- Voice alerts use pre-recorded MP3 for "您有新的乐客来福订单" and `flutter_tts` for dynamic content like order number and amount.
- Connection status indicator must always be visible on the main screen.
- Permission guide page must detect and prompt for: auto-start, battery optimization ignore, lock-screen display.

## Testing

- `flutter test` for unit tests
- `flutter analyze` for static analysis
- Manual testing required on 4 vendor phones: Huawei, Xiaomi, OPPO, vivo
- Test matrix: foreground, background, locked screen, process killed

## Do Not

- Do not use Provider (use Riverpod instead).
- Do not skip message deduplication in any delivery path.
- Do not use `awesome_notifications` (conflicts with JPush).
- Do not assume WebSocket is always connected.
- Do not write English user-facing strings.
- Do not hardcode API URLs; use environment config.
