---
applyTo: "locallife/**"
---

# Backend Instructions

Apply these rules for files under `locallife/`.

More specific backend instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `locallife/api/`, `locallife/logic/`, `locallife/db/query/`, `locallife/db/sqlc/`, `locallife/worker/`, `locallife/scheduler/`, `locallife/integration/`, `locallife/cmd/`, `locallife/media/`, `locallife/ocr/`, and `locallife/wechat/`.

## Read First

- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`

## Architecture Boundaries

- Keep the HTTP three-layer split: `api/` for transport, `logic/` for business rules, `db/sqlc/` for persistence.
- Do not put business logic in handlers.
- Inject dependencies through constructors or service structs. Do not add package-level runtime globals.
- Core functions should accept `context.Context` as the first argument.
- Use `db/sqlc/constants.go` as the single source of truth for business status constants.

## Implementation Rules

- Reuse existing request error mapping patterns instead of inventing a new API error shape.
- Use structured logging. Do not add `fmt.Println` or other unstructured logging in request paths.
- Keep handler, logic, and worker files within the existing file-size guardrail enforced by `make lint-filesize`.
- Inspect nearby files in the same domain package before adding new abstractions.

## Regeneration Triggers

- If you change SQL in `locallife/db/query/` or schema assumptions, run `make sqlc`.
- If you change interfaces used by mocks, run `make mock` or `make sqlc` as appropriate.
- If you change Swagger annotations or routes, run `make swagger`.

## Validation Defaults

- Prefer `make test-unit` for focused validation.
- Run `make test-integration` only when the change touches integration flows or database-backed behavior.
- Common local commands: `make server`, `make test`, `make migrateup`, `make new_migration name=<name>`.

## Link Instead Of Duplicating

- Media backend and migration docs: `.github/standards/domains/media/*`
- OCR rollout and refactor docs: `.github/standards/domains/ocr/*`
- WeChat payment plans and runbooks: `.github/standards/domains/wechat-payment/*`
