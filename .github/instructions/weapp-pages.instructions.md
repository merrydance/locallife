---
applyTo: "weapp/miniprogram/pages/**"
---

# Mini Program Pages Instructions

Apply these rules for files under `weapp/miniprogram/pages/`.

## Role Of This Surface

- Keep pages focused on user, merchant, rider, operator, or platform workflows with clear state transitions and robust fallback states.
- Treat each page as an app-shell surface that should remain structurally stable while data is loading or refreshing.

## Page Rules

- Define explicit loading, success, empty, and error states for data-driven pages.
- Prefer page composition through existing local components and TDesign primitives instead of large monolithic page templates.
- Keep page-level business styles local to the page unless they are genuinely shared.
- Make user-visible copy product-readable and role-appropriate for rider, merchant, operator, or consumer contexts.

## Data And Interaction Rules

- Keep service calls, event handlers, and state updates aligned with actual page responsibilities rather than scattering them into unrelated helpers.
- When a new field or action is added, thread it through page state, service calls, rendering, and empty or error messaging together.
- Preserve app-shell stability and avoid full-page flicker caused by overusing conditional destruction of the page body.
- Treat every page feature as a complete contract. If the page shows a switch, button, tab, order status, merchant state, payment result, or transfer action, verify the corresponding route, API call, permission check, state mutation, and refresh path all exist.
- Do not keep placeholder actions in production pages. If a page action is intentionally local-only, make that explicit in naming and copy; otherwise connect it to the backend.
- Do not route users into missing detail pages or unregistered subpackage pages.
- Do not let list filtering break pagination truth. If a page needs `order_type`, `status`, role, or region filtering, prefer pushing that filter into the request contract instead of trimming a paginated result afterward.
- For realtime workflow pages such as rider halls, merchant dashboards, task centers, or notification surfaces, confirm that cold start, reconnect, and foreground re-entry all restore live subscriptions correctly.
- For weak-network flows, provide a stable retry surface inside the page body. Toast-only failure handling is insufficient for first-screen data or core transaction actions.

## Validation Defaults

- Prefer `npm run quality:check` when page changes affect multiple files or role-specific workflows.