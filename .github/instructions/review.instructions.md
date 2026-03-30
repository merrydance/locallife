---
applyTo: "**"
---

# Review Instructions

Apply these rules when the user asks for a review.

## Primary Objective

- Prioritize bugs, behavioral regressions, contract violations, broken change propagation, and missing validation.
- Treat findings as more important than summaries.
- Focus on the changed code, the nearby code paths it can affect, and whether the change forms a complete end-to-end path.

## What To Check First

- API or data contract changes that are not reflected in callers, tests, or docs.
- Missing or weak validation, especially around status transitions, permissions, and nil or empty inputs.
- Regressions caused by moving logic across handler, service, persistence, or UI boundaries.
- Missing regeneration steps such as `make sqlc`, `make mock`, or `make swagger` after source changes.
- Missing tests for new branches, edge cases, or failure paths.

## Security Checks

- Check authentication and authorization boundaries, especially object-level access control and role scoping.
- Flag handlers or services that rely only on client-provided identity, role, merchant_id, owner_id, or status fields without server-side verification.
- Check whether new fields, endpoints, or actions expose secrets, tokens, internal IDs, raw provider payloads, or personally identifiable information to logs, responses, or UI.
- Flag missing validation or sanitization on user-controlled inputs that could affect SQL, HTML rendering, file paths, object keys, callback handling, or downstream requests.
- Check upload, download, media, OCR, payment, and webhook flows for missing ownership checks, signature checks, content-type checks, or replay protections.
- Flag hardcoded credentials, test keys, debug bypasses, insecure defaults, or new configuration that would be unsafe in production.

## Unverified High-Risk Paths

- If the change touches callbacks, async jobs, retries, payment, refunds, OCR, uploads, downloads, authorization-sensitive logic, or other externally triggered paths that were not actually validated, call that out explicitly.
- Do not treat "not enough evidence" as neutral. If a high-risk path was changed but not verified, report it as residual risk or a finding depending on the severity and likelihood of regression.
- Prefer concrete statements such as "callback idempotency was not exercised" or "worker retry classification remains unverified" over generic phrases like "needs more testing".
- If you cannot determine whether a high-risk path is safe because the diff lacks surrounding validation or evidence, say that directly.

## Structural Completeness Checks

- Check whether the change forms a complete path instead of stopping at one layer.
- Flag SQL, store, logic, handler, route, DTO, or UI changes that were added in one layer but not connected through the remaining layers.
- Flag newly added queries, methods, or services that appear unused, unreachable, or not wired into any execution path.
- Flag logic whose outputs are computed but never persisted, returned, emitted, or used to affect behavior.
- Flag code paths that appear dead because a new branch, condition, or return path prevents the logic from ever executing.

## Orphan And Drift Checks

- Flag SQL added under `locallife/db/query/` when there is no corresponding generated usage, logic caller, handler entrypoint, worker entrypoint, or test coverage.
- Flag new logic or service methods that are not called by any handler, worker, scheduler, or other production path.
- Flag API handlers or request fields that do not propagate into logic, persistence, response mapping, or tests.
- Flag schema, DTO, or status changes that only partially propagated across request parsing, business logic, persistence, response shaping, and documentation.

## Debug And Temporary Code Checks

- Flag debug leftovers such as temporary prints, panic-based probing, commented-out production code, hardcoded test values, short-circuit returns, or placeholder branches left in active paths.
- Flag temporary guards or TODO-style stubs when they materially change runtime behavior or hide incomplete implementation.
- Flag debugging artifacts even when they do not break compilation, if they create misleading behavior, noisy logs, or production risk.

## Review Output Rules

- List findings first, ordered by severity.
- Explain the runtime or maintenance impact of each finding, not just the local code smell.
- Reference concrete files and lines where possible.
- Keep summaries brief and secondary.
- If no findings are discovered, state that explicitly and mention any residual risk or untested area.
- If no code-level bug is proven but a changed high-risk path remains unverified, say so explicitly instead of presenting the review as fully closed.

## Area-Specific Review Reminders

- Backend: verify API contract semantics against `.github/standards/backend/API_CONTRACT_STANDARDS.md`, especially status codes, empty-state behavior, and route consistency.
- Backend: check that business logic stays out of handlers and that status constants still come from `locallife/db/sqlc/constants.go`.
- Backend: check that source changes in `locallife/db/query/`, interfaces, or Swagger annotations were followed by the required regeneration steps.
- Backend: review authn/authz, secret handling, callback verification, upload/download access control, and whether sensitive data is over-logged or over-returned.
- Backend: when callbacks, workers, schedulers, or retries are involved, check idempotency, repeated delivery semantics, and failure recovery expectations even if the diff only shows one layer.
- Web: check that new UI work still follows `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`.
- Web: check that new data or status fields are fully threaded through page state, API calls, rendering states, and user-visible copy.
- Web: flag client-only permission checks, sensitive data exposure in page state or rendered fields, unsafe rendering of user content, and dangerous actions without proper confirmation or disabled states.
- Web: if dangerous actions, payout states, private materials, or moderation-sensitive fields changed but no user-visible confirmation or failure-state evidence is shown, call out the gap.
- Mini Program: check that new patterns align with `.github/standards/weapp/DESIGN_SYSTEM.md` and do not leak business styles into shared global styles.
- Mini Program: check that new fields or actions are wired through page state, service calls, event handlers, and user-facing states.
- Mini Program: flag client-only permission assumptions, exposed private materials or internal fields, unsafe weak-network fallbacks, and dangerous operations without clear confirmation or failure handling.
- Mini Program: when payment, login recovery, realtime state, or weak-network flows are touched, call out any unverified cold-start, retry, duplicate-tap, or re-entry behavior explicitly.