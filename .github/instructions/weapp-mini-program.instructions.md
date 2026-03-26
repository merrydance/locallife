---
applyTo: "weapp/**"
---

# Mini Program Instructions

Apply these rules for files under `weapp/`.

More specific Mini Program instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `weapp/miniprogram/pages/` and `weapp/miniprogram/components/`.

## Read First

- `.github/standards/weapp/DESIGN_SYSTEM.md`
- `.github/standards/weapp/api/README.md`

## Working Style

- Prefer existing TDesign-based patterns and local components before introducing new UI structure.
- Keep business-specific styles out of global app styles unless they are truly shared.
- Treat user-facing copy as product copy, not developer terminology.
- Check adjacent pages or components before creating a new pattern.

## UI Rules To Apply Directly

- Use the CSS tokens already defined in `app.wxss` instead of hardcoded color, spacing, or radius values.
- Prefer TDesign Miniprogram components for buttons, tags, images, inputs, dialogs, loading, and icons before building custom equivalents.
- Ensure each data-driven surface has explicit loading, success, empty, and error states.
- Avoid full-screen spinner-only loading patterns when a skeleton or structural placeholder is more appropriate.
- Keep business styles out of shared global styles and keep developer-facing wording out of user-visible copy.

## Validation Defaults

- Run commands from `weapp/`.
- Common commands: `npm run compile`, `npm run lint`, `npm run lint:fix`, `npm run quality:check`.
- Prefer `npm run quality:check` before handing off changes that touch multiple Mini Program files.

## Scope Reminders

- Reuse existing local components and TDesign conventions before adding new primitives.
- Link to existing design and API docs instead of duplicating them in new markdown files.