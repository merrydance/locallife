# LocalLife AI Customization Index

This directory is the normalized entrypoint for AI-facing workspace rules, prompt templates, and workflow assets.

## Directory Layout

- `copilot-instructions.md`: workspace-wide default instructions.
- `standards/`: canonical project-owned engineering and domain standards.
- `instructions/`: auto-matched rules using `applyTo` patterns.
- `agents/`: narrowly scoped custom agents kept only when a dedicated tool boundary or read-only mode is required.
- `prompts/`: reusable prompt templates with normalized names.
- `workflows/`: workflow assets and CI gate definitions used by this workspace.

Cross-cutting governance lives under `standards/engineering/` and should be treated as the parent baseline for security, consistency, resilience, validation, release readiness, and incident feedback loops that span multiple product areas.

## Naming Conventions

Use these naming patterns for new files under `.github/`:

- Instructions: `<scope>-<area>.instructions.md`
- Prompts: `<scope>-<intent>.prompt.md`
- Agents: `<workflow-or-domain>.agent.md`
- Shared indexes: `README.md`

Examples:

- `backend-api.instructions.md`
- `backend-wechat.instructions.md`
- `web-operator-ui.instructions.md`
- `weapp-pages.instructions.md`
- `wechat-mini-program-audit.agent.md`
- `backend-review-closure.prompt.md`
- `general-review.prompt.md`

## Routing Rules

Prefer the smallest customization primitive that solves the routing problem.

1. Use `instructions/` for always-on repository or path-scoped rules.
2. Use `prompts/` for implementation, review, refactor, test, or diagram requests.
3. Use `agents/` only when the mode needs a real boundary such as read-only analysis, a deliberately restricted tool set, or a clearly separate working persona.

Practical defaults for this workspace:

- Default to a prompt, not an agent.
- Use `general-` prompts only when the task spans multiple product areas or the target area is not yet clear.
- Prefer area-specific prompts over `general-` prompts once the target is clearly `locallife/`, `web/`, or `weapp/`.
- Prefer specialized prompts such as payment, integration-test, task-card, or Mermaid only when the request explicitly matches that workflow.
- Do not create a new agent just to express expertise. Expertise belongs in prompt wording or instructions unless a tool boundary is required.
- Keep agent count minimal. In this workspace, read-only Mini Program audit and the delivery-loop orchestration workflow are the two user-facing agent modes that justify a separate boundary.
- The delivery workflow also uses three internal helper agents for implement, review, and doc-sync delegation. Treat those as supporting workflow parts, not as equal top-level routing modes.
- Do not use `prompts/` as a task archive. One-off planning notes and temporary implementation breakdowns should stay in session state or an existing design document unless the user explicitly asks to persist them.
- When a prompt is worth keeping, prefer updating an existing reusable prompt over adding another near-duplicate file.

Specialized workflow currently provided:

- `Delivery Loop Orchestrator`: executes an ordered task list through implement, review, fix, review, and doc-sync stages.

See `agents/README.md` for the active agent set and the distinction between user-facing agent modes and internal helper agents.

## Authoritative Source Strategy

The files under `.github/instructions/` are the AI-friendly, auto-matched layer.
They should summarize and operationalize the original project rules instead of replacing the original source documents.

Prefer this order when maintaining rules:

1. Keep the canonical domain or engineering standard in `.github/standards/`.
2. Reflect the operationally important parts in the matching `.github/instructions/*.instructions.md` file.
3. Link back to the original document when details are too long or domain-specific to duplicate.

Hot-path maintenance rule:

- Broad entrypoints such as `.github/README.md`, `copilot-instructions.md`, and app-wide instruction files should prefer stable governance indexes and the minimal active area standards set instead of enumerating long nested reading lists.
- Use long `Read First` enumerations only in narrower instructions when a specific subtree genuinely needs extra sources beyond the area index.

## High-Value Original Sources

Backend:

- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/GO_PRACTICES.md`
- `.github/standards/backend/SQL_STANDARDS.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Cross-cutting engineering governance:

- `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`
- `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`
- `.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md`
- `.github/standards/engineering/INCIDENT_FEEDBACK_LOOP.md`
- `.github/standards/engineering/HIGH_RISK_CHANGE_CHECKLISTS.md`

Web:

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/web/WEB_UI_STANDARDS.md`
- `.github/standards/web/DESIGN_GUARDRAILS.md`
- `.github/standards/web/design-system.md`

Mini Program:

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`
- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- `weapp/docs/miniprogram-prompt-system.md`

Domain modules:

- `.github/standards/domains/media/*`
- `.github/standards/domains/ocr/*`
- `.github/standards/domains/wechat-payment/*`

## Recommended Read Order

1. Start with `copilot-instructions.md`.
2. If the task is cross-cutting, high-risk, or about governance itself, read `standards/engineering/README.md` next.
3. Follow the matching file in `instructions/` for the current directory.
4. Use the matching prompt template in `prompts/` when the task is implementation, review, or integration-test related.
5. Open the linked project-owned source document such as the governance baseline, validation matrix, area standard, or domain runbook only when the task needs deeper detail.

## Maintenance Rule

When adding a new customization, update this index only if the new file changes routing behavior or introduces a new specialized workflow. If a new prompt competes with an existing one, narrow the descriptions first before adding another surface.

## Historical Material Rule

- New rollout plans, cutover checklists, release-only runbooks, migration playbooks, and task execution trackers may live next to active domain guidance only while they are still the current operating baseline.
- Once a rollout document becomes historical, move it under the matching domain `historical/` directory instead of leaving it in the default hot path.
- When moving a file to `historical/`, update the domain `README.md`, any matching `instructions/*.instructions.md`, and any prompt that still points at the old active path.
- Keep active guidance focused on long-lived design, security, operations, and validation rules that future code changes still need by default.
- Completed governance rollout plans belong under `.github/standards/engineering/historical/`, not in the default engineering hot path.