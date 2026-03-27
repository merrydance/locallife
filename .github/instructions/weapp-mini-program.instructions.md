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

## Full-Path Integrity Rules

- Any visible business action must be wired through the full path: service call, page state transition, success or failure feedback, and post-action refresh or reconciliation when needed.
- Do not present backend-affecting actions as completed when only local `setData` changed. If an operation changes merchant, rider, order, reservation, payment, or permission state, it must call the real API or be removed.
- Do not add or keep navigations to pages that are not registered in `app.json` or the relevant subpackage config.
- Do not paginate first and then apply business filters only on the client when that can distort list completeness, empty states, or `hasMore` behavior. Prefer server-side filtering or a query contract that preserves pagination correctness.
- Real-time pages must initialize subscriptions after async identity or status bootstrap completes, not only in `onShow`. Cold-start online states must still receive live updates.
- For high-traffic lists or home feeds, avoid per-item request fan-out on the first screen when a batched or progressive hydration contract can be used instead.
- Search and list pages must distinguish `empty` from `error`; a toast alone is not a complete failure state.

## High-Risk Anti-Patterns

- Dead routes: buttons or cards that navigate to missing pages.
- Fake success: success toast or optimistic copy without a real backend state change.
- Partial delivery: page UI added without connecting API, handler, state, and render branches together.
- Client-only truth: list totals, filtered counts, or permission states inferred locally in a way that can drift from backend truth.
- Cold-start realtime gaps: websocket or event listeners that only work after a second entry, manual refresh, or status toggle.
- First-screen request explosions: page boot logic that multiplies network requests per item and makes weak-network behavior unstable.

## Validation Defaults

- Run commands from `weapp/`.
- Common commands: `npm run compile`, `npm run lint`, `npm run lint:fix`, `npm run quality:check`.
- Prefer `npm run quality:check` before handing off changes that touch multiple Mini Program files.

## Scope Reminders

- Reuse existing local components and TDesign conventions before adding new primitives.
- Link to existing design and API docs instead of duplicating them in new markdown files.