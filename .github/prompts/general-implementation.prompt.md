---
name: "通用实现请求模板"
description: "Use only when drafting an implementation request that spans multiple product areas or the target area is not yet clear. Trigger phrases: cross-area implementation, one request touches backend and web, coordinate backend and Mini Program, multi-surface feature change. 适用于整理跨区域或尚未明确归属的实现型请求。"
routing-hints: "跨区域实现|cross-area implementation|backend and web|同时改 backend 和 web|coordinate backend and mini program|multi-surface feature change"
---
# General Implementation Template

Use this template when asking for a concrete code change in this workspace.

Use `general-implementation.prompt.md` only when the task spans multiple product areas or the target area is still ambiguous. Once the target is clearly `locallife/`, `web/`, or `weapp/`, prefer the matching area-specific implementation prompt and let `.github/instructions/` carry the detailed execution rules.

If the task is cross-area, high-risk, or touches security, money movement, status semantics, async recovery, or sensitive data, also ask the implementer to classify the work as `G0`, `G1`, `G2`, or `G3` using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` and to state the validation depth that matches that risk using `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

When the task spans multiple stacks, use the shared implementation and review matrix in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` to keep push items, prohibited shortcuts, and review expectations aligned across backend, web, and Mini Program work.

## General Change Request

Request:

- Implement <feature or fix>
- Keep the affected execution path complete across every touched layer or surface
- Reuse nearby patterns before introducing a new abstraction
- Tell me which target areas are involved: <backend, web, weapp, mixed>
- Classify the risk level (`G0`/`G1`/`G2`/`G3`) and explain why when the task is not obviously routine
- Tell me whether any regeneration or derived-artifact steps are required
- Run the smallest relevant validation command and report what was executed
- State which relevant paths were not verified and what residual risk remains

Required context:

- Target area or affected paths: <paths>
- Expected behavior after the change: <details>

Optional context:

- Related contract, workflow, or runbook: <paths>
- Existing reference implementation or page: <paths>
- Validation budget: <focused tests, lint + build, compile only, etc.>

Area-specific reminders:

- Backend-heavy work: name the handler/logic/store or worker path that should close the loop, and call out whether `make sqlc`, `make mock`, or `make swagger` may be needed.
- Web-heavy work: name the page or component path, expected loading or error behavior, and any sensitive fields or dangerous actions involved.
- Mini Program-heavy work: name the page or component path, expected weak-network or re-entry behavior, and any state-recovery expectations.