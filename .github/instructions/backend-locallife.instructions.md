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
- Do not report a change as complete if the affected execution path, regeneration step, or validation command has not been checked yet.

## High-Risk Change Gates

- For payment, refund, callback, webhook, upload, media, OCR, or other externally triggered flows, verify the server-side trust boundary explicitly instead of relying on client-provided identity, status, or ownership fields.
- For money movement, status transitions, and async recovery paths, make the persistence boundary explicit. Important state changes must be backed by persisted records, idempotency guards, and auditable transitions instead of in-memory assumptions.
- For worker, scheduler, outbox, retry, or callback-triggered work, define duplicate-delivery behavior and failure recovery behavior deliberately. Do not leave repeated execution semantics implicit.
- For private media, OCR, document, or download access, preserve ownership checks, visibility rules, and secret handling. Do not weaken access assumptions just to make a path easier to wire.
- If a high-risk path cannot be validated locally, call that out as residual risk instead of implying the path is production-safe.

## Regeneration Triggers

- If you change SQL in `locallife/db/query/` or schema assumptions, run `make sqlc`.
- If you change interfaces used by mocks, run `make mock` or `make sqlc` as appropriate.
- If you change Swagger annotations or routes, run `make swagger`.

## Validation Defaults

- Prefer `make test-unit` for focused validation.
- Run `make test-integration` only when the change touches integration flows or database-backed behavior.
- Common local commands: `make server`, `make test`, `make migrateup`, `make new_migration name=<name>`.

## Completion Contract

- Before hand-off, identify which layers changed or were checked: handler, logic, SQL/sqlc, worker, scheduler, route, Swagger, prompt or docs as applicable.
- State which regeneration steps were required, which were run, and which were confirmed unnecessary.
- State which validation commands were run, and which relevant validations were not run.
- If any affected path remains unverified, describe the exact residual risk instead of using a generic caveat.
- If a requested change stops short of a full end-to-end path, explain where it stops and why.

## Document Hygiene

- When a task touches standards, runbooks, execution plans, cutover checklists, or migration playbooks, decide whether each referenced document is still active guidance or historical rollout material.
- If a document appears to have completed its purpose, recommend `keep`, `archive`, or `delete` in the hand-off summary.
- Do not automatically archive or delete documentation unless the user asked for documentation cleanup or the current task explicitly includes that scope.
- Keep `Read First` sections pointed at the most stable long-lived guidance. Treat execution plans and cutover materials as conditional references unless they are still the active operating baseline.

## Link Instead Of Duplicating

- Media backend and migration docs: `.github/standards/domains/media/*`
- OCR rollout and refactor docs: `.github/standards/domains/ocr/*`
- WeChat payment plans and runbooks: `.github/standards/domains/wechat-payment/*`
