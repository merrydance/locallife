# LocalLife GitHub AI Asset Guide

This file applies to `/home/sam/locallife/.github`.

## Purpose

Use this as the Codex directory-local entrypoint for maintaining AI-facing assets.
Keep `.github/` as the source of truth for standards, prompts, instructions, workflows, and gate scripts.
Do not copy these rules into a separate `.codex/` tree; add thin `AGENTS.md` or skill routing links instead.

## First Reads

Read these before changing AI-facing assets:

1. `.github/README.md`
2. `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md`

Then open the smallest matching index for the active change:

- `.github/instructions/README.md` for path-scoped instruction changes.
- `.github/prompts/README.md` for prompt template changes.
- `.github/agents/README.md` for custom agent changes.
- `.github/standards/README.md` for canonical standards changes.

## Working Rules

- Keep long-lived rules in `.github/standards/` first, then mirror only high-frequency operational rules into instructions or prompts.
- Keep `AGENTS.md` files thin: route Codex to the right `.github` source instead of duplicating full standards.
- Treat `.github/prompts/` as reusable templates, not a task archive.
- Prefer updating an existing prompt or instruction over adding a near-duplicate.
- Add or update routing tests when a prompt introduces a new trigger surface.
- Keep workflow job names stable when governance docs require branch protection checks by name.

## Validation

For changes to prompts, instructions, agents, standards, workflows, or this file, run from the repository root when local tooling is available:

- `node .github/scripts/prompt_governance_lint.js`
- `node .github/scripts/prompt_routing_test.js`

If `node` is unavailable locally, state that explicitly and rely on `.github/workflows/prompt-governance.yml` to run the same checks with Node 20 in CI.
