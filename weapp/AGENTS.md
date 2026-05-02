# LocalLife Mini Program Agent Guide

This file applies to `/home/sam/locallife/weapp`.

## Scope

- WeChat Mini Program pages, components, services, styles, and quality gates.
- Backend, Web, and Flutter code live outside this scope. Follow their own `AGENTS.md` files when editing those areas.

## First Reads

For Mini Program changes, read the matching project-owned guidance instead of copying it here:

1. `../.github/instructions/weapp-mini-program.instructions.md`
2. `../.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
3. `../.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
4. `../.github/standards/weapp/README.md`

When the task touches visual structure or component choice, choose the role-matched visual standard:

- Consumer surfaces: `../.github/standards/weapp/DESIGN_SYSTEM.md`
- Merchant, operator, platform, rider, and other non-consumer surfaces: `../.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`

Use `../.github/prompts/weapp-implementation.prompt.md` for implementation requests and `../.github/prompts/weapp-review.prompt.md` for review requests.

## Working Rules

- Treat the backend contract as the sole source of truth for fields, statuses, permissions, pagination, and business semantics.
- Start from the user's task, task-domain owner, and ViewState before deciding page sections, cards, tabs, or component boundaries.
- Prefer TDesign Miniprogram components before adding user-visible local UI controls.
- Keep service calls, page state, handlers, WXML, WXSS, and feedback wired end to end.
- Explicitly handle weak network, re-entry, duplicate taps, and payment or async result recovery when the path is sensitive.

## Validation

Run commands from `weapp/` and choose the smallest relevant check first:

- If `node`, `npm`, or `npx` is not found, first prepend the local tool path: `PATH="$HOME/.local/bin:$PATH"`. This workspace has the Mini Program Node/npm toolchain available there.
- `npm run compile`
- `npm run lint`
- `npm run quality:check`
- `npm run gate:weapp`

In the handoff, state the risk class, the command run, any command not run, and any remaining weak-network, re-entry, duplicate-tap, or state-recovery risk.
