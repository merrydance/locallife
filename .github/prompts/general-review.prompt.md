---
name: "通用审查请求模板"
description: "Use only when drafting a review request that spans multiple product areas or the target area is not yet clear. Trigger phrases: cross-area review, backend plus web review, system-wide regression review, findings first across multiple surfaces. 适用于整理跨区域或尚未明确归属的通用审查请求。"
---
# General Review Template

Use this template when asking for a code review in this workspace.

## General Review

Request:

- Review this change with findings first, ordered by severity
- Prioritize bugs, behavioral regressions, contract violations, broken change propagation, and missing validation
- Check authentication, authorization, sensitive data exposure, unsafe defaults, and obvious injection or callback-handling risks when relevant
- Check whether the change forms a complete end-to-end path instead of stopping at one layer
- Call out missing tests, missing regeneration steps, and residual risk
- If a high-risk path changed but was not actually validated, say exactly which path remains unverified
- If there are no findings, say so explicitly

Optional context:

- Changed files or PR scope: <paths>
- Expected behavior: <details>
- Known risk areas: <details>
- Validation evidence already run: <commands or none>

## Backend Review

Target area: `locallife/`

Request:

- Review this backend change against `.github/standards/backend/API_CONTRACT_STANDARDS.md`
- Check handler, logic, store, route, DTO, Swagger, and tests for end-to-end completeness
- Flag logic that appears unused, unreachable, or computed without affecting behavior
- Flag SQL added under `locallife/db/query/` when it is not wired through generated code, logic, handlers, workers, or tests
- Check whether `make sqlc`, `make mock`, or `make swagger` should have been run
- Call out debug leftovers such as temporary prints, panic probes, hardcoded values, or short-circuit returns
- Check authn/authz boundaries, secret or PII exposure, callback verification, upload/download access control, and unsafe logging

Optional context:

- Changed endpoint or package: <path>
- Contract change: <details>
- Related docs: `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/backend/API_CONTRACT_STANDARDS.md`

## Web Review

Target area: `web/`

Request:

- Review this UI change against `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`
- Check whether new fields and statuses are fully threaded through state, API calls, rendering states, and user-facing copy
- Flag places where the UI diverges from existing patterns without a clear reason
- Call out missing empty states, loading states, validation states, and tests where relevant
- Flag sensitive field exposure, client-only permission gating, unsafe rendering of user content, and dangerous actions without proper confirmation

Optional context:

- Route or component path: <path>
- Expected UX behavior: <details>
- Existing reference page: <path>

## Mini Program Review

Target area: `weapp/`

Request:

- Review this Mini Program change against `.github/standards/weapp/DESIGN_SYSTEM.md`
- Check whether new actions and fields are fully wired through page state, service calls, event handlers, and user-visible states
- Flag business styles that leak into shared global styles
- Call out debug leftovers, placeholder branches, and missing validation or quality checks
- Flag exposed private materials or sensitive fields, client-only permission assumptions, and dangerous actions without clear confirmation or failure handling

Optional context:

- Page or component path: <path>
- Expected behavior: <details>
- Existing reference page: <path>