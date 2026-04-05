---
name: "小程序实现请求模板"
description: "Use when drafting a generic Mini Program page or component implementation request for weapp/, outside payment-specialized flows. Trigger phrases: update Mini Program page, fix component behavior, wire page state, improve weak-network UX, implement service-to-view change. 适用于发起常规微信小程序页面与组件实现任务。"
---
# Mini Program Implementation Template

Use this template when asking for a concrete Mini Program change in `weapp/`.

## Mini Program Page Or Component Change

Request:

- Update <page or component>
- Follow `.github/standards/weapp/DESIGN_SYSTEM.md`, `.github/standards/weapp/INTERACTION_STANDARDS.md`, and `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- Treat every page task as requiring explicit visual, interaction, and performance consideration, with backend contract truth as the hard boundary
- Reuse existing TDesign-based patterns first
- Keep business-specific styles out of global styles unless they are truly shared
- Run the smallest relevant validation command and report what was executed

Required context:

- Target page or component path: <path>
- User role and target task: <consumer, merchant, rider, operator + what they are trying to finish>
- Desired behavior or UX change: <details>
- Success condition: <what should feel clearly better or become reliably correct>
- Backend contract source for any touched API: <swagger, backend handler/DTO, typed service contract, or explicit note that contract is still missing>

Optional context:

- Task frequency: <first-time, occasional, high-frequency>
- Weak-network or re-entry sensitivity: <details>
- State to preserve: <scroll position, filters, draft form, selected tab, local cache>
- Existing reference page or component: <path>
- Related service or API change: <details>

Acceptance checklist:

- Layout structure, spacing rhythm, component composition, and safe-area handling follow existing page-shell patterns instead of ad-hoc local styling
- Page shell stays stable before data returns; no full-page white flash
- Loading, success, empty, and error states are all defined where relevant
- Refresh, retry, and re-entry behavior are deliberate where the task can span multiple states
- First-screen request scope, preloading, and foreground re-entry refreshes are controlled rather than left to default overfetch behavior
- New fields or actions are wired through service calls, page state, handlers, and user-visible feedback
- Request parameters, response fields, status enums, and types are aligned with the real backend contract; any adapter layer is explicit and does not invent backend truth
- Primary action is visually clear and duplicate-tap protection is explicit for backend-affecting actions
- Standard page buttons and tags do not use outline-style variants unless an explicit exception is documented for the task
- Token-based spacing, radius, and color variables are used instead of hardcoded values
- TDesign internals are not restyled for page-local visual preference; any internal override is minimal and justified by a verified limitation
- Shared component boundaries remain clean and business styles do not leak globally