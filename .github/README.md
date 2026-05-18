# LocalLife AI Customization Index

This directory is the normalized entrypoint for AI-facing workspace rules, prompt templates, and workflow assets.

## Directory Layout

- `copilot-instructions.md`: workspace-wide default instructions.
- `AGENTS.md`: Codex directory-local entrypoint for maintaining `.github` AI assets.
- `standards/`: canonical project-owned engineering and domain standards.
- `instructions/`: auto-matched rules using `applyTo` patterns.
- `agents/`: narrowly scoped custom agents kept only when a dedicated tool boundary or read-only mode is required.
- `prompts/`: reusable prompt templates with normalized names.
- `workflows/`: workflow assets and CI gate definitions used by this workspace.

Cross-cutting governance lives under `standards/engineering/` and should be treated as the parent baseline for security, consistency, resilience, validation, release readiness, and incident feedback loops that span multiple product areas.
AI-facing asset layering, Prompt gate rules, and implementation or review matrix reuse live in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md`.

## Naming Conventions

Use these naming patterns for new files under `.github/`:

- Instructions: `<scope>-<area>.instructions.md`
- Prompts: `<scope>-<intent>.prompt.md`
- Agents: `<workflow-or-domain>.agent.md`
- Codex directory guides: `AGENTS.md`
- Shared indexes: `README.md`

Examples:

- `backend-api.instructions.md`
- `backend-wechat.instructions.md`
- `web-operator-ui.instructions.md`
- `weapp-mini-program.instructions.md`
- `wechat-mini-program-audit.agent.md`
- `backend-review-closure.prompt.md`
- `general-review.prompt.md`
- `AGENTS.md`

## Routing Rules

Prefer the smallest customization primitive that solves the routing problem.

1. Use `instructions/` for always-on repository or path-scoped rules.
2. Use `prompts/` for implementation, review, refactor, test, or diagram requests.
3. Use `agents/` only when the mode needs a real boundary such as read-only analysis, a deliberately restricted tool set, or a clearly separate working persona.

Practical defaults for this workspace:

- Default to a prompt, not an agent.
- Treat prompt routing as layered, not flat: protocol prompts first, then stack prompts, then domain prompts, and only then agent or workflow boundaries.
- Use `general-` prompts only when the task spans multiple product areas or the target area is not yet clear.
- Prefer area-specific prompts over `general-` prompts once the target is clearly `locallife/`, `merchant_app/`, `web/`, or `weapp/`.
- Prefer specialized prompts such as payment, integration-test, task-card, or Mermaid only when the request explicitly matches that workflow.
- Do not create a new agent just to express expertise. Expertise belongs in prompt wording or instructions unless a tool boundary is required.
- Keep agent count minimal. In this workspace, default to prompts and instructions unless a future workflow adds real repository-backed agent files.
- Do not use `prompts/` as a task archive. One-off planning notes and temporary implementation breakdowns should stay in session state or an existing design document unless the user explicitly asks to persist them.
- When a prompt is worth keeping, prefer updating an existing reusable prompt over adding another near-duplicate file.
- Keep Codex-specific `AGENTS.md` files as thin routing guides that link to this `.github` source of truth instead of duplicating standards or prompts into a separate `.codex/` tree.

There is currently no repository-backed custom agent file set under `.github/agents/`; keep routing prompt-first unless a future workflow lands real agent assets.

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

- `.github/standards/backend/README.md`
- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/GO_PRACTICES.md`
- `.github/standards/backend/SQL_STANDARDS.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`
- `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
- `.github/standards/backend/WORKFLOW_AND_VALIDATION.md`
- `.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md`
- `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`
- `.github/standards/backend/FORMAL_REVIEW_DURABILITY.md`

Cross-cutting engineering governance:

- `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`
- `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`
- `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md`
- `.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md`
- `.github/standards/engineering/INCIDENT_FEEDBACK_LOOP.md`

Web:

- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/web/WEB_UI_STANDARDS.md`
- `.github/standards/web/DESIGN_GUARDRAILS.md`
- `.github/standards/web/design-system.md`

Mini Program:

- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
- `.github/standards/weapp/README.md`

Flutter Merchant App:

- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/flutter/README.md`
- `.github/standards/flutter/PRODUCTION_ROBUSTNESS_BASELINE.md`
- `.github/standards/flutter/FLUTTER_UI_DESIGN_STANDARDS.md`
- `.github/standards/flutter/TASK_ANNOTATION_TEMPLATE.md`
- `.github/standards/flutter/REVIEW_CHECKLIST.md`
- `.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md`
- `.github/standards/flutter/APP_AUTH_BINDING.md`
- `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`
- `.github/standards/flutter/ANDROID_KEEP_ALIVE_GUIDE.md`

Domain modules:

- `.github/standards/domains/media/*`
- `.github/standards/domains/ocr/*`
- `.github/standards/domains/wechat-payment/*`
- `.github/standards/domains/baofu-payment/*`

## Recommended Read Order

1. Start with `copilot-instructions.md`.
2. If the task is cross-cutting, high-risk, or about governance itself, read `standards/engineering/README.md` next.
3. If the task changes Prompt, Agent, Instruction, or AI-facing gate behavior, read `standards/engineering/AI_PROMPT_GOVERNANCE.md` before editing routing assets.
4. Follow the matching file in `instructions/` for the current directory.
5. Use the matching prompt template in `prompts/` when the task is implementation, review, or integration-test related.
6. Open the linked project-owned source document such as the governance baseline, validation matrix, area standard, or domain runbook only when the task needs deeper detail.

## Maintenance Rule

When adding a new customization, update this index only if the new file changes routing behavior or introduces a new specialized workflow. If a new prompt competes with an existing one, narrow the descriptions first before adding another surface.

Prompt, agent, instruction, and AI-facing README changes should pass the prompt governance gate in `.github/workflows/prompt-governance.yml` before merge.
When repository protection rules are available, require both `Prompt Governance Lint` and `Prompt Routing Tests` on the default branch.

## Historical Material Rule

- New rollout plans, cutover checklists, release-only runbooks, migration playbooks, and task execution trackers may live next to active domain guidance only while they are still the current operating baseline.
- Once a rollout document becomes historical, move it under the matching domain `historical/` directory instead of leaving it in the default hot path.
- When moving a file to `historical/`, update the domain `README.md`, any matching `instructions/*.instructions.md`, and any prompt that still points at the old active path.
- Keep active guidance focused on long-lived design, security, operations, and validation rules that future code changes still need by default.
- Completed governance rollout plans belong under `.github/standards/engineering/historical/`, not in the default engineering hot path.
