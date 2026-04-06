---
applyTo: "weapp/**"
---

# Mini Program Instructions

Apply these rules for files under `weapp/`.

More specific Mini Program instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `weapp/miniprogram/pages/` and `weapp/miniprogram/components/`.

## Read First

- `.github/standards/engineering/README.md`
- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`
- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- `weapp/docs/miniprogram-prompt-system.md`

Use `.github/standards/engineering/README.md` as the governance index; use the Mini Program interaction, performance, and API contract docs as the default hot path for `weapp/` work.
When choosing components, inspect TDesign Miniprogram through the TDesign MCP component list and docs first: start from the component group that matches the task purpose, then narrow to the closest existing component before adding local UI.

## Risk Classification

- Treat visual-only or copy-only refinements as `G0` only when no event handling, state semantics, request timing, permission display, or user action behavior changes.
- Treat ordinary page or component work as `G1` when it does not affect dangerous actions, auth-sensitive state, complex async flows, or weak-network recovery behavior.
- Escalate to `G2` when the change touches page lifecycle state, login recovery, realtime updates, async job polling, duplicate-tap protection, cross-page data continuity, or weak-network fallback behavior.
- Escalate to `G3` when the change touches payment, identity, private materials, sensitive account data, authorization-sensitive surfaces, or any flow where state drift or duplicate action could create a high-impact production incident.

## Must-Follow Rules

- Treat the backend contract as the sole source of truth. Capabilities, fields, enums, permissions, pagination truth, and state semantics must come from the real backend contract, not from old page code or local assumptions.
- Prefer TDesign Miniprogram first. Use the TDesign MCP component list and docs to find components by task purpose and group before building a local component or wrapper.
- Keep page shell spacing consistent. The gap between page content and the top navigation must follow one stable pattern, left-right page gutters must stay consistent across pages, and bottom content or action areas must include safe-area handling.
- Treat user-visible copy as product copy, and never expose raw backend, provider, database, or English diagnostic text to users.
- Every data-driven surface must have clear first-screen states: loading, success, empty, and error. First-screen failure must stay visible in the page with retry.
- One user action gets one clear primary prompt. Pick the clearest single channel for the context; Toast is allowed, including a moderately longer duration when that makes the result easier to notice.
- Backend-affecting actions must call the real API, show visible in-flight state, prevent duplicate taps, and for `G2`/`G3` changes define timeout, retry, re-entry, refresh-after-action, and cold-start recovery behavior.
- Keep first-screen requests controlled. Avoid per-item detail fan-out, unnecessary cross-role preload, and low-probability subpage prefetch.

## High-Risk Anti-Patterns

- Fake truth: optimistic copy, local-only `setData`, fake permission states, or filtered totals that drift from backend truth.
- Partial delivery: UI was added but route, API, state, permissions, or render branches are not fully wired together.
- Invisible failure: first-screen failure only uses Toast, or service failure is rendered as empty state.
- Hidden async risk: duplicate submission remains clickable, realtime state only recovers after a second entry, or re-entry behavior is undefined.
- First-screen request explosion: per-item fan-out or low-value preload makes weak-network behavior unstable.
- Raw leakage: untranslated backend, SQL, provider, or English diagnostic text reaches the user.

## Validation Defaults

- Run commands from `weapp/`.
- Common commands: `npm run compile`, `npm run lint`, `npm run lint:fix`, `npm run quality:check`.
- Prefer `npm run quality:check` before handing off changes that touch multiple Mini Program files.
- In hand-off, state the risk class and any remaining weak-network, re-entry, duplicate-tap, or state-recovery risk using concrete paths.