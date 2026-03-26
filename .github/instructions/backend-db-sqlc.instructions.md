---
applyTo: "locallife/db/sqlc/**"
---

# Backend DB SQLC Instructions

Apply these rules for files under `locallife/db/sqlc/`.

## Role Of This Layer

- Keep this layer responsible for typed persistence access, generated query surfaces, transaction helpers, and shared persistence constants.
- Preserve the separation between SQL source files in `locallife/db/query/`, generated code in `locallife/db/sqlc/*.sql.go`, and handwritten transaction or store glue such as `tx_*.go`, `store.go`, and `constants.go`.

## Persistence Conventions

- Prefer changing SQL source files under `locallife/db/query/` rather than hand-editing generated `*.sql.go` files.
- Keep transaction orchestration inside existing `execTx` patterns and `tx_*.go` files.
- Use `locallife/db/sqlc/constants.go` as the single source of truth for persistence-facing business constants.
- Preserve existing `Store` and `Querier` composition patterns when adding new operations.

## Boundary Checks

- New queries should have an expected caller in logic, workers, schedulers, or tests.
- Schema or query changes should be reflected in generated code, callers, and tests instead of stopping at the SQL layer.
- Avoid introducing transport concepts or response shaping into persistence code.

## Regeneration And Validation

- If SQL or query signatures change, run `make sqlc`.
- If store interfaces used by mocks change, run `make mock` or `make sqlc` as appropriate.
- Prefer focused persistence tests where available, then run `make test-unit` for broader validation.