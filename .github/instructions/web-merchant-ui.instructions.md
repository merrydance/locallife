---
applyTo: "web/src/app/merchant/**"
---

# Web Merchant UI Instructions

Apply these rules for files under `web/src/app/merchant/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Surface

- Keep merchant pages focused on day-to-day operations such as orders, dishes, finance, reservations, tables, and marketing workflows.
- Favor task completion, field clarity, and safe editing flows over ornamental UI variation.

## Layout And Interaction Rules

- Use the existing merchant page shell and nearby merchant pages as the default reference before introducing a new layout.
- Keep editing flows, detail flows, and list flows visually consistent with existing merchant dashboards and management pages.
- Prefer reusable ui components and existing panel or list patterns for catalog, finance, and reservations screens.
- Ensure loading, empty, and error states are explicit for data-heavy merchant pages.

## Copy And Field Rules

- Use merchant-facing business language for settings, finance, inventory, and order status displays.
- Map backend enums and flags to readable merchant labels instead of exposing raw values.
- Editable fields must align with actual backend mutability and validation rules.

## Validation Defaults

- Prefer `npm run lint` for focused validation unless the change affects shared layout or build-time behavior.