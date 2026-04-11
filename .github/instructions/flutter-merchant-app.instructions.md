---
applyTo: "merchant_app/**"
---

# Flutter Merchant App Instructions

Apply these rules for files under `merchant_app/`.

## Read First

- `.github/standards/flutter/README.md`

Open the smallest relevant Flutter deep docs for the current task:

- `FLUTTER_APP_ARCHITECTURE.md`: layering, state management, DI, directory conventions
- `PUSH_NOTIFICATION_STANDARDS.md`: three-channel delivery, dedup, order acceptance, voice alerts
- `ANDROID_KEEP_ALIVE_GUIDE.md`: vendor keep-alive, foreground service, permission guide

## Architecture Boundaries

- Use Riverpod for state management. Do not use Provider, GetX, or BLoC.
- Use feature-first directory structure: `lib/features/<feature>/`.
- Keep UI, business logic, and data layers separated within each feature.
- Core infrastructure in `lib/core/` must not import from `lib/features/`.
- Use GoRouter for navigation.

## Implementation Rules

- All user-facing strings must be in Chinese.
- Message deduplication by `message_id` is mandatory for all delivery channels.
- Do not assume WebSocket is always connected; design for disconnection as normal state.
- Foreground Service must run with a persistent, non-dismissable notification.
- Use `STREAM_ALARM` audio stream for order voice alerts to bypass silent mode.
- Do not hardcode API URLs; use environment config in `lib/config/env.dart`.
- Keep backend API contract aligned with Go backend Swagger docs.
- Map API errors to Chinese business messages in Dio interceptors.

## Android-Specific Rules

- Declare all required permissions in `AndroidManifest.xml`.
- Use `flutter_foreground_task` for foreground service, not manual platform channels.
- Notification channels: `order_alert` (HIGH), `merchant_fg_service` (LOW), `update_channel` (DEFAULT).
- Full-screen intent required for order alert notifications.

## Validation

- `flutter analyze` must pass with zero issues.
- `flutter test` for unit tests.
- Manual testing required on Huawei, Xiaomi, OPPO, vivo devices.

## Completion Contract

- State which features or layers changed.
- State whether the change affects message delivery, keep-alive, or push integration.
- Call out any path that remains untested on real devices.
