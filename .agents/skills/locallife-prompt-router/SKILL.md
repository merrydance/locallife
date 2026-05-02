---
name: locallife-prompt-router
description: Route LocalLife repository tasks to the correct project-owned .github instructions, prompt templates, standards, domain runbooks, and validation commands. Use when working in /home/sam/locallife, choosing between backend/web/weapp/flutter/general prompts, preparing implementation/review/bugfix/integration-test work, or updating the AI prompt system itself.
---

# LocalLife Prompt Router

## Purpose

Use this skill as the lightweight router for the LocalLife AI-facing prompt system. Keep `.github/` as the source of truth; this skill only decides what to read and when.

## Start Here

1. Read `AGENTS.md` at the repository root.
2. Read `.github/copilot-instructions.md` and `.github/README.md` for workspace routing.
3. Identify the target area: `locallife/`, `merchant_app/`, `web/`, `weapp/`, `legal_exports/`, `artifacts/`, or cross-cutting `.github/` work.
4. Open the smallest matching `.github/instructions/*.instructions.md` file before editing code.
5. Open the matching `.github/prompts/*.prompt.md` only when the task is implementation, bugfix, review, integration-test, takeover, incident follow-up, task loop, SQL review, or diagram work.
6. Open deeper `.github/standards/**` files only when the chosen instruction or prompt points to them, or when the task touches a high-risk area.

## Routing Rules

- Backend work under `locallife/`: follow `locallife/AGENTS.md`, then use `.github/instructions/backend-locallife.instructions.md` plus narrower backend instruction files for the active path.
- Backend implementation: use `.github/prompts/backend-implementation.prompt.md` unless a more specific backend prompt applies.
- Payment, refund, profit sharing, withdrawal, WeChat callbacks, or money movement: use `.github/prompts/backend-payment-domain.prompt.md` and `.github/standards/domains/wechat-payment/README.md`.
- SQL/schema/sqlc work: use `.github/instructions/backend-db-query.instructions.md`, `.github/instructions/backend-db-sqlc.instructions.md`, and `.github/prompts/backend-sql-review.prompt.md` when reviewing SQL.
- Backend production defects or regressions: use `.github/prompts/backend-bugfix.prompt.md`.
- Backend formal closure or review propagation: use `.github/prompts/backend-review-closure.prompt.md`.
- Web app work under `web/`: use `.github/instructions/web-ui.instructions.md` plus operator/merchant/shared UI instructions when matching, then `web-implementation` or `web-review` prompt.
- Mini Program work under `weapp/`: use `.github/instructions/weapp-mini-program.instructions.md`, then `weapp-implementation` or `weapp-review` prompt.
- Flutter merchant app work under `merchant_app/`: use `.github/instructions/flutter-merchant-app.instructions.md`, then `flutter-implementation`, `flutter-bugfix`, or `flutter-review` prompt.
- Cross-area work: use `general-implementation`, `general-review`, `general-task-loop`, or `general-incident-followup` prompts.
- Business process diagrams: use `.github/prompts/business-flow-mermaid.prompt.md`.
- AI prompt-system maintenance: read `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` before changing `.github/prompts/`, `.github/instructions/`, `.github/agents/`, or this skill.

## Validation Selection

- Choose validation from the target project's README, instruction file, or prompt template.
- For backend SQL changes, expect `make sqlc` and generated-code checks.
- For backend route or Swagger annotation changes, expect `make swagger`.
- For backend high-risk flows, prefer targeted tests plus safety checks called out by the relevant standard.
- For frontend apps, run the smallest relevant lint/build/test command from the app directory.
- If validation is not run, state why and what residual risk remains.

## Reference

For a compact file map, read `references/routing-map.md` when the first-read files do not make the right route obvious.
