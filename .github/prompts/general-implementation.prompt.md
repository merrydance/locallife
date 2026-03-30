---
name: "通用实现请求模板"
description: "Use only when drafting an implementation request that spans multiple product areas or the target area is not yet clear. Trigger phrases: cross-area implementation, one request touches backend and web, coordinate backend and Mini Program, multi-surface feature change. 适用于整理跨区域或尚未明确归属的实现型请求。"
---
# General Implementation Template

Use this template when asking for a concrete code change in this workspace.

## Backend Feature Or Fix

Target area: `locallife/`

Request:

- Implement <feature or fix>
- Follow the existing handler/logic/db layering
- Reuse nearby patterns before introducing a new abstraction
- Tell me whether the change requires `make sqlc`, `make mock`, or `make swagger`
- Run the smallest relevant validation command and report what was executed

Optional context:

- Affected endpoint or package: <path>
- Contract or behavior change: <details>
- Related docs: `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/backend/API_CONTRACT_STANDARDS.md`
- Acceptance checklist: handler/logic/store wiring, status constant reuse, regeneration steps, smallest relevant test command

## Web Page Change

Target area: `web/`

Request:

- Update <page or component>
- Follow `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`
- Reuse existing components from `web/src/components/ui/` where possible
- Keep data logic out of presentational components when existing patterns already do that
- Run the smallest relevant validation command and report what was executed

Optional context:

- Route or component path: <path>
- Desired UX change: <details>
- Existing reference page: <path>
- Acceptance checklist: field threading, loading/empty/error states, copy review, smallest relevant validation command

## Mini Program Change

Target area: `weapp/`

Request:

- Update <page or component>
- Reuse existing TDesign-based patterns and local components first
- Keep business-specific styles out of global styles unless they are truly shared
- Run the smallest relevant validation command and report what was executed

Optional context:

- Page or component path: <path>
- Desired behavior: <details>
- Existing reference page: <path>
- Acceptance checklist: page state completeness, service/event wiring, token usage, smallest relevant validation command