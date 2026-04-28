---
name: "Flutter 商户端缺陷修复模板"
description: "Use when drafting a Flutter merchant app bugfix or regression-fix request for merchant_app/, especially for auth refresh, push, dedup, weak-network recovery, keep-alive, or accept-order regressions. Trigger phrases: Flutter 商户端缺陷修复, merchant_app bugfix, Flutter 线上回归, 修 Flutter 漏单问题, 先追 Flutter 根因, 不能只在 Widget 打补丁, fix Flutter regression, trace Flutter root cause, repair dedup regression, keep-alive regression, accept-order regression."
---
# Flutter Merchant App Bugfix Template

Use this template when asking for a regression or production bug fix in `merchant_app/`.

Use `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md` to establish the failure model, state-owner expectations, prohibited shortcuts, and validation depth. Then open the smallest relevant Flutter deep doc such as architecture, auth binding, push notification standards, or Android keep-alive guide.
Classify the task as `G0`, `G1`, `G2`, or `G3` using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`, then choose validation depth and residual-risk wording using `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

## Flutter Merchant App Bug Fix

Request:

- Fix <bug, regression, or incorrect production behavior>
- Start by defining the wrong behavior and the invariant that should hold instead
- Trace the real runtime path: user entry, provider or service owner, persistence boundary, lifecycle transition, reconnect path, restore path, and any push or polling fallback as relevant
- Prefer the smallest root-cause fix over a surface patch that only hides the symptom in one Widget or one caller
- Run the smallest relevant regression validation and report what was executed
- Explain why the chosen fix layer is the lowest defensible layer that can truly enforce the invariant
- State which relevant paths were not verified and what residual risk remains
- If the issue should feed back into standards, prompts, tests, or workflows, say that explicitly

Required context:

- Failing behavior or regression: <details>
- Affected feature, provider, or service path: <path>
- Expected invariant after the fix: <details>

Optional context:

- Related incident or escaped-defect note: <path>
- Known risky path: <auth refresh, bind login, push registration, dedup, order alert, accept-order, keep-alive, weak-network recovery>
- Existing standard or runbook to consult: <path>

Acceptance checklist:

- The fix is tied to a concrete invariant, not just a symptom
- Widget, provider, persistence, lifecycle, push, and restore boundaries were checked where the bug can cross them
- The chosen fix layer can actually prevent recurrence instead of masking the issue in one surface
- The narrowest useful regression test or safety validation was run
- The hand-off clearly states what was verified, what remains unverified, and whether the issue should feed back into standards, prompts, workflows, or tests