# Web Implementation Template

Use this template when asking for a concrete web change in `web/`.

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