---
name: "Flutter 商户端审查请求模板"
description: "Use when drafting a Flutter merchant app review request with findings-first output, including provider wiring review, dedup review, keep-alive review, and weak-network recovery review. Trigger phrases: 审查这个 Flutter 商户端改动, Flutter 商户端审查, Flutter review, merchant_app review, provider wiring, 消息去重, 弱网恢复, 接单链路, 推送链路审查, inspect provider wiring, check message dedup, verify keep-alive flow."
---
# Flutter Merchant App Review Template

Use this template when asking for a review in `merchant_app/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the review scope before writing the request. Do not keep relying on stale context.

Use the Flutter Merchant App row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and findings-first review checks.
Use `.github/standards/flutter/README.md` as the standards index, `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md` as the default review baseline, and `.github/standards/flutter/REVIEW_CHECKLIST.md` as the compact checklist, then open the smallest relevant deep doc for visual design, push, auth, architecture, or keep-alive behavior.
Use `.agents/skills/locallife-human-centered-ui/references/review-rubric.md` and `.agents/skills/locallife-human-centered-ui/references/merchant-admin.md` when merchant-facing UI or feedback changes so store work habits, high-frequency actions, preserved context, and recovery defects are reviewed before cosmetic issues.
Infer or state the task risk level (`G0`/`G1`/`G2`/`G3`) using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`, then scale validation and residual-risk expectations with `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

## Flutter Merchant App Review

Request:

- Review this change with findings first, ordered by severity
- Infer or confirm the task risk level (`G0`/`G1`/`G2`/`G3`) and call out when a clearly reliability-sensitive path was treated as routine
- Check it against `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md` and the smallest relevant specialized Flutter standard
- Check it against `.github/standards/flutter/REVIEW_CHECKLIST.md` so reliability, state, and UI consistency are reviewed together instead of separately drifting
- Check whether any changed UI keeps the merchant's common path, first-screen priority, preserved context, and degraded-state recovery clear
- State what validation evidence exists, what was not verified, and what residual risk remains

Review must prioritize:

- Broken service-to-provider-to-widget propagation
- Missing state owners or multiple competing truth sources
- Dedup, retry, duplicate tap, reconnect, cold-start restore, or background re-entry regressions
- Visual-system drift that weakens hierarchy, Chinese readability, action priority, or degraded-state clarity
- Optimistic success or fake-online UI on `G2` or `G3` paths
- Foreground service, push registration, auth refresh, or accept-order flows that changed without lifecycle-safe handling
- High-risk paths that appear unverified on real devices or across weak-network transitions

Review must not do:

- Do not lead with summary before findings
- Do not spend most of the review on style trivia while reliability paths remain unchecked
- Do not accept Widget-level business logic or unmanaged background loops as harmless implementation detail
- Do not mark dedup, push, keep-alive, auth refresh, or accept-order changes as safe when duplicate or re-entry behavior was not exercised
- Do not treat missing backend semantics as freedom for the client to guess

Required context:

- Changed paths: <paths>

Optional context:

- Expected behavior or invariant: <details>
- Risk level guess: <G0/G1/G2/G3>
- Merchant task frequency and preserved context expectations: <current store/order/tab/filter/draft/alert state>
- Review focus: <general reliability | dedup | keep-alive | auth | accept-order>
- Validation evidence already run: <commands or manual scenarios>

Review checklist:

- The flow has one clear state owner and does not spread business truth across Widget state, provider state, and ad hoc caches
- Service changes propagate through provider, persistence, lifecycle handling, and visible UI states
- Duplicate delivery, duplicate tap, reconnect, cold start, and background re-entry were handled deliberately when relevant
- User-facing feedback distinguishes success, degraded mode, retrying, and unrecovered failure instead of flattening everything into one toast
- Merchant-facing UI keeps the high-frequency task easiest and preserves context across retry, cold start, and foreground re-entry when relevant
- Any real-device-only path or vendor-sensitive behavior that was not validated is called out as residual risk
- If there are no findings, say so explicitly and mention residual risks or validation gaps
