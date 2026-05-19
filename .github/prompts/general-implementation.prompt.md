---
name: "通用实现请求模板"
description: "Use only when drafting an implementation request that spans multiple product areas or the target area is not yet clear. Trigger phrases: cross-area implementation, 跨区域实现, one request touches backend and web, 同时改 backend 和 web, coordinate backend and Mini Program, multi-surface feature change. 适用于整理跨区域或尚未明确归属的实现型请求。"
---
# General Implementation Template

Use this template when asking for a concrete code change in this workspace.

Use `general-implementation.prompt.md` only when the task spans multiple product areas or the target area is still ambiguous. Once the target is clearly `locallife/`, `web/`, or `weapp/`, prefer the matching area-specific implementation prompt and let `.github/instructions/` carry the detailed execution rules.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the target area before writing the request. Do not keep relying on stale context.

If the task is cross-area, high-risk, or touches security, money movement, status semantics, async recovery, or sensitive data, also ask the implementer to classify the work as `G0`, `G1`, `G2`, or `G3` using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` and to state the validation depth that matches that risk using `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

When the task spans multiple stacks, use the shared implementation and review matrix in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` to keep push items, prohibited shortcuts, and review expectations aligned across backend, web, and Mini Program work.

The protocol goal of this template is to reduce silent assumption drift, overbuilt implementations, and unverifiable completion claims. Keep the request focused on the smallest complete behavior change.

## General Change Request

Request:

- Implement <feature or fix>
- Before implementation, split the work into the smallest set of clearly bounded tasks that each fit in one context window and still make sense if resumed later
- If the request is broad, mixed, or contains multiple independent outcomes, break it into sub-requests first instead of writing one fuzzy mega-task
- For each task, state the exact goal, files, acceptance criteria, and verification command so the next engineer does not need to guess the boundary
- Start by stating any assumption or ambiguity that could materially change behavior, scope, or validation; if multiple interpretations remain plausible, ask or say which one you are taking and why
- Keep the affected execution path complete across every touched layer or surface
- Start by stating which area or module owns the capability and whether the change affects any single-writer state transition
- Prefer the simplest implementation that solves the request and reuse nearby patterns before introducing a new abstraction
- Do not add speculative abstractions, extra configurability, or future-proofing that the request did not ask for
- Keep the diff surgical; every changed line should trace back to the request, and unrelated cleanup should be called out rather than folded into the same change
- Prefer existing single sources of truth for states, enums, permissions, and error semantics instead of introducing local duplicates
- Tell me which target areas are involved: <backend, web, weapp, mixed>
- Classify the risk level (`G0`/`G1`/`G2`/`G3`) and explain why when the task is not obviously routine
- For bugfixes, refactors, or multi-step requests, provide a short numbered plan where each step names its verification check
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

- Backend-heavy work: name the handler/logic/store or worker path that should close the loop, call out whether `make sqlc`, `make mock`, or `make swagger` may be needed, and separate durable-state changes from external side effects.
- Web-heavy work: name the page or component path, expected loading or error behavior, and any sensitive fields or dangerous actions involved.
- Mini Program-heavy work: name the page or component path, expected weak-network or re-entry behavior, and any state-recovery expectations.

Security baseline reminders:

- Call out replay, duplicate delivery, permission boundary, signature, injection, and sensitive-data risks when they are plausibly relevant to the task.
- Prefer explicit fail-closed behavior over silent fallback when the request touches trust boundaries or provider data.
- If a known security pattern could apply, mention it in the request and say which file, branch, or boundary will enforce it.
