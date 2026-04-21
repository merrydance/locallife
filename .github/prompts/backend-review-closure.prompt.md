---
name: "后端闭环审查模板"
description: "Use when drafting a backend review request focused on end-to-end completeness. Trigger phrases: review backend closure, DTO 字段, handler、logic、sqlc 和测试, check handler to store propagation, inspect orphan SQL, verify generation steps, findings first backend review. 适用于强调端到端闭环与传播完整性的后端审查。"
---
# Backend Review With Closure Checks Template

Use this template when asking for a backend review that emphasizes end-to-end completeness.

Use the backend row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and findings-first review checks.
Use `.github/standards/backend/RUNTIME_ARCHITECTURE.md` to keep review scope on the real production path rather than a simplified three-layer sketch.
Use `.github/standards/backend/WORKFLOW_AND_VALIDATION.md` when deciding which generation and validation steps should have been run.
Use `.github/standards/backend/README.md` plus the matching domain README to bias review depth toward already-known high-risk production chains.
Use `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md` when the review is formal enough that findings, residual risk, or systemic feedback should become durable project knowledge.
Use `.github/standards/backend/FORMAL_REVIEW_DURABILITY.md` when deciding where reusable findings should be written back.

## Backend Closure Review

Target area: `locallife/`

Request:

- Review this backend change with findings first, ordered by severity
- Prioritize bugs, regressions, contract violations, broken propagation, and missing validation
- Check whether the capability owner is clear and whether any important state transition now has multiple writers
- Check for logic that appears unused, unreachable, or computed without affecting behavior
- Check for SQL, store, logic, handler, route, worker, or scheduler changes that were added in one layer but not connected through the remaining layers
- Check whether durable state changes and external side effects remain separated by a defensible boundary
- Flag debug leftovers such as temporary prints, panic probes, hardcoded values, placeholder branches, or short-circuit returns
- Check whether `make sqlc`, `make mock`, `make swagger`, `make test-unit`, or `make test-integration` should have been run
- Check whether repo-specific closeout actions such as `make check-generated`, safety regressions, or standards/workflow feedback should have been triggered
- Call out unverified high-risk paths explicitly when the change touches callbacks, async jobs, payment, uploads, OCR, or authorization-sensitive logic
- If a high-risk path changed but evidence is missing, say exactly what remained unverified, such as callback idempotency, retry classification, signed access control, or recovery scheduling
- If no findings are discovered, say so explicitly and mention residual risk or untested areas

Optional context:

- Changed files or PR scope: <paths>
- Expected behavior after the change: <details>
- Known risky layers: <details>

## API And Persistence Closure Review

Request:

- Review whether request fields, DTO changes, SQL changes, generated code, logic calls, handlers, and tests form a complete path
- Call out places where the change stops halfway, drifts from the API contract, or leaves orphaned code behind
- If the change touches rollout, migration, runbook, or cutover documents, call out whether those docs still look active or now belong in historical or archive status

Optional context:

- Endpoint or package: <path>
- Contract change details: <details>

Related docs:

- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`
- `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
- `.github/standards/backend/WORKFLOW_AND_VALIDATION.md`
- `.github/standards/backend/README.md`
- `.github/standards/domains/wechat-payment/README.md`
- `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`
- `.github/standards/backend/FORMAL_REVIEW_DURABILITY.md`
- If the change touches runbooks, execution plans, or cutover documents, say whether those docs still look active or should move to archive or historical status.