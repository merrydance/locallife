---
name: "通用审查请求模板"
description: "Use only when drafting a review request that spans multiple product areas or the target area is not yet clear. Trigger phrases: cross-area review, 跨区域审查, backend plus web review, system-wide regression review, 跨多个界面审查, findings first across multiple surfaces. 适用于整理跨区域或尚未明确归属的通用审查请求。"
---
# General Review Template

Use this template when asking for a code review in this workspace.

Use `general-review.prompt.md` only when the review spans multiple product areas or the target area is still ambiguous. Once the scope is clearly backend-only, web-only, or Mini Program-only, prefer the matching area-specific review prompt and let `.github/instructions/review.instructions.md` plus the area instructions carry the detailed review rules.

If the change is cross-area, high-risk, or touches security, status semantics, async recovery, sensitive data, or dangerous user actions, ask the reviewer to infer the risk level (`G0`/`G1`/`G2`/`G3`) using `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md` and scale the review depth accordingly, with validation expectations grounded in `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`.

When the review spans multiple stacks, use the shared implementation and review matrix in `.github/standards/engineering/AI_PROMPT_GOVERNANCE.md` so implementation push items, prohibited shortcuts, and findings-first review checks stay on the same source of truth.

## General Review

Request:

- Review this change with findings first, ordered by severity
- Infer the likely risk level (`G0`/`G1`/`G2`/`G3`) and call out if the implementation or validation evidence treated a clearly higher-risk path as routine
- Prioritize bugs, behavioral regressions, contract violations, broken change propagation, and missing validation
- Check whether the change forms a complete end-to-end path instead of stopping at one layer
- Check whether the capability owner and single source of truth remain clear, or whether the change introduced duplicate state semantics or multiple writers
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

- Backend-heavy review: name the handler/logic/store or worker path, any DTO or contract change, and whether regeneration steps may be relevant.
- Web-heavy review: name the route or component path, expected loading or error behavior, and any sensitive fields or dangerous actions involved.
- Mini Program-heavy review: name the page or component path, expected weak-network or re-entry behavior, and any state-recovery expectations.