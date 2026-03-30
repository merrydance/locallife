---
name: "Web 审查请求模板"
description: "Use when drafting a web review request with findings-first output. Trigger phrases: review web change, inspect UI regression, check state propagation, audit operator page wiring, findings first web review. 适用于发起 Web 界面与状态传播审查。"
---
# Web Review Template

Use this template when asking for a web review in `web/`.

## Web Review

Request:

- Review this change with findings first, ordered by severity
- Check it against `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`
- Prioritize broken field propagation, inconsistent states, contract drift, and UI pattern regressions
- Flag missing loading, empty, error, disabled, and validation states
- Call out dangerous or high-impact user paths that changed but still appear unverified in real interaction flow
- If there are no findings, say so explicitly and mention residual risks

Required context:

- Changed route or component paths: <paths>

Optional context:

- Expected UX behavior: <details>
- Reference page or pattern: <path>
- Validation evidence already run: <commands or none>

Review checklist:

- Field and status changes are fully threaded through data fetch, state, render, and copy
- Shared UI primitives were reused instead of cloning divergent patterns
- Tabs, filters, tables, and action areas still follow established semantics
- Error copy and disabled states are understandable to operators or merchants