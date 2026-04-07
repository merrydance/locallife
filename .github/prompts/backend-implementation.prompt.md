---
name: "后端实现请求模板"
description: "Use when drafting a normal backend implementation or bug-fix request for locallife/, outside payment-specialized or task-card-specialized flows. Trigger phrases: implement backend endpoint, 后端接口, handler 到 sqlc, fix handler logic wiring, update sqlc flow, add business rule, backend contract change. 适用于发起常规 Go 后端功能开发与缺陷修复任务。"
---
# Backend Implementation Template

Use this template when asking for a concrete backend change in `locallife/`.

Use the backend row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for must-push items, prohibited shortcuts, and review-ready hand-off expectations.

## Backend Feature Or Fix

Request:

- Implement <feature or fix>
- Keep the change complete across handler, logic, store, route, DTO, and tests when those layers are affected
- Reuse nearby patterns before introducing a new abstraction
- Use constants from `db/sqlc/constants.go` instead of new magic strings
- Tell me whether the change requires `make sqlc`, `make mock`, or `make swagger`
- Run the smallest relevant validation command and report what was executed
- Report which layers changed, which relevant validations were not run, and what residual risk remains if the path is not fully verified

Required context:

- Target package or endpoint: <path>
- Expected contract or behavior: <details>

Optional context:

- Existing reference implementation: <path>
- Related domain docs: `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Acceptance checklist:

- Request binding, auth extraction, and error mapping remain in handler only
- Business rules live in logic or a service, not in handler
- Persistence changes are wired through sqlc/store and actually used by logic
- Required generation steps were identified and run if source files changed
- Tests cover the new branch, failure path, or contract edge case
- The hand-off states what was verified, what was not verified, and where the execution path would still need confirmation