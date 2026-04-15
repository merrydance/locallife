# LocalLife Backend Agent Guide

This file applies to `/home/sam/locallife/locallife`.

## Scope

- Backend stack: Go monolith with `gin`, `pgx/sqlc`, `redis`, `asynq`, cron schedulers, WeChat Pay/Ecommerce, OCR, media storage, WebSocket push.
- Frontend code lives outside this scope. Do not assume `weapp/` conventions apply here.

## First Reads

For any non-trivial backend task, read these first:

1. `../.github/standards/backend/README.md`
2. `../.github/standards/backend/RUNTIME_ARCHITECTURE.md`
3. `../.github/standards/backend/WORKFLOW_AND_VALIDATION.md`
4. `../.github/standards/backend/BACKEND_RISK_MAP.md`

Use the prompt templates in `../.github/prompts/` when the task matches. Legacy `.codex/prompts/` files should be treated as backend-local wrappers, not the long-term source of truth.

## Working Rules

- Trace the full production path before editing: `api` -> `logic` -> `db/sqlc` -> `worker`/`scheduler`/`webhook`.
- Treat payment, refund, profit sharing, delivery, reservation, merchant withdraw, and complaint flows as high-risk.
- Prefer fixing the invariant at the lowest defensible layer. If a business invariant is enforced only in API or logic, verify whether it must move into a transaction or database constraint.
- Do not hand-edit generated files in `db/sqlc/*.sql.go` or `docs/swagger.*` unless the task explicitly requires generated outputs and you also regenerate them.
- When changing SQL, transaction signatures, or store interfaces, regenerate artifacts with `make sqlc`.
- When changing Swagger annotations or public API contracts, regenerate docs with `make swagger`.
- Before closing SQL or API contract work, run `make check-generated` to confirm generated code and Swagger outputs stay in sync.
- Preserve existing dependency-injection patterns. New logic should depend on interfaces such as `db.Store`, `worker.TaskDistributor`, and WeChat client interfaces.
- Keep handler and logic files reasonably bounded. The repo has a `make lint-filesize` check with a default 500-line threshold.

## Verification Expectations

- Prefer targeted tests first, then broader package tests if the change touches shared flows.
- For DB-backed tests, understand that `db/sqlc/main_test.go` auto-runs migrations against `TEST_DB_SOURCE` or `DB_SOURCE`.
- If a change touches funds or state machines, verify at the transaction layer and at least one upstream caller layer.
- Use `make test-safety` after changes in high-risk paths such as order creation, payment, refund, and delivery transitions.
- Formal reviews and audits should follow `../.github/standards/backend/FORMAL_REVIEW_DURABILITY.md`; the current backend ledgers are `../.github/review/open-findings.md` and `../.github/review/audit-log.md`, while durable recurring patterns should update `../.github/standards/backend/BACKEND_RISK_MAP.md`.
- Call out when you did not run `make sqlc`, `make swagger`, or package tests.

## Existing Project Signals

- `main.go` contains startup wiring, production safeguards, scheduler registration, and worker boot order.
- `../.github/standards/backend/BACKEND_RISK_MAP.md` is the durable risk register for production backend paths.
- `../.github/standards/backend/RUNTIME_ARCHITECTURE.md` plus `../.github/standards/domains/wechat-payment/README.md` are the preferred entrypoints for current execution paths and payment-domain capability routing.
- There is existing user work in the tree; never revert unrelated changes.
