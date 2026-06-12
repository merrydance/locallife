---
name: "Web 实现请求模板"
description: "Use when drafting a web implementation or UI change request for web/. Trigger phrases: update route, web 页面, change component, add field to page, fix operator UI state, web UX adjustment. 适用于发起 Next.js Web 页面与组件实现任务。"
---
# Web Implementation Template

Use this template when asking for a concrete web change in `web/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the implementation scope before writing the request. Do not keep relying on stale context.

Use the web row in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` as the shared source for implementation push items, prohibited shortcuts, and review-ready hand-off expectations.
Use `.agents/skills/locallife-human-centered-ui` before non-trivial Web UI structure, workflow, or UX changes so role habits, task frequency, first-screen judgment, preserved context, and recovery paths shape the page before component or style choices.

## Web Page Or Component Change

Request:

- Update <route or component>
- Follow `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md` for user-task-first design, ViewState modeling, and API-flattening anti-patterns
- Include a compact Human-Centered UI Check from `.agents/skills/locallife-human-centered-ui`
- Follow `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`
- Reuse existing components from `web/src/components/ui/` before creating new primitives
- Keep page-level data logic out of presentational components when nearby code already separates them
- Run the smallest relevant validation command and report what was executed

Required context:

- Target route or component path: <path>
- User role and primary task: <operator, merchant, platform, or other + what they are trying to finish>
- Desired UX or behavior change: <details>

Optional context:

- Task frequency and habit path: <first-time, occasional, high-frequency + default/filter/state memory needs>
- Failure or recovery sensitivity: <timeout, retry, partial failure, re-entry, stale data>
- Existing reference page: <path>
- Backend field or status changes involved: <details>

Acceptance checklist:

- Page structure is driven by the user's task and ViewState, not by endpoint count, DTO field groups, or raw API response shape
- The most common path, first-screen information priority, preserved state, and recovery path are explicit
- Page still follows `PageHeader + PageContent`
- New fields and statuses are threaded through types, API calls, state, rendering, and copy
- Loading, empty, error, and validation states are complete where relevant
- No one-off colors, typography, or bespoke control patterns were introduced without need
- User-facing copy is business-readable and does not expose implementation language
