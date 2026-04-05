---
applyTo: "weapp/**"
---

# Mini Program Instructions

Apply these rules for files under `weapp/`.

More specific Mini Program instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `weapp/miniprogram/pages/` and `weapp/miniprogram/components/`.

## Read First

- `.github/standards/engineering/README.md`
- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/weapp/README.md`
- `weapp/docs/miniprogram-prompt-system.md`

Use `.github/standards/weapp/README.md` as the Mini Program standards index and `.github/standards/engineering/README.md` as the governance index; only drill into the individual standards when the active path needs the extra detail.

## Risk Classification

- Treat visual-only or copy-only refinements as `G0` only when no event handling, state semantics, request timing, permission display, or user action behavior changes.
- Treat ordinary page or component work as `G1` when it does not affect dangerous actions, auth-sensitive state, complex async flows, or weak-network recovery behavior.
- Escalate to `G2` when the change touches page lifecycle state, login recovery, realtime updates, async job polling, duplicate-tap protection, cross-page data continuity, or weak-network fallback behavior.
- Escalate to `G3` when the change touches payment, identity, private materials, sensitive account data, authorization-sensitive surfaces, or any flow where state drift or duplicate action could create a high-impact production incident.

## Must-Follow Rules

- Prefer TDesign Miniprogram native components before adding a local component or wrapper.
- When adding, replacing, or upgrading TDesign usage, check the official TDesign MCP or official docs first for component list, props, events, and changelog, then confirm the conclusion still matches the version pinned in `weapp/package.json`.
- Do not use outline or border-only affordances as the default style for primary actions or status tags. Low-emphasis button and tag variants are acceptable only for secondary classification, filters, or inline auxiliary actions with clear contrast and an explicitly weaker role.
- Use the design tokens already defined in `app.wxss`; do not hardcode one-off color, spacing, or radius values when an existing token can express the same meaning.
- Keep business-specific styles out of global app styles unless they are genuinely shared.
- Treat user-visible copy as product copy, not engineering terminology.
- Any data-driven surface must define explicit state handling for first-screen loading, success, empty, and error.
- First-screen failure must be visible inside page structure with retry; Toast-only failure handling is not complete.
- Do not display raw backend, gateway, provider, database, or English diagnostic text directly to users.
- A single user action should produce one primary prompt. If state change, navigation, or result page already explains the outcome, do not add another success Toast.
- Any backend-affecting action must call the real API or be removed from the UI. Pure local `setData` is not a valid substitute for a server-side state change.
- Treat the backend contract as the single source of truth for interface capability, field semantics, status enums, and types. Before using or changing an API, align the frontend request and response model with the real backend contract instead of guessing from existing page code.
- High-risk actions must have deliberate duplicate-tap protection and visible in-flight state.
- For `G2` and `G3` changes, define what happens on timeout, retry, re-entry, duplicate trigger, refresh after action, and cold-start recovery. Do not leave these behaviors implicit.
- Keep first-screen request count controlled. Do not preload subpages, cross-role surfaces, or per-item detail requests unless the preload has explicit high-probability benefit and safe weak-network degradation.
- Do not restyle TDesign internals for local visual preference. Default to tokens, theme props, spacing, safe-area handling, and outer layout composition; only make minimal internal overrides for verified platform or component limitations.

## High-Risk Anti-Patterns

- Dead routes: buttons or cards that navigate to missing pages.
- Fake success: success toast or optimistic copy without a real backend state change.
- Partial delivery: page UI added without connecting API, handler, state, and render branches together.
- Client-only truth: list totals, filtered counts, or permission states inferred locally in a way that can drift from backend truth.
- Raw server leakage: exposing untranslated backend, SQL, provider, or internal diagnostic strings directly to users.
- Cold-start realtime gaps: websocket or event listeners that only work after a second entry, manual refresh, or status toggle.
- First-screen request explosions: page boot logic that multiplies network requests per item and makes weak-network behavior unstable.
- TDesign drift: replacing a suitable TDesign component with custom UI, or changing TDesign internals for non-essential visual preference.
- Outline drift: using outline buttons or tags as a default page style, or letting border-only affordances carry primary action or key status semantics.
- Toast-only first-screen failure: the page has no inline recovery surface.
- Hidden duplicate submission: the request is in flight but the main action still looks idle and re-clickable.
- False empty state: service failure or loading gap is rendered as “暂无数据”.

## Validation Defaults

- Run commands from `weapp/`.
- Common commands: `npm run compile`, `npm run lint`, `npm run lint:fix`, `npm run quality:check`.
- Prefer `npm run quality:check` before handing off changes that touch multiple Mini Program files.
- In hand-off, state the risk class and any remaining weak-network, re-entry, duplicate-tap, or state-recovery risk using concrete paths.