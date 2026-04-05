---
applyTo: "weapp/miniprogram/pages/**"
---

# Mini Program Pages Instructions

Apply these rules for files under `weapp/miniprogram/pages/`.

## Role Of This Surface

- Keep pages focused on user, merchant, rider, operator, or platform workflows with clear state transitions and robust fallback states.
- Treat each page as an app-shell surface that should remain structurally stable while data is loading or refreshing.

## Page State Rules

- Define explicit loading, success, empty, and error states for data-driven pages.
- Prefer page composition through existing local components and TDesign primitives instead of large monolithic page templates.
- Keep page-level business styles local to the page unless they are genuinely shared.
- Make user-visible copy product-readable and role-appropriate for rider, merchant, operator, or consumer contexts.
- Distinguish first-screen loading, refresh loading, submit-in-flight, and silent resync failure instead of collapsing them into one `loading` flag.
- Preserve a stable page shell while async data is loading or reconciling; do not destroy the full page body just because one request is pending.

## Task Flow Rules

- Every page must make its primary task obvious within the first screen.
- One screen should have one clear primary action; secondary actions must not compete visually with the primary path.
- When a user returns from a detail page, login recovery, upload flow, or external payment hop, preserve as much valid page context as possible, including filters, tab state, and scroll position when practical.
- Weak-network recovery must stay inside the current task context; do not force the user to restart the whole path for a retryable failure.

## Data And Interaction Rules

- Keep service calls, event handlers, and state updates aligned with actual page responsibilities rather than scattering them into unrelated helpers.
- When a new field or action is added, thread it through page state, service calls, rendering, and empty or error messaging together.
- Preserve app-shell stability and avoid full-page flicker caused by overusing conditional destruction of the page body.
- Treat every page feature as a complete contract. If the page shows a switch, button, tab, order status, merchant state, payment result, or transfer action, verify the corresponding route, API call, permission check, state mutation, and refresh path all exist.
- Do not keep placeholder actions in production pages. If a page action is intentionally local-only, make that explicit in naming and copy; otherwise connect it to the backend.
- Do not route users into missing detail pages or unregistered subpackage pages.
- Do not let list filtering break pagination truth. If a page needs `order_type`, `status`, role, or region filtering, prefer pushing that filter into the request contract instead of trimming a paginated result afterward.
- For realtime workflow pages such as rider halls, merchant dashboards, task centers, or notification surfaces, confirm that cold start, reconnect, and foreground re-entry all restore live subscriptions correctly.

## Recovery Rules

- If the page already holds trusted data, refresh failure should preserve it and clearly indicate that the current view is based on the last successful sync.
- If a long task can outlive the page session, returning to the page should reconnect to the real task state instead of resetting to the initial state.
- Form pages should preserve unsaved but still valid input when retrying recoverable failures.
- Keyboard, picker, popup, and bottom action bars should not trap the user in an obscured or unreachable state during error recovery.
- Do not override TDesign internal classes for page-local visual preference; solve page differences first with tokens, outer layout, and approved shared styles.

## Validation Defaults

- Prefer `npm run quality:check` when page changes affect multiple files or role-specific workflows.