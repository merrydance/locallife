---
applyTo: "locallife/db/query/**"
---

# Backend DB Query Instructions

Apply these rules for files under `locallife/db/query/`.

Use `.github/standards/backend/SQL_STANDARDS.md` as the canonical SQL authoring and migration standard for this layer.

## Role Of This Layer

- Keep this directory as the source of truth for SQL query definitions consumed by sqlc.
- Treat changes here as source changes that must propagate into generated code, callers, and tests.

## SQL Authoring Conventions

- Prefer extending existing domain SQL files instead of scattering related queries into a new file without a clear boundary.
- Preserve existing sqlc naming conventions such as `-- name: Xxx :one`, `:many`, or `:exec`.
- Keep query names stable and domain-specific so generated methods remain predictable for callers.
- Follow existing patterns for optional arguments, pagination, and update statements instead of inventing a new style in one file.

## Boundary Checks

- A new query should have an expected caller in `locallife/db/sqlc/`, `locallife/logic/`, `locallife/worker/`, `locallife/scheduler/`, or tests.
- Schema or field changes in SQL should be reflected in generated code and propagated to business callers rather than stopping at the SQL layer.
- Do not encode transport-layer response shaping or handler-specific assumptions into query definitions.

## Regeneration And Validation

- Run `make sqlc` after changing SQL in this directory.
- If generated interfaces used by mocks change, run `make mock` or `make sqlc` as appropriate.
- Prefer focused tests around the affected persistence or business callers, then run `make test-unit` for broader validation.