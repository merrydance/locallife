---
applyTo: "web/**"
---

# Web Instructions

Apply these rules for files under `web/`.

More specific web instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `web/src/app/operator/`, `web/src/app/merchant/`, and `web/src/components/ui/`.

## Read First

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/web/WEB_UI_STANDARDS.md`
- `.github/standards/web/DESIGN_GUARDRAILS.md`
- `web/README.md`
- `.github/standards/web/design-system.md`

## Working Style

- Preserve the existing visual system and component patterns.
- Prefer existing components in `web/src/components/ui/` before creating new primitives.
- Do not hardcode one-off colors or typography tokens when a semantic utility already exists.
- Check the existing route segment and nearby pages before introducing a new layout pattern.
- Keep page-level data fetching and API logic out of presentational components when the codebase already separates them.

## UI Rules To Apply Directly

- Use the established `PageHeader + PageContent` page skeleton.
- Keep a single business flow within at most two card layers unless the task explicitly changes the visual system.
- Prefer the existing component white list and semantics from `.github/standards/web/WEB_UI_STANDARDS.md`: status filters should use `Select`, category switching should use `Tabs`, and list data should use the existing table patterns.
- Keep user-facing copy business-readable. Map backend enum values to readable labels instead of exposing raw enum strings.
- Treat feedback behavior as a system rule: no raw backend errors in UI, no redundant success prompt after navigation or structural page update, and no Toast-only handling for first-screen failures.
- Do not use developer-facing phrasing such as `debug`, `fallback`, `proxy`, or “与小程序一致” in operator-facing or merchant-facing UI copy.

## Validation Defaults

- Run commands from `web/`.
- Common commands: `npm run dev`, `npm run build`, `npm run lint`.
- Prefer the smallest relevant validation command for the area you changed.

## Scope Reminders

- Preserve established operator and merchant page patterns unless the task explicitly changes the design system.
- Link to existing UI docs instead of restating them in new markdown files.
