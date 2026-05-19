---
applyTo: "locallife/db/query/**"
---

# Backend DB Query Instructions

Apply these rules for files under `locallife/db/query/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

Use `.github/standards/backend/SQL_STANDARDS.md` as the canonical SQL authoring and migration standard for this layer.

## Role Of This Layer

- Keep this directory as the source of truth for SQL query definitions consumed by sqlc.
- Treat changes here as source changes that must propagate into generated code, callers, and tests.

## SQL Authoring Conventions

- Prefer extending existing domain SQL files instead of scattering related queries into a new file without a clear boundary.
- Preserve existing sqlc naming conventions such as `-- name: Xxx :one`, `:many`, or `:exec`.
- Keep query names stable and domain-specific so generated methods remain predictable for callers.
- Follow existing patterns for optional arguments, pagination, and update statements instead of inventing a new style in one file.
- Do not use `SELECT *`; list the real columns needed by callers.
- Do not add `LIMIT` / `OFFSET` to `:many` queries without a stable `ORDER BY`.
- Do not write `UPDATE` / `DELETE` without an explicit `WHERE` scope.
- Prefer explicit column lists in `INSERT` statements instead of positional `VALUES` against the whole table shape.
- Treat implicit `INSERT INTO table VALUES (...)` style writes as guardrail-level bad SQL unless a query block documents a specific exception.
- If a query block uses `sqlguard:` exception comments, require an inline reason that states why the default rule is not applicable and why the exception is still safe in this narrow block.
- For order, payment, refund, delivery, reservation, and inventory writes, prefer conditional updates that encode the required current state instead of updating by `id` alone.
- Do not rely on transaction-external `SELECT` checks to enforce exclusivity such as single active reservation order, single successful delivery claim, or no oversell inventory semantics.
- Treat merchant order lists, rider pool lists, reservation lists, and recovery scans as hot paths; avoid assuming high-offset pagination or broad `COUNT(*)` queries are free.

## Boundary Checks

- A new query should have an expected caller in `locallife/db/sqlc/`, `locallife/logic/`, `locallife/worker/`, `locallife/scheduler/`, or tests.
- Schema or field changes in SQL should be reflected in generated code and propagated to business callers rather than stopping at the SQL layer.
- Do not encode transport-layer response shaping or handler-specific assumptions into query definitions.

## Regeneration And Validation

- Run `make sqlc` after changing SQL in this directory.
- If generated interfaces used by mocks change, run `make mock` or `make sqlc` as appropriate.
- Prefer focused tests around the affected persistence or business callers, then run `make test-unit` for broader validation.