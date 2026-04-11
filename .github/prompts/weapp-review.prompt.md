---
name: "Mini Program Review Template"
description: "Use when drafting a Mini Program review request with findings-first output, including page review, payment-flow review, and overall upgrade audits. Trigger phrases: review Mini Program change, 小程序审查, inspect setData misuse, check page state propagation, audit weak-network UX, 整体升级角度审查, 交互和风格, findings first weapp review, review pay result state, 小程序支付审查, duplicate tap pay review."
---
# Mini Program Review Template

Use this template when asking for a Mini Program review in `weapp/`.

Use the Mini Program row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and findings-first review checks.
Use `.github/standards/weapp/README.md` as the standards index, `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` as the default review baseline, and `.github/standards/weapp/REVIEW_CHECKLIST.md` as the compact checklist instead of copying the full weapp standards body into the prompt.
Infer or state the task risk level (`G0`/`G1`/`G2`/`G3`) using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`, then scale validation and residual-risk expectations with `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

## Mini Program Review

Request:

- Review this change with findings first, ordered by severity
- Infer or confirm the task risk level (`G0`/`G1`/`G2`/`G3`) and call out when the implementation treated a clearly higher-risk path as routine
- Check it against `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`; when the task explicitly involves visual-system drift or component visual baseline, use the role-matched design document: consumer surfaces use `.github/standards/weapp/DESIGN_SYSTEM.md`; non-consumer surfaces use `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`
- Use `.github/standards/weapp/REVIEW_CHECKLIST.md` as the compact PR review checklist so the review covers both baseline conformance and user-facing coherence
- State what validation evidence exists, what was not verified, and what residual risk remains

Review must prioritize:

- Wrong capability grouping, such as one-interface-one-page mapping or oversized all-in-one pages with no single primary task
- Missing domain-component extraction when dense local workflows are left inline in a page shell
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
- Review mode: <baseline conformance | overall upgrade audit | payment/high-risk focus>
- Reference page or component: <path>
- Validation evidence already run: <commands or none>

Baseline review must check:

- Backend-supported capabilities were grouped into sensible user tasks before page and component boundaries were chosen, instead of being mirrored mechanically from interface count
- The changed surface still has a clear page boundary: one page when the task stays singular and first-screen-readable, a page group when task continuity, density, or local-state complexity requires separation
- Regions with independent local state, repeated edit flows, or dense composition were extracted into domain components when needed instead of being left as one oversized page block
- New fields and actions propagate through service layer, state, handlers, and visible UI
- Request parameters, response fields, enums, and types stay aligned with the real backend contract instead of drifting from page-local assumptions
- Backend semantics are treated as the only source of truth; if the code is guessing around missing or ambiguous backend meaning, call that out as a finding or residual risk
- App Shell structure remains stable during loading and error states
- Page shell, outer gutter, nav gap, safe-area handling, and content-container spacing follow the current role-side standards instead of drifting into local one-off layout rules
- TDesign component choice and customization stay within supported methods, and any non-TDesign or visual-system exception is justified explicitly against the weapp standards
- Non-consumer pages do not inherit consumer-side custom design language, branding colors, or decorative styling by default
- The review names the correct role-side design document when visual-system assertions depend on it rather than treating one design document as universal
- User-facing copy and affordances are clear in weak-network and empty-data scenarios
- Primary and secondary actions remain visually and behaviorally clear
- Returning to the page, retrying, or foreground re-entry does not break the user's task context

Overall upgrade audit add-on:

- Use this only when the request explicitly asks for upgrade audit, style unification, or system-level experience review
- Separate baseline violations from upgrade opportunities
- Check whether the change still preserves low-quality patterns such as fake success, over-fragmented cards, stacked explanations, unstable first-screen request fan-out, or cross-page inconsistency that keeps the flow from feeling like one coherent system
- If the code is baseline-compliant but still keeps a notable experience or coherence debt, classify that as an upgrade opportunity instead of a defect finding

Payment / high-risk review add-on:

- Use this whenever the task is explicitly payment-focused or the changed path should be treated as `G2`/`G3`
- Explicitly check login recovery, duplicate taps, stale polling, delayed confirmation, unknown-result handling, and cross-page state consistency between order, payment, result, and history surfaces
- If these paths changed but were not actually validated, call them out as residual risk instead of treating the review as fully closed

Output rules:

- When present, report capability-grouping, page-boundary, and component-boundary defects before lower-level implementation defects, because they usually invalidate the rest of the page decision.
- Separate proven code defects from interaction defects when both exist
- In overall upgrade audit mode, also separate baseline violations from upgrade opportunities so the review can distinguish “must fix” from “should redesign”
- If a high-risk flow was changed but not actually validated, call it out as residual risk even when no direct bug is proven
- If the frontend behavior depends on backend semantics that remain ambiguous or missing, call out whether backend clarification or backend changes are required
- If there are no findings, say so explicitly and mention residual risks or validation gaps