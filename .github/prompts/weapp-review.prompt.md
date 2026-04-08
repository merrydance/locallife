---
name: "Mini Program Review Template"
description: "Use when drafting a Mini Program review request with findings-first output, including page review, payment-flow review, and overall upgrade audits. Trigger phrases: review Mini Program change, 小程序审查, inspect setData misuse, check page state propagation, audit weak-network UX, 整体升级角度审查, 交互和风格, findings first weapp review, review pay result state, 小程序支付审查, duplicate tap pay review."
---
# Mini Program Review Template

Use this template when asking for a Mini Program review in `weapp/`.

Use the Mini Program row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and findings-first review checks.

## Mini Program Review

Request:

- Review this change with findings first, ordered by severity
- Check it against `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`; when the task explicitly involves visual-system drift or component visual baseline, use the role-matched design document: consumer surfaces use `.github/standards/weapp/DESIGN_SYSTEM.md`; non-consumer surfaces use `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`
- Use `.github/standards/weapp/REVIEW_CHECKLIST.md` as the compact PR review checklist so the review covers both baseline conformance and user-facing coherence

Review must prioritize:

- Broken service-to-state-to-view wiring
- Missing page states, missing recovery paths, and weak-network regressions
- Backend-truth drift, fake local truth, and incomplete state propagation
- Business styles leaking into shared global styles or shared components
- Consumer-side design-language bleed into merchant, operator, platform, rider, or other non-consumer surfaces
- Approved design-system drift when the task explicitly touches visual-system behavior
- Debug leftovers, dead interactions, and half-connected handlers
- Unverified high-risk flows when payment, weak-network retry, realtime updates, login recovery, or duplicate-tap protection are involved

Review must not do:

- Do not lead with generic summaries before findings
- Do not spend most of the review on cosmetic trivia while runtime or contract risks remain unchecked
- Do not treat ambiguous backend semantics as harmless frontend freedom
- Do not mark high-risk flows as safe when validation evidence is missing
- Do not collapse baseline violations, upgrade opportunities, and residual risk into one vague paragraph
- Do not accept unsupported TDesign overrides as harmless styling detail

Required context:

- Changed page or component paths: <paths>

Optional context:

- Expected behavior: <details>
- User role and task goal: <details>
- High-frequency or weak-network sensitivity: <details>
- Review mode: <baseline conformance | overall upgrade audit>
- Reference page or component: <path>
- Validation evidence already run: <commands or none>

Review must check:

- New fields and actions propagate through service layer, state, handlers, and visible UI
- Request parameters, response fields, enums, and types stay aligned with the real backend contract instead of drifting from page-local assumptions
- Backend semantics are treated as the only source of truth; if the code is guessing around missing or ambiguous backend meaning, call that out as a finding or residual risk
- App Shell structure remains stable during loading and error states
- Page shell, outer gutter, nav gap, safe-area handling, and content-container spacing are established outside the inner TDesign controls rather than scattered across local wrappers
- TDesign component choice matches the task and can be justified by TDesign MCP-based component discovery rather than habit alone
- TDesign styling changes use only officially supported customization methods; internal class overrides or structure-dependent hacks are treated as findings unless explicitly approved
- Popup forms use a stable bottom action area instead of leaving action buttons inside scroll content tails
- Bottom popup dual actions render as equal-width block buttons and do not degrade into content-width small buttons
- Buttons and tags do not fall back to forbidden outline-style defaults unless an explicit exception is documented
- TDesign internals are not overridden for page-local visual preference when tokens, theme props, and shared layout patterns would suffice
- Non-consumer pages do not inherit consumer-side custom design language, branding colors, or decorative styling by default
- The review names the correct role-side design document when visual-system assertions depend on it rather than treating one design document as universal
- Sibling pages in the same task scope still read as one coherent system rather than a mix of competing local patterns
- User-facing copy and affordances are clear in weak-network and empty-data scenarios
- Primary and secondary actions remain visually and behaviorally clear
- Returning to the page, retrying, or foreground re-entry does not break the user's task context
- In overall upgrade audit mode, check whether the change still preserves low-quality patterns such as fake success, over-fragmented cards, stacked explanations, unstable first-screen request fan-out, or cross-page inconsistency that keeps the flow from feeling like one coherent system
- In payment-related review mode, explicitly check login recovery, duplicate taps, stale polling, delayed confirmation, unknown result handling, and cross-page state consistency between order, payment, result, and history surfaces

Output rules:

- Separate proven code defects from interaction defects when both exist
- In overall upgrade audit mode, also separate baseline violations from upgrade opportunities so the review can distinguish “must fix” from “should redesign”
- If a high-risk flow was changed but not actually validated, call it out as residual risk even when no direct bug is proven
- If the frontend behavior depends on backend semantics that remain ambiguous or missing, call out whether backend clarification or backend changes are required
- If there are no findings, say so explicitly and mention residual risks or validation gaps