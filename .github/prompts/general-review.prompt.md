---
name: "通用审查请求模板"
description: "Use only when drafting a review request that spans multiple product areas or the target area is not yet clear. Trigger phrases: cross-area review, 跨区域审查, backend plus web review, system-wide regression review, 跨多个界面审查, findings first across multiple surfaces. 适用于整理跨区域或尚未明确归属的通用审查请求。"
---
# General Review Template

Use this template when asking for a code review in this workspace.

Use `general-review.prompt.md` only when the review spans multiple product areas or the target area is still ambiguous. Once the scope is clearly backend-only, web-only, or Mini Program-only, prefer the matching area-specific review prompt and let `.github/instructions/review.instructions.md` plus the area instructions carry the detailed review rules.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the review scope before writing findings. Do not keep relying on stale context.

If the change is cross-area, high-risk, or touches security, status semantics, async recovery, sensitive data, or dangerous user actions, ask the reviewer to infer the risk level (`G0`/`G1`/`G2`/`G3`) using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` and scale the review depth accordingly, with validation expectations grounded in `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

When the review spans multiple stacks, use the shared implementation and review matrix in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` so implementation push items, prohibited shortcuts, and findings-first review checks stay on the same source of truth.

If the review uses or updates `artifacts/codegraph/`, follow
`artifacts/codegraph/README.md`: use CodeGraph only as a discovery and drift
checking aid, then treat the reviewed `*.slice.md` and `*.edges.json` artifacts
as the durable source for LocalLife business semantics.

The protocol goal of this template is to surface concrete defects and missing validation early, especially when failures can disappear through silent fallbacks, vague error semantics, or incomplete propagation.

## General Review

Request:

- Review this change with findings first, ordered by severity
- Infer the likely risk level (`G0`/`G1`/`G2`/`G3`) and call out if the implementation or validation evidence treated a clearly higher-risk path as routine
- Prioritize bugs, behavioral regressions, contract violations, broken change propagation, and missing validation
- Flag silent error swallowing, nil-or-empty values treated as implicit success, missing logging boundaries for unexpected failures, or caller-facing errors that are vague, unstable, or leak internal details
- Check whether the change forms a complete end-to-end path instead of stopping at one layer
- For user-reported defects, check the User-Reported Defect Closure Gate in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md`: original failing scenario, any known working reference path, regression evidence, and why the old behavior or previous fix missed the real root cause
- Check whether the capability owner and single source of truth remain clear, or whether the change introduced duplicate state semantics or multiple writers
- Check whether replay, duplicate delivery, authorization, signature, injection, or sensitive-data boundaries are missing, bypassed, or only implied by comments
- Check whether known security patterns were handled by an explicit guard, validation, or fail-closed branch instead of a silent fallback
- For codegraph artifact work, check whether CodeGraph findings were verified against current code, SQL, route groups, domain standards, and then written back to the artifact instead of left as tool output
- Call out missing tests, missing regeneration steps, and residual risk
- For `G2` and `G3` paths, call out missing failure-mode coverage, duplicate-trigger handling, rollback or recovery story, and user-visible degradation handling
- If a high-risk path changed but was not actually validated, say exactly which path remains unverified
- If there are no findings, say so explicitly

Optional context:

- Changed files or PR scope: <paths>
- Expected behavior: <details>
- Risk level if already known: <G0/G1/G2/G3 and why>
- Known risk areas: <details>
- Validation evidence already run: <commands or none>
- Release or rollback notes: <details or none>

Area-specific reminders:

- Backend-heavy review: name the handler/logic/store or worker path, any DTO or contract change, whether unexpected errors reach one structured logging boundary, whether public error semantics stay clear and stable, and whether regeneration steps may be relevant.
- Web-heavy review: name the route or component path, expected loading or error behavior, and any sensitive fields or dangerous actions involved.
- Mini Program-heavy review: name the page or component path, expected weak-network or re-entry behavior, and any state-recovery expectations.

Security review reminders:

- Prefer findings that identify the exact security boundary and the exact bypass path, not just a generic security smell.
- If a known attack class is relevant but not covered by the diff, call that out as a gap only if this change should have enforced it.
- Do not invent new security policy in review if the relevant rule already belongs in standards; point to the missing standard or missing hook instead.
