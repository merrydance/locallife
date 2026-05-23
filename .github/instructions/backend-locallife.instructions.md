---
applyTo: "locallife/**"
---

# Backend Instructions

Apply these rules for files under `locallife/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

More specific backend instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `locallife/api/`, `locallife/logic/`, `locallife/db/query/`, `locallife/db/sqlc/`, `locallife/worker/`, `locallife/scheduler/`, `locallife/integration/`, `locallife/cmd/`, `locallife/media/`, `locallife/ocr/`, and `locallife/wechat/`.

## Read First

- `.github/standards/engineering/README.md`
- `.github/standards/backend/README.md`

Use `.github/standards/engineering/README.md` as the stable governance index, then open the baseline, validation matrix, or matching area/domain standards when the active change needs them.

Open the smallest relevant backend deep docs for the current task instead of reading the whole backend stack every time:

- `RUNTIME_ARCHITECTURE.md`: real entrypoints, async boundaries, and takeover or high-risk path tracing
- `WORKFLOW_AND_VALIDATION.md`: regeneration triggers, local commands, and validation depth
- `API_CONTRACT_STANDARDS.md`: contract semantics, status codes, empty states, and route behavior
- `ERROR_HANDLING.md`: logging boundary, public error semantics, and safe 4xx/5xx handling when a task changes error paths or caller-facing failure behavior
- `EXTERNAL_API_CONTRACT_STANDARDS.md`: external API/provider truth source, field matrix, drift review, error mapping, and explicit downgrade rules
- `SYSTEM_PROMPT.md`: detailed layering, middleware, DTO, and implementation-shape rules when a task truly needs the deeper contract
- matching `domain README`: payment, media, OCR, and other high-risk domains

Prompt routing defaults for this area:

- Use `.github/prompts/backend-bugfix.prompt.md` for production defects, regressions, or root-cause fixes.
- Use `.github/prompts/backend-takeover.prompt.md` when a new engineer or agent needs development-ready backend context before making changes.
- Use `.github/prompts/backend-review-closure.prompt.md` for formal backend review focused on closure and propagation.

## Risk Classification

- Treat low-risk copy or presentation-only fixes as `G0` only when they do not change state semantics, trust boundaries, or user action outcomes.
- Treat normal backend product changes as `G1` when they stay within ordinary CRUD or business-path adjustments without changing money movement, authz, callbacks, async recovery, or cross-layer status semantics.
- Escalate to `G2` when the change affects status transitions, retries, workers, schedulers, idempotency, recovery, weakly ordered events, or complex field propagation across handler, logic, store, worker, or UI expectations.
- Escalate to `G2` or higher when the change touches external API/provider request, response, callback, error mapping, field propagation, or user-visible provider state.
- Escalate to `G3` when the change touches payment, refund, profit sharing, withdrawal, authentication, authorization, tenant boundaries, callbacks, uploads/downloads, media visibility, OCR, sensitive data, provider contracts for high-impact flows, or any path that could cause a high-impact production or security incident.
- When in doubt, classify upward and validate more heavily rather than treating a path as routine.

## Architecture Boundaries

- Keep the HTTP three-layer split: `api/` for transport, `logic/` for business rules, `db/sqlc/` for persistence.
- Do not put business logic in handlers.
- Inject dependencies through constructors or service structs. Do not add package-level runtime globals.
- Keep modules cohesive around business capabilities. If a change needs multiple unrelated packages to mutate the same core status, stop and re-check ownership before coding.
- Prefer a single writer for important state transitions. Other modules should call the owning capability instead of writing the same status directly through side paths.
- Prefer small, caller-shaped interfaces over broad implementation-shaped interfaces when introducing a new abstraction boundary.
- Do not create new `common`, `shared`, or helper-style abstractions just for speculative reuse; wait for stable demand from multiple real callers.
- Core functions should accept `context.Context` as the first argument.
- Do not store `context.Context` in struct fields or replace upstream context with `context.Background()` in ordinary request or task flows.
- Use `db/sqlc/constants.go` as the single source of truth for business status constants.

## Implementation Rules

- Reuse existing request error mapping patterns instead of inventing a new API error shape.
- Use structured logging. Do not add `fmt.Println` or other unstructured logging in request paths.
- Do not silently swallow unexpected errors, collapse `nil` / zero-row / missing-dependency cases into implicit success, or convert infrastructure failures into vague business success without an explicit contract that says the no-op is intentional.
- Do not ignore JSON decode or encode errors for persisted data, response-building blobs, or upstream payload fragments that affect outward behavior or stored state. Best-effort downgrade is allowed only when the contract explicitly says the field is optional and the degraded semantics are still correct.
- Do not guess external API/provider field names, nesting, types, amount units, enum values, requiredness, callback resource structure, or error codes. Use official docs and provider-confirmed samples as contract truth and update the matching domain source or field matrix before relying on the field in code.
- Do not silently downgrade external API/provider failures, malformed payloads, missing required fields, unknown enum values, signature failures, or timeouts into nil, empty DTOs, no-op states, or vague business success. Downgrade is allowed only when `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md` permits it and tests cover the branch.
- Unexpected failures must propagate to one deliberate logging boundary with enough context for diagnosis. Caller-facing responses and UI-visible messages must stay stable and semantically clear without exposing raw SQL, driver, provider, or stack details.
- For security-sensitive work, explicitly check replay, duplicate delivery, authorization, signature, injection, and sensitive-data leakage boundaries instead of assuming the caller or provider behaves correctly.
- Prefer fail-closed branches over silent fallback when a known attack class or trust-boundary pattern is relevant to the change.
- Do not add fire-and-forget goroutines in request paths; if work must outlive the request, move it to a worker, scheduler, outbox, or another explicit background boundary.
- Do not replace upstream request or task context with `context.Background()` in ordinary flows; keep cancellation, timeout, and tracing semantics threaded through the call chain.
- Do not store `context.Context` in struct fields.
- Do not use string matching on `err.Error()` when `errors.Is` / `errors.As` or typed request errors can express the branch safely.
- Do not introduce `panic(...)` in ordinary backend request, task, or state-machine flows; treat zero-row conditional updates and business conflicts as explicit errors, not crash paths.
- If a line uses `goguard:` to allow an exception, require a concrete same-line reason; bare allow markers are not acceptable.
- Keep handler, logic, and worker files within the existing file-size guardrail enforced by `make lint-filesize`.
- Inspect nearby files in the same domain package before adding new abstractions.
- After changing Go files, normalize formatting and imports before hand-off instead of leaving basic cleanup to review.
- Do not report a change as complete if the affected execution path, regeneration step, or validation command has not been checked yet.
- If documentation is in scope, keep active guidance pointed at stable index docs and treat finished rollout material as historical rather than leaving it in the default hot path.

## High-Risk Change Gates

- For payment, refund, callback, webhook, upload, media, OCR, or other externally triggered flows, verify the server-side trust boundary explicitly instead of relying on client-provided identity, status, or ownership fields.
- For external API/provider work, name the active provider and capability group, check `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md`, and use the matching domain README/source matrix when one exists. Treat official docs and provider-confirmed samples as the only structure truth source.
- For provider, callback, webhook, or replay-sensitive paths, call out the exact security pattern being enforced, the boundary that enforces it, and any residual risk if the enforcement is partial.
- For API and async failure-path changes, preserve the backend error-handling contract: business errors should stay machine- and caller-meaningful, infrastructure failures should still be observable in logs, and internal details should not leak into user-facing responses.
- For money movement, status transitions, and async recovery paths, make the persistence boundary explicit. Important state changes must be backed by persisted records, idempotency guards, and auditable transitions instead of in-memory assumptions.
- For order, delivery, reservation, and inventory work, treat conditional state updates, exclusivity rules, and release/recovery behavior as first-class concerns; do not rely on transaction-external checks or process-local state to keep them correct.
- For worker, scheduler, outbox, retry, or callback-triggered work, define duplicate-delivery behavior and failure recovery behavior deliberately. Do not leave repeated execution semantics implicit.
- Do not place third-party calls, websocket emits, or other external side effects inside transaction-owned critical sections; commit durable state first, then trigger post-commit work.
- For private media, OCR, document, or download access, preserve ownership checks, visibility rules, and secret handling. Do not weaken access assumptions just to make a path easier to wire.
- For `G2` and `G3` changes, explicitly check timeout handling, partial failure behavior, repeated delivery semantics, and what the operator or downstream caller observes when the path degrades.
- When the change is payment- or authz-sensitive, apply the matching governance and area/domain standards surfaced from `.github/standards/engineering/README.md`; for WeChat Pay use `.github/standards/domains/wechat-payment/README.md` instead of relying on generic review memory.
- When the change touches order, fulfillment, reservation, or inventory state machines, apply the matching backend and runtime standards from `.github/standards/backend/README.md` and `.github/standards/backend/RUNTIME_ARCHITECTURE.md` instead of relying on generic review memory.
- If a high-risk path cannot be validated locally, call that out as residual risk instead of implying the path is production-safe.

## Regeneration Triggers

- If you change SQL in `locallife/db/query/` or schema assumptions, run `make sqlc`.
- If you change interfaces used by mocks, run `make mock` or `make sqlc` as appropriate.
- If you change Swagger annotations or routes, run `make swagger`.

## Validation Defaults

- Prefer `make test-unit` for focused validation.
- Run `make test-integration` only when the change touches integration flows or database-backed behavior.
- When changing backend error handling, response builders, or persisted-data decoding paths, run at least one focused failure-path regression for the affected dependency, malformed blob, or degraded branch instead of validating only the happy path.
- When changing external API/provider DTOs, parsers, validators, request builders, callback handlers, or error classifiers, run focused tests or fixture checks that lock field names, types, requiredness, enum values, malformed payload handling, and error mapping.
- Common local commands: `make server`, `make test`, `make migrateup`, `make new_migration name=<name>`.
- Use `.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md` before closing a non-trivial backend implementation or fix.
- Use `.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md` after formal backend review or subsystem audit.
- Use `.github/standards/backend/FORMAL_REVIEW_DURABILITY.md` when formal backend review findings should become durable project knowledge.

## Completion Contract

- State the risk class (`G0`/`G1`/`G2`/`G3`) and why the change belongs there.
- Before hand-off, identify which layers changed or were checked: handler, logic, SQL/sqlc, worker, scheduler, route, Swagger, prompt or docs as applicable.
- State which regeneration steps were required, which were run, and which were confirmed unnecessary.
- State which validation commands were run, and which relevant validations were not run.
- If any affected path remains unverified, describe the exact residual risk instead of using a generic caveat. Residual risk should name the concrete callback, retry branch, duplicate-delivery path, authz path, or failure mode that remains unexercised.
- If a requested change stops short of a full end-to-end path, explain where it stops and why.
