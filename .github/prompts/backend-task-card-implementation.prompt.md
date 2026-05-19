---
name: "后端任务卡实现模板"
description: "Use when drafting a backend implementation request from one task card. Trigger phrases: implement this task card, 任务卡, task card, 单张任务卡, fix one backend card, complete scoped card only, update card checkboxes after validation. 适用于按单张任务卡推进后端开发。"
---
# Backend Task Card Implementation Template

Use this template when asking for a concrete backend fix from one scoped task card.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the card scope before writing the request. Do not keep relying on stale context.

## Backend Task Card Fix

Request:

- Implement the fix described in task card: <task card path>
- Read the task card fully before changing code
- Keep the change scoped to this card only unless a small dependent change is strictly required
- Follow the existing handler/logic/db layering
- Reuse nearby patterns before introducing a new abstraction
- Tell me whether the change requires `make sqlc`, `make mock`, or `make swagger`
- Run the smallest relevant validation command and report what was executed
- Update the task card completion checkboxes only if the code change and validation are actually done

Required context:

- Task card: <path>
- Related phase map if applicable: <path>
- Target package or endpoint: <path>

Optional context:

- Existing reference implementation: <path>
- Closely related task card that should not be implemented now: <path>
- Related standards: `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Execution rules:

- Do not implement other cards in the same phase unless the requested card cannot be completed without a tiny supporting change
- If you discover the task card is too broad for one change, stop after finishing the smallest coherent slice and explain what remains
- Prefer fixing the root cause over patching the symptom
- Add or update the smallest relevant tests for the exact branch changed
- Do not rewrite unrelated docs or refactor unrelated modules

Required output:

- What changed
- What tests or validation were run
- Whether any checkbox in the task card can now be marked complete
- Remaining risks or follow-up work, if any

Acceptance checklist:

- The requested card scope is implemented without spilling into unrelated cards
- Affected layers are wired end-to-end where needed
- Tests cover the new branch, failure path, or state transition
- Regeneration steps were identified and run if required
- The task card can be updated honestly based on actual completion
