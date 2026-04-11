---
name: "Web 实现请求模板"
description: "Use when drafting a web implementation or UI change request for web/. Trigger phrases: update route, web 页面, change component, add field to page, fix operator UI state, web UX adjustment. 适用于发起 Next.js Web 页面与组件实现任务。"
---
# Web Implementation Template

Use this template when asking for a concrete web change in `web/`.

Use the web row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and review-ready hand-off expectations.

## Web Page Or Component Change

Request:

- Update <route or component>
- Follow `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`
- Reuse existing components from `web/src/components/ui/` before creating new primitives
- Keep page-level data logic out of presentational components when nearby code already separates them
- Run the smallest relevant validation command and report what was executed

Required context:

- Target route or component path: <path>
- Desired UX or behavior change: <details>

Optional context:

- Existing reference page: <path>
- Backend field or status changes involved: <details>

Acceptance checklist:

- Page still follows `PageHeader + PageContent`
- New fields and statuses are threaded through types, API calls, state, rendering, and copy
- Loading, empty, error, and validation states are complete where relevant
- No one-off colors, typography, or bespoke control patterns were introduced without need
- User-facing copy is business-readable and does not expose implementation language