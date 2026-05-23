---
name: "后端接手模板"
description: "Use when drafting a backend takeover or onboarding request for locallife/. Trigger phrases: backend takeover, 接手后端项目, 接手这个后端项目, build backend working context, 工作上下文, map backend production paths, 关键生产链路, new backend owner handoff, understand backend system before changes. 适用于新接手后端项目时快速建立可开发工作上下文。"
---
# Backend Takeover Template

Use this template when a new engineer or agent is taking over backend work in `locallife/` and needs a durable, development-ready understanding of the system before making changes.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the takeover scope before writing the request. Do not keep relying on stale context.

Use `.github/standards/backend/RUNTIME_ARCHITECTURE.md` to establish the real composition roots and async boundaries, `.github/standards/backend/WORKFLOW_AND_VALIDATION.md` to establish generation and test workflow, and `.github/standards/backend/README.md` plus the matching domain README to identify the highest-risk production chains.

## Backend Takeover

Request:

- Build a backend working context for `locallife/` before implementation starts
- Read the actual startup, routing, business, persistence, and async boundaries instead of inferring a simplified architecture
- Identify the highest-risk production chains, the current generated-code workflow, and the minimum validation commands a new owner must know
- Call out the unknowns that should be verified before editing high-risk code
- Reference concrete files and packages instead of giving a generic summary

Required context:

- Target subsystem or broad scope: <whole backend or specific domain>
- Immediate goal after takeover: <feature work, bugfix, audit, on-call support, etc.>

Optional context:

- Related docs or runbooks: <paths>
- Recent risky area: <payment, refund, delivery, reservation, media, OCR, authz, callback, scheduler>

Required output:

1. System structure and composition roots
2. Main production paths that must be traced end to end
3. Highest-risk invariants and existing defenses
4. Generated artifact and test workflow
5. Unknowns or ambiguous areas that should be confirmed before making changes

Acceptance checklist:

- The output is specific enough that a new owner can start safe work without guessing the architecture
- It names which flows must be read across `api`, `logic`, `db/sqlc`, `worker`, `scheduler`, and webhook or callback boundaries
- It highlights concrete high-risk domains instead of giving a flat package tour
- It identifies at least the minimum validation and regeneration workflow a new owner must preserve

