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

## Area-Specific Review Reminders

- Backend: verify API contract semantics against `.github/standards/backend/API_CONTRACT_STANDARDS.md`, especially status codes, empty-state behavior, and route consistency.
- Backend: check that business logic stays out of handlers and that status constants still come from `locallife/db/sqlc/constants.go`.
- Backend: check that source changes in `locallife/db/query/`, interfaces, or Swagger annotations were followed by the required regeneration steps.
- Web: check that new UI work still follows `.github/standards/web/WEB_UI_STANDARDS.md` and `.github/standards/web/DESIGN_GUARDRAILS.md`.
- Web: check that new data or status fields are fully threaded through page state, API calls, rendering states, and user-visible copy.
- Mini Program: check that new patterns align with `.github/standards/weapp/DESIGN_SYSTEM.md` and do not leak business styles into shared global styles.
- Mini Program: check that new fields or actions are wired through page state, service calls, event handlers, and user-facing states.