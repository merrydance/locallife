---
applyTo: "weapp/**"
---

# Mini Program Instructions

Apply these rules for files under `weapp/`.

More specific Mini Program instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `weapp/miniprogram/pages/` and `weapp/miniprogram/components/`.

## Read First

- `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`

Use `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` as the single non-visual hot path for `weapp/` page and component work.
When the task touches visual structure, page shell, spacing rhythm, or component visual baseline, choose the role-matched design document explicitly: consumer surfaces use `.github/standards/weapp/DESIGN_SYSTEM.md`; merchant, operator, platform, rider, and other non-consumer surfaces use `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`.
When choosing components, inspect TDesign Miniprogram through the TDesign MCP component list and docs first: start from the component group that matches the task purpose, then narrow to the closest existing component before adding local UI.
Use `.github/prompts/weapp-implementation.prompt.md` for all Mini Program implementation asks, including diagnosis-first page方案 and payment-adjacent flows. Use `.github/prompts/weapp-review.prompt.md` for all Mini Program review asks.

## Risk Classification

- Treat visual-only or copy-only refinements as `G0` only when no event handling, state semantics, request timing, permission display, or user action behavior changes.
- Treat ordinary page or component work as `G1` when it does not affect dangerous actions, auth-sensitive state, complex async flows, or weak-network recovery behavior.
- Escalate to `G2` when the change touches page lifecycle state, login recovery, realtime updates, async job polling, duplicate-tap protection, cross-page data continuity, or weak-network fallback behavior.
- Escalate to `G3` when the change touches payment, identity, private materials, sensitive account data, authorization-sensitive surfaces, or any flow where state drift or duplicate action could create a high-impact production incident.

## Always-On Rules

- Treat the backend contract as the sole source of truth. Do not invent fields, states, permissions, pagination conclusions, or business semantics in page code.
- Start from the user task and current-page boundary before layout or styling. First-screen essentials come before secondary capability coverage.
- For UI refactor, style unification, or TDesign-first rebuild requests, default to a whole-page information architecture and layout rethink rather than a local patch unless the user explicitly limits scope.
- Decide page structure in this order: backend-supported capabilities and states, first-screen hierarchy and section boundaries, TDesign MCP component selection, component decomposition, then implementation.
- Establish the page shell, outer page gutter, nav gap, safe-area handling, and content-container padding outside TDesign controls before composing the page's inner TDesign components.
- Prefer TDesign Miniprogram first. User-facing controls should stay in the TDesign system unless a suitable component genuinely does not exist.
- Treat TDesign MCP documentation as the sole guidance source for TDesign component usage, supported props, and allowed extension methods.
- Consumer surfaces may use the consumer-side design language from `.github/standards/weapp/DESIGN_SYSTEM.md`; non-consumer surfaces default to the restrained TDesign-first system from `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`.
- Complex regions with independent local state, repeated add/remove/edit flows, or dense task-specific layout should be extracted into dedicated domain components rather than left inline in the page shell.
- Section-level and row-level add/remove actions should default to TDesign icon buttons or icon-led small buttons; text-only add/remove buttons require an explicit exception.
- Do not carry consumer-side custom design language into merchant, operator, platform, rider, or other non-consumer surfaces unless a standard explicitly allows it.
- Keep page shell rhythm consistent: stable top spacing below navbar, stable horizontal gutter, and explicit bottom safe-area handling.
- Shared components must remain reusable and explicit. Do not hide page-level API calls, route jumps, or role-specific state machines inside shared components.
- User-facing copy must be product copy. Do not leak raw backend, provider, SQL, or English diagnostic text.
- Use the implementation or review prompt to carry detailed delivery structure; keep this instruction as the always-on execution baseline.

## High-Risk Anti-Patterns

- Fake truth: optimistic copy, local-only `setData`, fake permission states, or filtered totals that drift from backend truth.
- Boundary drift: stuffing future capabilities, unsupported backend gaps, or cross-role capabilities into the current page just to make it look more complete.
- Legacy-shell anchoring: keeping the old page layout and only swapping a few controls even though the request is explicitly for a TDesign-first refactor, style reset, or full-page relayout.
- Partial delivery: UI was added but route, API, state, permissions, or render branches are not fully wired together.
- Invisible failure: first-screen failure only uses Toast, or service failure is rendered as empty state.
- Hidden async risk: duplicate submission remains clickable, realtime state only recovers after a second entry, or re-entry behavior is undefined.
- First-screen request explosion: per-item fan-out, low-value preload, or cross-role warmup makes weak-network behavior unstable.
- Shared-component overreach: reusable components absorb page-only orchestration or side effects.
- Consumer-language bleed: non-consumer surfaces inherit customer-side brand colors, decorative token usage, or marketing-style visual language without explicit approval.
- Unsupported TDesign override: internal class overrides, structure-dependent styling, or behavior changes not supported by official TDesign docs.
- Text-action drift: section headers, list rows, repeaters, or local toolbars use plain text add/remove buttons even though a TDesign icon button would express the action more clearly and consistently.
- Raw leakage: untranslated backend, SQL, provider, or English diagnostic text reaches the user.

## Validation Defaults

- Run commands from `weapp/`.
- Common commands: `npm run compile`, `npm run lint`, `npm run lint:fix`, `npm run quality:check`.
- Prefer `npm run quality:check` before handing off changes that touch multiple Mini Program files or shared components.
- In hand-off, state the risk class and any remaining weak-network, re-entry, duplicate-tap, or state-recovery risk using concrete paths.