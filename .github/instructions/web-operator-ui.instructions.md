---
applyTo: "web/src/app/operator/**"
---

# Web Operator UI Instructions

Apply these rules for files under `web/src/app/operator/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Surface

- Keep operator pages optimized for operational clarity, dense but readable scanning, and explicit state handling.
- Favor workflows that help operators review, approve, reject, route, or inspect business entities without ambiguity.

## Layout And Interaction Rules

- Use the established `PageHeader + PageContent` shell and keep the main workflow within two card layers.
- Use `Tabs` for category or mode switching and `Select` for enumerated filters.
- Keep table, audit, rules, and review screens aligned with existing operator pages before inventing a new page structure.
- Prefer visible filters, status badges, and empty states that help operators understand why a record is missing or unavailable.

## Copy And Field Rules

- Use business-readable labels for statuses, review outcomes, exception reasons, and audit states.
- Do not expose internal identifiers, debug wording, or implementation-origin phrasing unless the page is explicitly an internal diagnostic surface.
- New data or status fields should thread through API, page state, rendering state, and operator-facing copy together.

## Validation Defaults

- Prefer `npm run lint` for focused validation unless the page change affects broader app structure.