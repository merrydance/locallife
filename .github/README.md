# LocalLife AI Customization Index

This directory is the normalized entrypoint for AI-facing workspace rules, prompt templates, and workflow assets.

## Directory Layout

- `copilot-instructions.md`: workspace-wide default instructions.
- `standards/`: canonical project-owned engineering and domain standards.
- `instructions/`: auto-matched rules using `applyTo` patterns.
- `agents/`: narrowly scoped custom agents kept only when a dedicated tool boundary or read-only mode is required.
- `prompts/`: reusable prompt templates with normalized names.
- `workflows/`: workflow assets if the workspace later adds them.

## Naming Conventions

Use these naming patterns for new files under `.github/`:

- Instructions: `<scope>-<area>.instructions.md`
- Prompts: `<scope>-<intent>.prompt.md`
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
- Keep agent count minimal. At the moment, read-only Mini Program audit is the only agent-mode that justifies a separate boundary.

## Authoritative Source Strategy

The files under `.github/instructions/` are the AI-friendly, auto-matched layer.
They should summarize and operationalize the original project rules instead of replacing the original source documents.

Prefer this order when maintaining rules:

1. Keep the canonical domain or engineering standard in `.github/standards/`.
2. Reflect the operationally important parts in the matching `.github/instructions/*.instructions.md` file.
3. Link back to the original document when details are too long or domain-specific to duplicate.

## High-Value Original Sources

Backend:

- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Web:

- `.github/standards/web/WEB_UI_STANDARDS.md`
- `.github/standards/web/DESIGN_GUARDRAILS.md`
- `.github/standards/web/design-system.md`

Mini Program:

- `.github/standards/weapp/DESIGN_SYSTEM.md`
- `.github/standards/weapp/api/README.md`

Domain modules:

- `.github/standards/domains/media/*`
- `.github/standards/domains/ocr/*`
- `.github/standards/domains/wechat-payment/*`

## Recommended Read Order

1. Start with `copilot-instructions.md`.
2. Follow the matching file in `instructions/` for the current directory.
3. Use the matching prompt template in `prompts/` when the task is implementation, review, or integration-test related.
4. Open the linked project-owned source document when the task needs deeper domain detail.

## Maintenance Rule

When adding a new customization, update this index only if the new file changes routing behavior or introduces a new specialized workflow. If a new prompt competes with an existing one, narrow the descriptions first before adding another surface.

## Historical Material Rule

- New rollout plans, cutover checklists, release-only runbooks, migration playbooks, and task execution trackers may live next to active domain guidance only while they are still the current operating baseline.
- Once a rollout document becomes historical, move it under the matching domain `historical/` directory instead of leaving it in the default hot path.
- When moving a file to `historical/`, update the domain `README.md`, any matching `instructions/*.instructions.md`, and any prompt that still points at the old active path.
- Keep active guidance focused on long-lived design, security, operations, and validation rules that future code changes still need by default.