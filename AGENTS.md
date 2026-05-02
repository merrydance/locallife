# LocalLife Codex Instructions

This file is the Codex entrypoint for `/home/sam/locallife`.

## First Reads

Start with these two project-owned indexes before opening deeper guidance:

1. `.github/copilot-instructions.md`
2. `.github/README.md`

Use them as routing summaries. Do not bulk-load `.github/standards/**`, `.github/prompts/**`, or `.github/instructions/**`; open only the smallest matching files for the active task.

## Workspace Routing

- `locallife/`: Go backend API, workers, scheduler, SQL migrations, sqlc code, and integration tests.
- `merchant_app/`: Flutter merchant Android app.
- `web/`: Next.js web console.
- `weapp/`: WeChat Mini Program.
- `legal_exports/`: generated or exported legal-reference material.
- `artifacts/`: plans, audits, migration notes, and other task artifacts.

Before editing, identify the target area and apply the matching `.github/instructions/*.instructions.md` file. More specific instruction files override broad ones when their `applyTo` pattern matches.

## Prompt Routing

Treat `.github/prompts/` as reusable workflow templates, not as task archives.

- Use backend prompts for `locallife/` implementation, bugfix, payment-domain, SQL review, integration-test, takeover, and review-closure work.
- Use web, WeChat Mini Program, or Flutter prompts for frontend implementation and review work in `web/`, `weapp/`, or `merchant_app/`.
- Use general prompts only when the target area is unclear or the task spans multiple product areas.
- Update existing prompt templates rather than adding near-duplicates unless the user explicitly asks for a new reusable prompt.

## Backend Rules

For any non-trivial backend work under `locallife/`, also follow `locallife/AGENTS.md`.

High-risk backend paths include payment, refund, profit sharing, withdrawal, delivery, reservation, callbacks, workers, schedulers, idempotency, recovery, authn/authz, tenant boundaries, media visibility, and OCR. Classify these upward, trace the production path before editing, and use the matching domain README under `.github/standards/domains/` when relevant.

## Validation Baseline

- Run commands from the project directory that owns the change.
- Prefer the smallest relevant validation command first.
- Regenerate artifacts when source files require it, for example `make sqlc`, `make mock`, or `make swagger` in `locallife/`.
- In the final handoff, state what changed, what was validated, what was not validated, and any residual risk.

## Repository Skills

Repo-local Codex skills live under `.agents/skills/`. Use `locallife-prompt-router` when a task needs help selecting the correct `.github` instructions, prompts, standards, or validation path.

## Superpowers Compatibility

LocalLife project instructions are the primary workflow source in this repository. Use superpowers skills only as lightweight process aids when they add value.

- Use `superpowers:systematic-debugging` for real bugs, failing tests, or unexpected behavior.
- Use `superpowers:verification-before-completion` before making completion, fixed, or passing claims.
- Use planning or brainstorming skills only for broad, ambiguous, high-risk, or multi-step work that genuinely needs a separate design or plan.
- Do not let superpowers create duplicate source-of-truth documents, mandatory specs, commits, or worktrees unless the user explicitly asks.
- Do not duplicate `.github` rules into `docs/superpowers/`, `.codex/`, or another parallel prompt tree.
- Validation commands and routing decisions come from `AGENTS.md`, narrower directory `AGENTS.md` files, `.agents/skills/locallife-prompt-router`, and `.github/`.
