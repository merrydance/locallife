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
When the task is a non-consumer page cleanup, TDesign-first refactor, or style convergence pass, also read `.github/standards/weapp/NON_CONSUMER_PAGE_EXECUTION_CHECKLIST.md` as the compressed execution checklist.
When choosing components, inspect TDesign Miniprogram through the TDesign MCP component list and docs first: start from the component group that matches the task purpose, then narrow to the closest existing component before adding local UI.
Use `.github/prompts/weapp-implementation.prompt.md` for all Mini Program implementation asks, including diagnosis-first page方案 and payment-adjacent flows. Use `.github/prompts/weapp-review.prompt.md` for all Mini Program review asks.

## Risk Escalation

- Use the risk model from `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` instead of redefining a parallel weapp-only grading table here.
- If the change touches payment, identity, private materials, authorization-sensitive surfaces, async confirmation, realtime recovery, duplicate-tap protection, login recovery, or cross-page state continuity, treat it as high-risk and follow the engineering validation and release rules.

## Always-On Rules

- Treat the backend contract as the sole source of truth. Do not invent fields, states, permissions, pagination conclusions, or business semantics in page code.
- Start from backend-supported capabilities and the user's real task before layout or styling. First-screen essentials come before secondary capability coverage.
- For UI refactor, style unification, or TDesign-first rebuild requests, default to a whole-page information architecture and layout rethink rather than a local patch unless the user explicitly limits scope.
- Use this default sequence for weapp development: inventory backend entities, fields, states, actions, permissions, and async outcomes; group them into user task domains; decide which task domains need dedicated domain components; decide whether those task domains belong in one page or a page group; then choose TDesign composition; then implement.
- Do not map one backend interface to one page by default, and do not pack every available capability into one oversized page just because the entity is the same.
- Page boundaries come after capability composition. Decide one page vs a page group by task continuity, first-screen clarity, information density, local-state complexity, and failure/recovery behavior.
- Component boundaries come before page polish. If a region already owns local state, repeated add/remove/edit flows, dense editing UI, or reusable task-specific interaction, split it into a domain component before expanding the page shell.
- Establish the page shell, outer page gutter, nav gap, safe-area handling, and content-container padding outside TDesign controls before composing the page's inner TDesign components.
- Prefer TDesign Miniprogram first. User-facing controls should stay in the TDesign system unless a suitable component genuinely does not exist.
- Treat TDesign MCP documentation as the sole guidance source for TDesign component usage, supported props, and allowed extension methods.
- If TDesign already provides the component or an officially supported outer composition, keep LocalLife customization at the page-shell, layout, and shared-token level instead of adding a new user-visible local shell.
- Consumer surfaces may use the consumer-side design language from `.github/standards/weapp/DESIGN_SYSTEM.md`; non-consumer surfaces default to the restrained TDesign-first system from `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`.
- Section-level and row-level add/remove actions should default to TDesign icon buttons or icon-led small buttons; text-only add/remove buttons require an explicit exception.
- Do not carry consumer-side custom design language into merchant, operator, platform, rider, or other non-consumer surfaces unless a standard explicitly allows it.
- Non-consumer surfaces should not import `styles/customer.wxss` or borrow customer-side brand tokens unless the governing standard explicitly allows that boundary exception.
- Keep page shell rhythm consistent: stable top spacing below navbar, stable horizontal gutter, and explicit bottom safe-area handling.
- Shared components must remain reusable and explicit. Do not hide page-level API calls, route jumps, or role-specific state machines inside shared components.
- Do not keep expanding known super service files such as `services/operator-console.ts`. New capability work must first choose a task-domain owner, dedicated service module, or workflow owner before touching a protected aggregator.
- If a protected super service must change, update its ownership note in the same change so the page-group owner, temporary scope, and non-extraction reason stay explicit.
- User-facing copy must be product copy. Do not leak raw backend, provider, SQL, or English diagnostic text.
- Default to no explanatory copy. First make the page understandable through information architecture, labels, state, and actions; only add a sentence when omitting it would hide risk, state meaning, field constraints, or the next required action.
- Do not repeat the same explanation across title, subtitle, note, notice bar, and card body, and do not add page-boundary copy such as “this page is mainly for...” unless the explanation itself is the task.
- Use the implementation or review prompt to carry detailed delivery structure; keep this instruction as the always-on execution baseline.

## High-Risk Anti-Patterns

- Fake truth: optimistic copy, local-only `setData`, fake permission states, or filtered totals that drift from backend truth.
- Boundary drift: stuffing future capabilities, unsupported backend gaps, or cross-role capabilities into the current page just to make it look more complete.
- Interface-to-page mapping: turning each backend interface into its own page without first checking whether those capabilities should be combined into one user task.
- Capability pile-up: keeping unrelated or low-frequency abilities in one page even after the page no longer has a single clear primary task.
- Legacy-shell anchoring: keeping the old page layout and only swapping a few controls even though the request is explicitly for a TDesign-first refactor, style reset, or full-page relayout.
- View-first design: choosing page count, section layout, or TDesign controls before capability grouping, component boundaries, and page boundaries are understood.
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
- `npm run gate:weapp` runs the configured gate set for page-shell, WXML expression safety, component policy, tdesign-component-declarations, tdesign-boundary, non-consumer-ui-patterns, page-responsibility, page-complexity, super-service-boundary, request-boundary, role-contract, and business-status-boundary; note that `gate:non-consumer-ui-patterns` is currently changed-only so new drift is blocked while historical cleanup continues.
- Prefer `npm run quality:check` before handing off changes that touch multiple Mini Program files or shared components.
- In hand-off, follow the engineering governance wording for risk and residual risk, and still call out any remaining weak-network, re-entry, duplicate-tap, or state-recovery risk using concrete paths.