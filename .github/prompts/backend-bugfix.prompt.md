---
name: "后端缺陷修复模板"
description: "Use when drafting a backend production bugfix or regression-fix request for locallife/. Trigger phrases: backend production bugfix, 后端缺陷修复, 后端线上回归, fix backend regression, 追真实根因, 不能只在 handler 打补丁, trace backend root cause, repair callback regression, rollback unsafe state drift. 适用于发起后端生产缺陷、回归问题与高风险修复任务。"
---
# Backend Bugfix Template

Use this template when asking for a backend regression or production bug fix in `locallife/`.

Use `.github/standards/backend/RUNTIME_ARCHITECTURE.md` to trace the real production path, `.github/standards/backend/WORKFLOW_AND_VALIDATION.md` to choose the right regeneration and regression commands, `.github/standards/backend/README.md` plus the matching domain README to identify already-known hot paths, and `.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md` before claiming the fix is complete.

## Backend Bug Fix

Request:

- Fix <bug, regression, or incorrect production behavior>
- Start by defining the wrong behavior and the invariant that should hold instead
- Trace the real production path: entrypoint, transaction boundary, callback, worker, scheduler, recovery, and compensating paths as relevant
- Prefer the smallest root-cause fix over a surface patch that only hides the symptom
- Tell me whether the fix requires `make sqlc`, `make mock`, `make swagger`, or `make check-generated`
- Run the smallest relevant regression validation and report what was executed
- Explain why the chosen fix layer is the lowest defensible layer that can truly enforce the invariant
- State which relevant paths were not verified and what residual risk remains

Required context:

- Failing behavior or regression: <details>
- Affected entrypoint, task, or package: <path>
- Expected invariant after the fix: <details>

Optional context:

- Related incident or escaped-defect note: <path>
- Existing audit or domain docs: `.github/standards/backend/README.md`, `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/domains/wechat-payment/README.md`
- Known risky path: <payment, refund, delivery, reservation, withdraw, complaint, callback, worker, scheduler>

Acceptance checklist:

- The fix is tied to a concrete invariant, not just a symptom
- Transaction, callback, worker, scheduler, and recovery paths were checked where the bug can cross those boundaries
- The chosen fix layer can actually prevent recurrence instead of masking the issue in one caller
- The narrowest useful regression test or safety validation was run
- The hand-off clearly states what was verified, what remains unverified, and whether the issue should feed back into standards, prompts, workflows, or tests



