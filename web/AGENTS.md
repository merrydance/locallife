# LocalLife Web Agent Guide

This file applies to `/home/sam/locallife/web`.

## Scope

- Next.js web console for operator and merchant surfaces.
- Backend, Mini Program, and Flutter code live outside this scope. Follow their own `AGENTS.md` files when editing those areas.

## First Reads

For Web changes, read the matching project-owned guidance instead of copying it here:

1. `../.github/instructions/web-ui.instructions.md`
2. `../.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
3. `../.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
4. `README.md`
5. `../.github/standards/web/WEB_UI_STANDARDS.md`
6. `../.github/standards/web/DESIGN_GUARDRAILS.md`
7. `../.github/standards/web/design-system.md`

When the active path matches a narrower surface, also read:

- `../.github/instructions/web-operator-ui.instructions.md` for `src/app/operator/**`.
- `../.github/instructions/web-merchant-ui.instructions.md` for `src/app/merchant/**`.
- `../.github/instructions/web-shared-ui.instructions.md` for `src/components/ui/**`.

Use `../.github/prompts/web-implementation.prompt.md` for implementation requests and `../.github/prompts/web-review.prompt.md` for review requests.

## Working Rules

- Start from the user's task and ViewState before mirroring backend API shapes into routes or components.
- Preserve the existing visual system and component patterns.
- Prefer existing components in `src/components/ui/` before creating new primitives.
- Keep page-level data fetching and API orchestration out of presentational components when nearby code already separates them.
- Map backend enums and errors to business-readable UI copy; do not expose raw provider, SQL, or diagnostic text to users.

## Validation

Run commands from `web/` and choose the smallest relevant check first:

- `npm run lint`
- `npm run build`

In the handoff, state the risk class, the command run, any command not run, and any residual UX or contract risk.
