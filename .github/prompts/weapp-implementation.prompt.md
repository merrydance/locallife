---
name: "Mini Program Implementation Template"
description: "Use when drafting any Mini Program implementation request for weapp/, including normal page or component changes, diagnosis-first page方案 before coding, and payment-adjacent flows. Trigger phrases: update Mini Program page, 小程序页面, 小程序页面方案, build Mini Program page, create merchant page, 新建商户页面, 新建运营页面, 新建平台页面, fix component behavior, 列表空态和错误态, wire page state, improve weak-network UX, implement service-to-view change, setData 热点, 弱网体验, 小程序支付, 支付结果, login recovery after pay, duplicate tap guard, 重复点击支付."
---
# Mini Program Implementation Template

Use this template when asking for a concrete Mini Program change in `weapp/`.

This is the default implementation prompt for Mini Program page work, including new non-consumer pages, management surfaces, diagnosis-first page方案, and payment-adjacent flows.

Use the Mini Program row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and review-ready hand-off expectations.

## Mini Program Implementation Request

Request:

- Update or build <page or component>
- Follow `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` for the non-visual delivery baseline
- Run the smallest relevant validation command and report what was executed

Implementation must push:

- Start from the user task, first-screen essentials, and current-page boundary before coding or styling
- Treat the real backend contract as the only source of truth for fields, statuses, permissions, pagination, and metric meaning
- Keep information architecture and page boundary decisions ahead of component and styling choices
- Match TDesign components through TDesign MCP by purpose and component category before falling back to familiar local patterns
- Treat TDesign MCP documentation as the sole guidance source for TDesign component usage, supported props, supported styling methods, and limitations
- Wire service calls, page state, handlers, WXML, WXSS, and user-visible feedback end to end
- Keep page shell rhythm, safe-area handling, and small-screen usability consistent within the current role surface
- Keep non-consumer surfaces visually separate from consumer-side custom design language; only shell rhythm, spacing, and basic usability patterns should be shared by default
- Report the actual validation commands that were run and any residual risk that remains

Implementation must not do:

- Do not invent backend fields, states, permissions, metric semantics, or pagination conclusions
- Do not force unfinished, future, unsupported, or cross-role capabilities into the current page just to make it look complete
- Do not leave business-specific styles in global styles unless they are truly shared
- Do not carry customer-side brand colors, decorative token language, or marketing-style visual treatment into merchant, operator, platform, rider, or other non-consumer surfaces by default
- Do not override TDesign internals for page-local taste when official props, theme hooks, or page-level layout control would suffice
- Do not modify TDesign in ways the official documentation does not support
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

Delivery baseline:

- The page's primary task, first-screen essentials, and current-page boundary are explicit rather than implied by whatever the old page or draft layout happened to contain
- Layout structure, spacing rhythm, component composition, and safe-area handling follow existing page-shell patterns instead of ad-hoc local styling
- Page shell stays stable before data returns; no full-page white flash
- Loading, success, empty, and error states are all defined where relevant
- Refresh, retry, and re-entry behavior are deliberate where the task can span multiple states
- First-screen request scope, preloading, and foreground re-entry refreshes are controlled rather than left to default overfetch behavior
- New fields or actions are wired through service calls, page state, handlers, and user-visible feedback
- Request parameters, response fields, status enums, and types are aligned with the real backend contract; any adapter layer is explicit and does not invent backend truth
- When the page shows metrics, summaries, sorting labels, percentage copy, or explanatory notes, each one is aligned with the real backend source instead of page-local assumptions
- If backend semantics are ambiguous or required fields are missing, the request must explicitly state whether backend changes are needed instead of guessing on the frontend
- The page does not force unfinished, unsupported, or cross-role capabilities into the current surface just to make it look more complete
- Primary action is visually clear and duplicate-tap protection is explicit for backend-affecting actions
- Standard page buttons and tags do not use outline-style variants unless an explicit exception is documented for the task
- Token-based spacing, radius, and color variables are used instead of hardcoded values
- TDesign component selection is justified by task fit rather than habit; TDesign internals are not restyled for page-local visual preference, and unsupported override methods are not used
- Non-consumer pages do not drift into customer-side brand styling just because those tokens already exist in the app
- Shared component boundaries remain clean and business styles do not leak globally

Diagnosis-first mode:

- If the user asked for a page方案, do not jump straight to code. First establish page task, first-screen hierarchy, page boundary, backend truth, and TDesign-first UI composition.
- The diagnosis-first structure must cover: target page, user role, primary task, current problems, first-screen essentials, backend-source verification, problem diagnosis, proposed solution, UI delivery rules, implementation steps, non-goals, and validation plan.
- The page solution must keep UI consistency, small-screen usability, TDesign-first composition, and page shell stability as required deliverables instead of optional polish.
- The page solution must explicitly state whether the page belongs to consumer or non-consumer visual scope and avoid borrowing the wrong side's design language.

Payment-related mode:

- Login expiry and recovery are connected to the payment path
- Duplicate taps, stale polling, and delayed confirmation states are handled deliberately
- User-facing copy distinguishes success, failure, cancellation, and unknown result states
- Service calls, page state, event handlers, and view feedback stay wired end to end
- Leaving the app, returning from WeChat pay, or re-entering the page can reconnect to the correct payment state
- Unknown result states provide a credible next step such as status recheck, delayed confirmation guidance, or safe retry rules
- If backend payment state ownership, callback timing, or result semantics are ambiguous, the request must call out the backend gap explicitly and decide whether backend changes are needed before frontend implementation proceeds
- The same order or payment record is not shown as conflicting states across entry, result, and history surfaces