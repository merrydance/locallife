---
applyTo: "locallife/api/**"
---

# Backend API Instructions

Apply these rules for files under `locallife/api/`.

## Role Of This Layer

- Keep handlers limited to transport concerns: bind input, read auth context, call logic or store entrypoints, map errors, and shape responses.
- Do not put business rules, status transitions, pricing logic, or persistence orchestration in handlers.

## Handler Conventions

- Reuse existing request structs, response DTO patterns, and `newXxxResponse` style mapping where the package already uses them.
- Reuse existing request error helpers instead of inventing new API error envelopes.
- Keep Swagger annotations and registered routes aligned with the actual handler path and method.
- Use validation tags and existing binding patterns for request input.
- Keep outward-facing routes under the existing `/v1/` API contract.
- Prefer strong typed response structs over ad hoc maps when shaping JSON responses.
- Do not start background goroutines from handlers to hide latency or defer correctness-sensitive work; move that work to an explicit async boundary if it must outlive the request.
- Map internal, database, driver, and upstream provider failures into stable business-facing API errors. Do not leak raw SQL text, Go driver errors, or untranslated upstream payloads into user-facing responses.

## Boundary Checks

- New request fields should propagate into logic or store calls instead of stopping at binding.
- New response fields should come from actual logic or persistence outputs instead of placeholder values.
- If an endpoint changes contract semantics, check `.github/standards/backend/API_CONTRACT_STANDARDS.md` and update tests or docs as needed.
- Treat “not yet enabled / not yet applied / not yet configured” states as business states that usually need `200` plus status fields rather than an automatic `404`.
- Keep pagination semantics explicit and stable. If a response includes `total`, it must mean the full matched result count rather than the current page length; otherwise expose `has_more`, cursor, or equivalent pagination truth explicitly and keep tests/docs aligned.

## Validation Defaults

- Prefer targeted handler tests or `make test-unit` when API behavior changes.
- If routes or Swagger annotations change, run `make swagger`.