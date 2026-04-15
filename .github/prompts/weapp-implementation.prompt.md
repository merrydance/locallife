---
name: "Mini Program Implementation Template"
description: "Use when drafting any Mini Program implementation request for weapp/, including normal page or component changes, diagnosis-first page方案 before coding, payment-adjacent flows, and TDesign-first UI refactors. Trigger phrases: update Mini Program page, 小程序页面, 小程序页面方案, 小程序 UI 重构, TDesign 重构, TDesign-first 页面重写, 全页重构, 整页重新布局, 极简美学, build Mini Program page, create merchant page, 新建商户页面, 新建运营页面, 新建平台页面, fix component behavior, 列表空态和错误态, wire page state, improve weak-network UX, implement service-to-view change, setData 热点, 弱网体验, 小程序支付, 支付结果, login recovery after pay, duplicate tap guard, 重复点击支付, 组件拆分重构."
---
# Mini Program Implementation Template

Use this template when asking for a concrete Mini Program change in `weapp/`.

This is the default implementation prompt for Mini Program page work, including new non-consumer pages, management surfaces, diagnosis-first page方案, and payment-adjacent flows.

Use the Mini Program row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and review-ready hand-off expectations.
Use `.github/standards/weapp/README.md` as the weapp standards index, `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` as the default page-delivery baseline, and the role-matched design document for visual rules instead of restating the full standards body here.
Classify the task as `G0`, `G1`, `G2`, or `G3` using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`, then choose validation depth and residual-risk wording using `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

## Mini Program Implementation Request

Request:

- Update or build <page or component>
- State the task risk level (`G0`/`G1`/`G2`/`G3`) and why
- Follow `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` for the non-visual delivery baseline
- Use the role-matched visual standard explicitly: consumer surfaces use `.github/standards/weapp/DESIGN_SYSTEM.md`; merchant, operator, platform, rider, and other non-consumer surfaces use `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`
- Run validation that matches the risk level and report what was executed
- State which relevant paths remain unverified and what residual risk remains

Implementation must push:

- Start from backend-supported capabilities and the user task before coding or styling
- Treat the real backend contract as the only source of truth for fields, statuses, permissions, pagination, and metric meaning
- First inventory backend entities, fields, states, actions, permissions, and async outcomes; then group them into task domains before deciding any view structure
- Decide component boundaries before page polish: if a region owns local state, repeated add/remove/edit flows, dense editing UI, or reusable task-specific interaction, extract a dedicated domain component first
- Decide page boundaries after capability grouping and component boundaries: choose one page vs a page group by task continuity, first-screen clarity, information density, local-state complexity, and failure/recovery behavior
- Keep information architecture and page boundary decisions ahead of TDesign and styling choices, following the role-matched weapp standards instead of local page guesswork
- For TDesign-first refactor or style-reset requests, default to full-page information architecture and layout redesign rather than patching the legacy page shell unless the user explicitly asks for a local adjustment only
- Use TDesign MCP and the role-matched design standard to justify major component choices and any user-visible non-TDesign exception
- Keep first-screen copy structural and brief: push guidance down into labels, field notes, state strips, and action-adjacent copy instead of explanatory hero cards or standalone guide blocks unless the explanation itself is the task
- Keep non-consumer pages restrained: prefer page shell + content container + TDesign components, and make every extra local wrapper earn its keep through layout, state ownership, summary, or danger containment
- Default section-level and row-level local actions to TDesign icon buttons or icon-led small buttons; if a text-only local action remains, name the exception and why icon-led affordance would be misleading or insufficient
- Check TDesign MCP against the installed Mini Program component set before introducing any user-visible local control or wrapper exception, and state the exact component or supported composition chosen
- Wire service calls, page state, handlers, WXML, WXSS, and user-visible feedback end to end
- State which role-side design document governed the visual decisions and whether any exception crossed that boundary
- Report any user-visible area that still does not use TDesign, any backend-contract ambiguity, and any remaining weak-network, re-entry, duplicate-tap, or payment-state risk
- For native-to-TDesign replacement tasks, default to replacing native controls with TDesign equivalents where available (for example image->image, button->button, input->input, textarea->textarea, switch->switch, checkbox/radio groups, tag-like status), and explicitly list non-replaceable native tags that remain due platform capability gaps (commonly scroll-view, navigator, picker wrappers)

TDesign Miniprogram component inventory (MCP snapshot; choose from this list first):

- feedback: action-sheet, dialog, dropdown-menu, guide, loading, message, notice-bar, overlay, popover, popup, pull-down-refresh, swipe-cell, toast
- data: avatar, badge, cell, collapse, count-down, empty, footer, grid, image-viewer, image, progress, qrcode, result, skeleton, sticky, swiper, tag, watermark
- navigation: back-top, drawer, indexes, navbar, side-bar, steps, tab-bar, tabs
- base: button, divider, fab, icon, layout, link
- form: calendar, cascader, checkbox, color-picker, date-time-picker, form, input, picker, radio, rate, search, slider, stepper, switch, textarea, tree-select, upload

Implementation must not do:

