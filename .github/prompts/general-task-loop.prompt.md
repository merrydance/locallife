---
name: "任务清单闭环执行"
description: "Use when executing a task list through an implement -> review -> fix -> review -> docs sync loop until complete. Trigger phrases: run task list, handoff to review, fix findings, continue next task, close all tasks. 适用于把一组任务按开发、审查、修复、复审、文档同步的闭环方式逐项完成。"
argument-hint: "Provide target area, ordered task list, acceptance criteria, validation scope, and any doc-sync expectations."
agent: "Delivery Loop Orchestrator"
---
Run the provided task list through a closed delivery loop.

Required input:

- Target area: <backend, web, weapp, or mixed>
- Ordered task list: <task 1, task 2, task 3...>
- Acceptance criteria: <what counts as done>
- Risk notes: <known or suspected G0/G1/G2/G3 items, or say unknown>

Optional input:

- Validation scope or budget: <focused tests only, lint + build, compile only, etc.>
- Documentation expectations: <always sync docs, sync only if behavior changes, etc.>
- Stop conditions: <when to halt instead of continuing>

Execution rules:

- Work one task at a time.
- At the start of each task, classify the task risk as `G0`, `G1`, `G2`, or `G3` using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` when the classification is not already given.
- Choose validation depth and hand-off detail using `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.
- For each task, follow this order: implement -> validate -> review -> fix if needed -> review again -> docs sync decision -> next task.
- Do not skip review.
- For `G2` and `G3` tasks, do not advance to the next task until the hand-off explicitly states validation evidence, unverified failure paths, and any residual release or recovery risk.
- For UI tasks, review must treat approved design-system drift as a valid findings source, not as optional polish. This includes popup action-bar structure, equal-width bottom action buttons, forbidden outline defaults, non-essential TDesign internal overrides, and whether sibling pages in scope still fail to look like one coherent system.
- Do not skip the doc-sync decision after review passes, even if the result is no-op.
- After one task is accepted, immediately hand off to the next remaining task unless an explicit stop condition was provided.
- Do not treat a single accepted task as overall completion when the ordered task list still has pending items.
- Stop only when the full task list is complete or a concrete blocker prevents safe continuation.

Output expectations:

- Keep progress updates concise and stage-based.
- Surface review findings clearly before fixes.
- State the task risk level and the concrete residual risk, if any, before handing off to the next task.
- After each accepted task, explicitly state the next task handoff instead of ending the workflow.
- When blocked, explain which task is blocked and why.