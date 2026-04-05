---
applyTo: "locallife/**"
---

# Backend Instructions

Apply these rules for files under `locallife/`.

More specific backend instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `locallife/api/`, `locallife/logic/`, `locallife/db/query/`, `locallife/db/sqlc/`, `locallife/worker/`, `locallife/scheduler/`, `locallife/integration/`, `locallife/cmd/`, `locallife/media/`, `locallife/ocr/`, and `locallife/wechat/`.

## Read First

- `.github/standards/engineering/README.md`
- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/backend/GO_PRACTICES.md`
- `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Use `.github/standards/engineering/README.md` as the stable governance index, then open the baseline, validation matrix, or high-risk checklists when the active change needs them.

## Risk Classification

- Treat low-risk copy or presentation-only fixes as `G0` only when they do not change state semantics, trust boundaries, or user action outcomes.
- Treat normal backend product changes as `G1` when they stay within ordinary CRUD or business-path adjustments without changing money movement, authz, callbacks, async recovery, or cross-layer status semantics.
- Escalate to `G2` when the change affects status transitions, retries, workers, schedulers, idempotency, recovery, weakly ordered events, or complex field propagation across handler, logic, store, worker, or UI expectations.
- Escalate to `G3` when the change touches payment, refund, profit sharing, withdrawal, authentication, authorization, tenant boundaries, callbacks, uploads/downloads, media visibility, OCR, sensitive data, or any path that could cause a high-impact production or security incident.
- When in doubt, classify upward and validate more heavily rather than treating a path as routine.

## Architecture Boundaries

- Keep the HTTP three-layer split: `api/` for transport, `logic/` for business rules, `db/sqlc/` for persistence.
- Do not put business logic in handlers.
- Inject dependencies through constructors or service structs. Do not add package-level runtime globals.
- Prefer small, caller-shaped interfaces over broad implementation-shaped interfaces when introducing a new abstraction boundary.
- Core functions should accept `context.Context` as the first argument.
- Do not store `context.Context` in struct fields or replace upstream context with `context.Background()` in ordinary request or task flows.
- Use `db/sqlc/constants.go` as the single source of truth for business status constants.

## Implementation Rules

- Reuse existing request error mapping patterns instead of inventing a new API error shape.
- Use structured logging. Do not add `fmt.Println` or other unstructured logging in request paths.
- Do not add fire-and-forget goroutines in request paths; if work must outlive the request, move it to a worker, scheduler, outbox, or another explicit background boundary.
- Keep handler, logic, and worker files within the existing file-size guardrail enforced by `make lint-filesize`.
- Inspect nearby files in the same domain package before adding new abstractions.
- After changing Go files, normalize formatting and imports before hand-off instead of leaving basic cleanup to review.
- Do not report a change as complete if the affected execution path, regeneration step, or validation command has not been checked yet.
- If documentation is in scope, keep active guidance pointed at stable index docs and treat finished rollout material as historical rather than leaving it in the default hot path.

## High-Risk Change Gates

- For payment, refund, callback, webhook, upload, media, OCR, or other externally triggered flows, verify the server-side trust boundary explicitly instead of relying on client-provided identity, status, or ownership fields.
- For money movement, status transitions, and async recovery paths, make the persistence boundary explicit. Important state changes must be backed by persisted records, idempotency guards, and auditable transitions instead of in-memory assumptions.
- For worker, scheduler, outbox, retry, or callback-triggered work, define duplicate-delivery behavior and failure recovery behavior deliberately. Do not leave repeated execution semantics implicit.
- For private media, OCR, document, or download access, preserve ownership checks, visibility rules, and secret handling. Do not weaken access assumptions just to make a path easier to wire.
- For `G2` and `G3` changes, explicitly check timeout handling, partial failure behavior, repeated delivery semantics, and what the operator or downstream caller observes when the path degrades.
- When the change is payment- or authz-sensitive, apply the matching section in `.github/standards/engineering/HIGH_RISK_CHANGE_CHECKLISTS.md` instead of relying on generic review memory.
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

- State the risk class (`G0`/`G1`/`G2`/`G3`) and why the change belongs there.
- Before hand-off, identify which layers changed or were checked: handler, logic, SQL/sqlc, worker, scheduler, route, Swagger, prompt or docs as applicable.
- State which regeneration steps were required, which were run, and which were confirmed unnecessary.
- State which validation commands were run, and which relevant validations were not run.
- If any affected path remains unverified, describe the exact residual risk instead of using a generic caveat. Residual risk should name the concrete callback, retry branch, duplicate-delivery path, authz path, or failure mode that remains unexercised.
- If a requested change stops short of a full end-to-end path, explain where it stops and why.
