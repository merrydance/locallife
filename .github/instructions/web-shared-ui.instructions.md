---
applyTo: "web/src/components/ui/**"
---

# Web Shared UI Instructions

Apply these rules for files under `web/src/components/ui/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Layer

- Keep this directory for reusable UI primitives and shared presentation building blocks.
- Do not move page-specific business logic, API fetching, or role-specific workflows into shared UI primitives.

## Component Rules

- Preserve existing naming, props, and styling patterns so shared components remain predictable across operator, merchant, and platform surfaces.
- Favor composability and semantic props over one-off business-specific variants.
- If a new primitive is added, it should be reusable across multiple pages or flows rather than solving a single isolated screen.
- Keep accessibility, state styling, and selected or active states explicit where the component semantics require them.

## Validation Defaults

- Prefer `npm run lint` after changing shared UI primitives because the blast radius crosses multiple route surfaces.