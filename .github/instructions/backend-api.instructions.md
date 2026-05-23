---
applyTo: "locallife/api/**"
---

# Backend API Instructions

Apply these rules for files under `locallife/api/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Layer

- Keep handlers limited to transport concerns: bind input, read auth context, call logic or store entrypoints, map errors, and shape responses.
- Do not put business rules, status transitions, pricing logic, or persistence orchestration in handlers.

## Handler Conventions

- Reuse existing request structs, response DTO patterns, and `newXxxResponse` style mapping where the package already uses them.
- Reuse existing request error helpers instead of inventing new API error envelopes.
- Known business failures should be mapped through the existing request-error path. Unexpected failures must go through the existing structured logging boundary such as `internalError(...)` or the repo's equivalent logged server-error helper rather than being ignored or returned as a raw body.
- Keep Swagger annotations and registered routes aligned with the actual handler path and method.
- Use validation tags and existing binding patterns for request input.
- Keep outward-facing routes under the existing `/v1/` API contract.
- Prefer strong typed response structs over ad hoc maps when shaping JSON responses.
- Do not start background goroutines from handlers to hide latency or defer correctness-sensitive work; move that work to an explicit async boundary if it must outlive the request.
- Map internal, database, driver, and upstream provider failures into stable business-facing API errors. Do not leak raw SQL text, Go driver errors, or untranslated upstream payloads into user-facing responses.
- Do not use 5xx `errorResponse(...)` patterns or silent early returns that skip logging for unexpected failures. If the status must stay `502` or `503`, log the real error and return a stable public message.
- Treat persisted JSON, serialized blobs, and upstream payload fragments that shape outward responses as contract-bearing data. Do not discard decode failures with `_ = parseJSON(...)`, `_ = json.Unmarshal(...)`, or equivalent best-effort patterns unless the field has a narrow, explicit downgrade contract.
- If a handler exposes or maps external API/provider state, preserve the provider contract boundary from `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md`: do not pass raw provider payloads, unknown enums, unstable provider text, or guessed fields into public API responses.

## Boundary Checks

- New request fields should propagate into logic or store calls instead of stopping at binding.
- New response fields should come from actual logic or persistence outputs instead of placeholder values.
- Errors returned from logic, store, or upstream clients should either be intentionally translated into stable caller-facing semantics or propagated to the handler's logging boundary; they must not disappear as `nil`, empty DTOs, or silent success.
- If a handler or response builder decodes persisted JSON or embedded payloads, malformed stored data must either fail the request or follow an explicitly documented downgrade contract; do not silently drop response fields and continue with `200` by default.
- Provider failures, timeout, malformed provider payloads, unknown provider statuses, and missing required provider fields must produce stable business-facing guidance and structured logs at the server boundary; do not return raw provider diagnostics or generic success.
- If an endpoint changes contract semantics, check `.github/standards/backend/API_CONTRACT_STANDARDS.md` and update tests or docs as needed.
- Treat “not yet enabled / not yet applied / not yet configured” states as business states that usually need `200` plus status fields rather than an automatic `404`.
- Keep pagination semantics explicit and stable. If a response includes `total`, it must mean the full matched result count rather than the current page length; otherwise expose `has_more`, cursor, or equivalent pagination truth explicitly and keep tests/docs aligned.

## Validation Defaults

- Prefer targeted handler tests or `make test-unit` when API behavior changes.
- When a handler change touches error propagation, response assembly, persisted JSON decoding, or upstream/store failures, add or run at least one focused failure-path regression that exercises the failing dependency or malformed-data branch.
- If routes or Swagger annotations change, run `make swagger`.
