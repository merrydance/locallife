---
applyTo: "weapp/miniprogram/pages/**"
---

# Mini Program Pages Instructions

Apply these rules for files under `weapp/miniprogram/pages/`.

## Role Of This Surface

- Keep pages focused on user, merchant, rider, operator, or platform workflows with clear state transitions and robust fallback states.
- Treat each page as an app-shell surface that should remain structurally stable while data is loading or refreshing.

## Core Rules

- Make the first screen task obvious and keep one clear primary action.
- Keep page shell and states stable. Distinguish first-screen loading, refresh, submit-in-flight, and silent resync failure instead of collapsing everything into one `loading` flag.
- Keep page shell spacing uniform: page content must keep a consistent gap below the top navigation, consistent left-right gutters, and explicit bottom safe-area space.
- Thread every new field or action through service calls, page state, handlers, render branches, route/API/permission checks, and refresh paths together.
- Preserve valid task context on return, weak-network retry, foreground re-entry, and long-task recovery; realtime pages must restore subscriptions to the real state.
- Do not ship placeholder actions, dead routes, or list filtering that breaks pagination truth.

## Recovery Details

- If trusted data already exists, refresh failure should preserve it and explain that the view reflects the last successful sync.
- Form pages should preserve unsaved but still valid input when retrying recoverable failures.
- Keyboard, picker, popup, and bottom action bars should not trap the user in an obscured or unreachable state during recovery.

## Validation Defaults

- Prefer `npm run quality:check` when page changes affect multiple files or role-specific workflows.