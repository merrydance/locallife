# LocalLife Flutter Merchant App Agent Guide

This file applies to `/home/sam/locallife/merchant_app`.

## Scope

- Flutter merchant Android app for order notifications, voice alerts, receipt printing, auth binding, and reliability-sensitive merchant workflows.
- Backend, Web, and Mini Program code live outside this scope. Follow their own `AGENTS.md` files when editing those areas.

## First Reads

For Flutter merchant app changes, read the matching project-owned guidance instead of copying it here:

1. `../.github/instructions/flutter-merchant-app.instructions.md`
2. `../.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
3. `../.github/standards/flutter/README.md`

Open the smallest relevant Flutter deep doc for the task:

- `../.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md` for reliability, failure model, and validation depth.
- `../.github/standards/flutter/FLUTTER_UI_DESIGN_STANDARDS.md` for visual hierarchy and Chinese readability.
- `../.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md` for layering, state management, and dependency boundaries.
- `../.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md` for delivery, dedup, voice alerts, and order acceptance.
- `../.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md` for vendor keep-alive and foreground service behavior.
- `../.github/standards/flutter/REVIEW_CHECKLIST.md` for review and self-check on high-risk paths.

Use `../.github/prompts/flutter-implementation.prompt.md` for implementation requests, `../.github/prompts/flutter-bugfix.prompt.md` for reliability bugs, and `../.github/prompts/flutter-review.prompt.md` for review requests.

## Working Rules

- Treat the merchant app as a lossy edge client: duplicate delivery, reconnect, process death, and permission loss are normal conditions.
- Model the merchant task, ViewState, state owner, and recovery boundary before implementing Widgets.
- Use Riverpod and feature-first structure under `lib/features/<feature>/`.
- Keep business logic and long-lived side effects out of Widgets.
- Do not assume WebSocket, push, polling, or foreground service is always available.
- Keep all user-facing strings in Chinese.

## Validation

Run commands from `merchant_app/` and choose the smallest relevant check first:

- `flutter analyze`
- `flutter test`
- `flutter build apk --release`

For device-sensitive behavior, call out any Huawei, Xiaomi, OPPO, vivo, or Android-version path that remains untested on real devices.
