---
name: "后端阶段批处理实现模板"
description: "Use when drafting a controlled backend implementation request for one phase map. Trigger phrases: implement this phase, 阶段图, phase map, batch backend cards by phase, follow dependency order, complete work in small slices, phase-by-phase backend rollout. 适用于按阶段图分批推进后端任务。"
---
# Backend Phase Batch Implementation Template

Use this template when asking for a controlled batch implementation for one phase from a task-card set or phase map.

## Backend Phase Batch Fix

Request:

- Implement tasks for phase: <phase name>
- Use the phase map first: <phase map path>
- Follow the order and dependencies in the phase map instead of choosing an arbitrary path
- Complete the phase in small coherent slices, not one giant diff
- After each slice, run the smallest relevant validation and state what remains open
- Update task cards only for slices that are actually complete

Required context:

- Phase map: <path>
- Task cards included in this phase: <list of paths>
- Excluded task cards: <list of paths>

Optional context:

- Recommended first slice: <card or subtask>
- Existing reference implementation: <path>
- Related standards: `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Execution rules:

- Respect the phase map dependencies; do not jump ahead to downstream cards before upstream cards are stable
- Keep each implementation slice reviewable and testable on its own
- If one card turns out to need design clarification, pause that branch and continue only with independent cards in the same phase
- Do not silently widen scope into the next phase
- Prefer several small completions over one large risky refactor

Recommended workflow:

1. Read the phase map and all cards in the phase.
2. Pick the first smallest coherent slice.
3. Implement only that slice.
4. Run the smallest relevant validation.
5. Report completed checkboxes and remaining work.
6. Continue to the next slice only if the current one is stable.

Required output after each slice:

- Slice implemented
- Files changed
- Validation run
- Task cards or checkboxes now completed
- Next recommended slice

Acceptance checklist:

- Work follows the phase dependency order
- Each slice leaves the repo in a reviewable state
- Completed checkboxes correspond to real code and test completion
- No accidental spillover into later phases