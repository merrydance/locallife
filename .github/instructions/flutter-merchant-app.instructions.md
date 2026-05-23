---
applyTo: "merchant_app/**"
---

# Flutter Merchant App Instructions

Apply these rules for files under `merchant_app/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Read First

- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/flutter/README.md`

Open the smallest relevant Flutter deep docs for the current task:

- `PRODUCTION_ROBUSTNESS_BASELINE.md`: failure model, prohibited shortcuts, risk hints, validation depth, task annotation fields
- `FLUTTER_UI_DESIGN_STANDARDS.md`: visual hierarchy, Chinese readability, spacing, state presentation, interaction rules
- `FLUTTER_APP_ARCHITECTURE.md`: layering, state management, DI, directory conventions
- `PUSH_NOTIFICATION_STANDARDS.md`: three-channel delivery, dedup, order acceptance, voice alerts
- `ANDROID_KEEP_ALIVE_GUIDE.md`: vendor keep-alive, foreground service, permission guide
- `REVIEW_CHECKLIST.md`: compact checklist for review and self-check on `G2`/`G3` paths

## Architecture Boundaries

- Use `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md` as the shared frontend rule for user-task-first design, repository/use-case/presentation boundaries, ViewState modeling, and API-flattening anti-patterns.
- Use Riverpod for state management. Do not use Provider, GetX, or BLoC.
- Use feature-first directory structure: `lib/features/<feature>/`.
- Keep UI, business logic, and data layers separated within each feature.
- Core infrastructure in `lib/core/` must not import from `lib/features/`.
- Use GoRouter for navigation.

## Implementation Rules

- All user-facing strings must be in Chinese.
- Treat `merchant_app/` as a lossy edge client: duplicate delivery, reconnect, process death, and permission loss are default assumptions.
- Model the merchant's task before mirroring backend entities into screens; repository/API models must be adapted through feature state or use-case objects before Widgets render them.
- For UI work, use Flutter design tokens and stable spacing hierarchy; do not transplant web-only concepts like hover-dependent behavior or CSS blur defaults into Flutter screens.
- Message deduplication by `message_id` is mandatory for all delivery channels.
- Do not assume WebSocket is always connected; design for disconnection as normal state.
- For non-trivial flows, define the state owner, recovery boundary, and validation plan explicitly.
- Critical actions must be safe under retry or duplicate tap; do not ship client-only "exactly once" assumptions.
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
- CI now enforces `flutter analyze`, `flutter test`, and a changed-file architecture guard for `merchant_app/` changes.
- The changed-file guard blocks widget or page files that import `Dio`, `FlutterSecureStorage`, `sqflite`, or direct `ApiClient` infrastructure, and blocks hardcoded endpoints outside `lib/config/env.dart`.
- Manual testing required on Huawei, Xiaomi, OPPO, vivo devices.

## Completion Contract

- State which features or layers changed.
- State the risk level or reliability sensitivity when the task is not routine UI-only work.
- State whether the change affects message delivery, keep-alive, or push integration.
- Call out any path that remains untested on real devices.