- Do not invent backend fields, states, permissions, metric semantics, or pagination conclusions
- Do not map one backend interface to one page by default
- Do not decide page count, section layout, or component choice before capability grouping and page-boundary reasoning are explicit
- Do not keep adding capabilities to one page once it no longer has a single clear primary task
- Do not preserve the legacy page layout by default when the request is a refactor, redesign, style unification, or TDesign rewrite
- Do not jump from the old WXML structure straight to component selection without first inventorying the backend-supported capabilities and actions
- Do not force unfinished, future, unsupported, or cross-role capabilities into the current page just to make it look complete
- Do not spend first-screen budget on explanatory hero cards, guide cards, or stacked instruction blocks for single-task non-consumer pages
- Do not default to text-only local edit/delete/add/test/status buttons when an icon button or icon-led small button would communicate the action
- Do not wrap TDesign regions in extra local notice/card/panel shells unless the wrapper owns real layout, state, summary, warning, or danger responsibility
- Do not leave business-specific styles in global styles unless they are truly shared
- Do not carry customer-side brand colors, decorative token language, or marketing-style visual treatment into merchant, operator, platform, rider, or other non-consumer surfaces by default
- Do not override TDesign internals for page-local taste when official props, theme hooks, or page-level layout control would suffice
- Do not modify TDesign in ways the official documentation does not support
- Do not leave complex edit or composition regions inline in a page file once they already behave like standalone submodules
- Do not stop at WXML or WXSS changes when the task actually requires service, state, handler, or feedback changes
- Do not treat payment as just a button click if the task touches payment, result state, login recovery, or duplicate-tap protection

Required context:

- Target page or component path: <path>
- User role and target task: <consumer, merchant, rider, operator, platform, or other + what they are trying to finish>
- Desired behavior or UX change: <details>
- Success condition: <what should feel clearly better or become reliably correct>
- Backend contract source for any touched API: <swagger, backend handler/DTO, typed service contract, or explicit note that contract is still missing>

Optional context:

- Task frequency: <first-time, occasional, high-frequency>
- Weak-network or re-entry sensitivity: <details>
- State to preserve: <scroll position, filters, draft form, selected tab, local cache>
- Existing reference page or component: <path>
- Related service or API change: <details>
- Related backend payment callback or polling path: <path>
- Known weak-network or re-entry issue: <details>
- Payment preconditions or consent requirements: <details>

Acceptance focus:

- The hand-off names the task risk level, the role-side design document used, and the validation depth chosen for that risk
- The implementation is closed across service, state, handlers, render branches, feedback, and any affected payment or recovery path
- The first screen enters the task directly instead of opening with explanatory cards or stacked guidance copy
- Non-consumer local actions default to icon buttons or icon-led small buttons, and any text-only exception is called out explicitly
- If backend semantics are ambiguous or required fields are missing, the request states whether backend clarification or backend changes are needed instead of guessing in the page layer
- Any visual-system exception, non-TDesign exception, or remaining weak-network / re-entry / duplicate-tap / unknown-result risk is stated explicitly

TDesign-first refactor mode:

- If the request is to refactor UI, unify style, switch fully to TDesign, or rebuild a page in a minimalist direction, first decide whether the old page shell should be discarded instead of incrementally preserved
- Before coding, explicitly map backend-supported capabilities, states, and actions into page sections and first-screen priority rather than treating the existing DOM tree as the page architecture
- Use TDesign MCP to justify each major component choice by task fit; only fall back to native or local custom user-visible controls after recording why TDesign and supported outer composition do not satisfy the need
- Complex business areas should be split into dedicated components with explicit input, output, and page-owned orchestration boundaries rather than staying as one oversized page file
- Delivery notes for this mode must name the page sections that were relaid out, the TDesign components chosen for each major area, every deliberate exception from TDesign-first usage, and which explanatory copy or text-action patterns were removed, demoted, or retained with justification

Diagnosis-first mode:

- If the user asked for a page方案, do not jump straight to code. First establish backend capability inventory, capability grouping, component boundaries, page boundaries, first-screen hierarchy, backend truth, and then TDesign-first UI composition.
- The diagnosis-first structure must cover: backend capability inventory, capability grouping, target user role, primary task, proposed component boundaries, page vs page-group decision, first-screen essentials, backend-source verification, problem diagnosis, proposed solution, implementation steps, non-goals, risk level, and validation plan.
- The page solution must keep UI consistency, small-screen usability, TDesign-first composition, and page shell stability as required deliverables instead of optional polish.
- The page solution must explicitly state whether the page belongs to consumer or non-consumer visual scope and avoid borrowing the wrong side's design language.

Payment-related mode:

- Treat payment-adjacent work as at least `G2`, and use `G3` when identity, authorization-sensitive state, private materials, or high-impact duplicate action risk is involved
- Login expiry and recovery are connected to the payment path
- Duplicate taps, stale polling, and delayed confirmation states are handled deliberately
- User-facing copy distinguishes success, failure, cancellation, and unknown result states
- Service calls, page state, event handlers, and view feedback stay wired end to end
- Leaving the app, returning from WeChat pay, or re-entering the page can reconnect to the correct payment state
- Unknown result states provide a credible next step such as status recheck, delayed confirmation guidance, or safe retry rules
- If backend payment state ownership, callback timing, or result semantics are ambiguous, the request must call out the backend gap explicitly and decide whether backend changes are needed before frontend implementation proceeds
- The same order or payment record is not shown as conflicting states across entry, result, and history surfaces