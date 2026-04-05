---
name: "小程序审查请求模板"
description: "Use when drafting a Mini Program review request with findings-first output. Trigger phrases: review Mini Program change, inspect setData misuse, check page state propagation, audit weak-network UX, findings first weapp review. 适用于发起微信小程序代码审查。"
---
# Mini Program Review Template

Use this template when asking for a Mini Program review in `weapp/`.

## Mini Program Review

Request:

- Review this change with findings first, ordered by severity
- Check it against `.github/standards/weapp/DESIGN_SYSTEM.md`, `.github/standards/weapp/INTERACTION_STANDARDS.md`, and `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- Use `.github/standards/weapp/REVIEW_CHECKLIST.md` as the compact PR review checklist so the review covers both baseline conformance and user-facing coherence
- If the request is about overall upgrade, consistency, or raising UX quality across a flow, also use `weapp/docs/WEAPP_DOCUMENTATION_ARCHITECTURE_2026-04-04.md`, `weapp/docs/WEAPP_DOCUMENT_REWRITE_BLUEPRINTS_2026-04-04.md`, and `weapp/docs/WEAPP_95_SCORE_IMPROVEMENT_PLAN_2026-03-27.md` as historical audit inputs rather than active rule authorities
- Prioritize broken service-to-state-to-view wiring, missing page states, token violations, approved design-system drift, interaction regressions, and debug leftovers
- Flag business styles leaking into shared global styles or shared components
- Call out unverified high-risk flows explicitly when payment, weak-network retry, realtime updates, login recovery, or duplicate-tap protection are involved
- If there are no findings, say so explicitly and mention residual risks

Required context:

- Changed page or component paths: <paths>

Optional context:

- Expected behavior: <details>
- User role and task goal: <details>
- High-frequency or weak-network sensitivity: <details>
- Review mode: <baseline conformance | overall upgrade audit>
- Reference page or component: <path>
- Validation evidence already run: <commands or none>

Review dimensions:

- New fields and actions propagate through service layer, state, handlers, and visible UI
- Request parameters, response fields, enums, and types stay aligned with the real backend contract instead of drifting from page-local assumptions
- App Shell structure remains stable during loading and error states
- TDesign or existing shared components were used where appropriate
- Popup forms use a stable bottom action area instead of leaving action buttons inside scroll content tails
- Bottom popup dual actions render as equal-width block buttons and do not degrade into content-width small buttons
- Buttons and tags do not fall back to forbidden outline-style defaults unless an explicit exception is documented
- TDesign internals are not overridden for page-local visual preference when tokens, theme props, and shared layout patterns would suffice
- Sibling pages in the same task scope still read as one coherent system rather than a mix of competing local patterns
- User-facing copy and affordances are clear in weak-network and empty-data scenarios
- Primary and secondary actions remain visually and behaviorally clear
- Returning to the page, retrying, or foreground re-entry does not break the user's task context
- In overall upgrade audit mode, check whether the change still preserves historical low-quality patterns called out by the blueprint docs, such as fake success, over-fragmented cards, stacked explanations, unstable first-screen request fan-out, or cross-page inconsistency that keeps the flow from feeling like one coherent system

Output rules:

- Separate proven code defects from interaction defects when both exist
- In overall upgrade audit mode, also separate baseline violations from upgrade opportunities so the review can distinguish “must fix” from “should redesign”
- If a high-risk flow was changed but not actually validated, call it out as residual risk even when no direct bug is proven