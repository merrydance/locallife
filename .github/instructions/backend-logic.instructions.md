---
applyTo: "locallife/logic/**"
---

# Backend Logic Instructions

Apply these rules for files under `locallife/logic/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Layer

- Keep this layer responsible for business rules, status transitions, authorization decisions, and orchestration across store and service dependencies.
- Do not depend on `gin.Context`, HTTP request objects, or transport-only DTOs here.

## Logic Conventions

- Accept `context.Context` as the first argument.
- Prefer explicit input and result structs or service methods with constructor-injected dependencies.
- Prefer narrow interfaces defined by actual caller needs instead of lifting large concrete service surfaces into a broad dependency contract.
- Use `logic.NewRequestError(...)` patterns where business failures need to map to request-level errors.
- Reuse constants from `locallife/db/sqlc/constants.go` instead of redefining status strings.
- Do not introduce package-level mutable runtime state for caches, clients, or configuration.
- Do not store `context.Context` in service structs or switch to `context.Background()` inside ordinary logic flows.
- Do not rely on in-process maps, mutexes, websocket state, or cache projections as the source of truth for order, payment, refund, delivery, reservation, or inventory correctness.
- When a business action needs both durable state and side effects, write the durable anchor first and emit tasks, notifications, or third-party calls only after the state boundary is safe.
- Worker, scheduler, callback, and retry orchestration should classify retryable vs non-retryable outcomes explicitly; do not loop all failures through the same retry path by default.
- Keep request-path fan-out bounded and deliberate; if one logic method needs multiple downstream reads or calls, define the concurrency limit and partial-failure behavior explicitly.

## Boundary Checks

- Outputs computed in logic should either affect persistence, returned results, emitted tasks, or downstream behavior.
- New logic methods should have a clear production caller in handlers, workers, schedulers, or other logic paths.
- Avoid hiding transport or persistence assumptions in ad hoc helper functions when a nearby service pattern already exists.
- Keep transport-only request parsing and response shaping out of this layer.

## Validation Defaults

- Prefer focused unit coverage for new branches, status transitions, and failure paths.
- Run `make test-unit` when logic behavior changes unless the task explicitly needs integration coverage.