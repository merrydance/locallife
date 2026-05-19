---
name: "Flutter 商户端实现请求模板"
description: "Use when drafting a Flutter implementation request for merchant_app/, including bind-code login, push registration, foreground service, order alert, and reconnect-sensitive feature work. Trigger phrases: merchant_app, 绑定码登录, 推送注册, 前台服务, 实现请求模板, 实现 Flutter 商户端, merchant_app 实现, Flutter 页面开发, implement Flutter feature, 开发商户端功能, 新增 Flutter 功能, 绑定码登录实现, 接单弹窗实现, 推送注册实现, 前台服务接入, reconnect flow implementation, token refresh implementation, order alert implementation, JPush 接入, keep-alive implementation."
---
# Flutter Merchant App Implementation Template

Use this template when asking for a concrete Flutter change in `merchant_app/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the implementation scope before writing the request. Do not keep relying on stale context.

Use `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md` as the cross-frontend baseline for user-task-first design, repository/use-case/presentation boundaries, ViewState modeling, and API-flattening anti-patterns.
Use the Flutter Merchant App row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and review-ready hand-off expectations.
Use `.github/standards/flutter/README.md` as the standards index, `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md` as the default reliability baseline, and the smallest relevant deep doc such as visual design, architecture, push, auth, or keep-alive instead of copying the full standards body here.
Classify the task as `G0`, `G1`, `G2`, or `G3` using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`, then choose validation depth and residual-risk wording using `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

## Flutter Merchant App Change

Request:

- Update or build <feature, page, provider, or service>
- State the task risk level (`G0`/`G1`/`G2`/`G3`) and why
- Follow `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md` for failure model, prohibited shortcuts, and task annotation expectations
- Follow the smallest relevant deep doc explicitly: visual design standard, architecture, auth binding, push notification standards, or Android keep-alive guide
- Run validation that matches the risk level and report what was executed
- State which paths remain unverified and what residual risk remains

Implementation must push:

- Treat the merchant app as a lossy edge client where duplicate delivery, reconnect, process death, and delayed backend truth are normal conditions
- Start from the merchant's task and ViewState before designing screens; do not mirror API/entity models directly into Widgets
- When the task changes UI or component hierarchy, follow `.github/standards/flutter/FLUTTER_UI_DESIGN_STANDARDS.md` so visual polish does not drift away from operational clarity
- Name the state owner for the touched flow and keep business state out of Widgets
- Keep the flow closed across service, provider, persistence, lifecycle hooks, and user-visible feedback
- Explicitly handle duplicate tap, retry, cold-start restore, and background-to-foreground re-entry when the changed path is `G2` or `G3`
- Use backend contract or stable message identifiers as the source of truth instead of inventing client-only semantics
- Name the recovery boundary for the changed flow: what survives restart, what is rebuilt from backend truth, and what is intentionally ephemeral
- Report any real-device dependency or vendor-specific behavior that remains unverified

Implementation must not do:

- Do not put Dio, database access, or long-lived side effects directly in Widgets
- Do not assume WebSocket, push, or foreground service is always available
- Do not assume one click or one message means one delivery and one effect
- Do not ship optimistic success for `G2` or `G3` actions before backend confirmation
- Do not create multiple timers, reconnect loops, pollers, or audio queues without a single owner
- Do not invent backend fields, status semantics, permission meaning, or lifecycle guarantees
- Do not hide verification gaps when the change touches dedup, order alert, accept-order flow, auth refresh, push registration, or keep-alive

Required context:

- Target path or feature: <path>
- User task or business invariant: <what the merchant must reliably finish>
- Risk level: <G0/G1/G2/G3 + reason>
- Backend or protocol source of truth: <swagger/backend handler/typed contract>

Optional context:

- Failure modes to preserve: <duplicate/retry/weak network/cold start/re-entry/permission loss>
- State owner candidate: <provider/service>
- Recovery boundary: <persisted state + restore path>
- Device or vendor sensitivity: <Huawei/Xiaomi/OPPO/vivo/Android version>
- Related push, polling, foreground service, or printing path: <details>

Acceptance focus:

- The hand-off names the risk level, state owner, recovery boundary, and validation depth
- The implementation is closed across service, provider, persistence, lifecycle, and user feedback instead of stopping at one layer
- If the changed path is reliability-sensitive, duplicate delivery and re-entry behavior are either verified or explicitly listed as residual risk
- Any missing backend contract or real-device dependency is stated directly instead of guessed around
