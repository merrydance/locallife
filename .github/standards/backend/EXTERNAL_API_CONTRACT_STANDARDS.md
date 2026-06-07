# External API Contract Standards

> Purpose: make external provider contracts traceable, typed, reviewable, and hard to guess.

This standard applies whenever backend code calls, parses, stores, maps, or exposes data from an external API, SDK, provider callback, webhook, file export, or provider-side status query.

Examples include payment providers, OCR providers, map/location providers, cloud print vendors, delivery providers, identity or verification providers, and any future provider whose request or response shape affects LocalLife state, user-visible behavior, reconciliation, or operator decisions.

## 1. Risk Classification

Treat external API contract work as at least `G2` when it affects stored state, async recovery, cross-layer field propagation, user-visible workflow state, or operator decisions.

Escalate to `G3` when the provider affects money movement, payment, refund, profit sharing, withdrawal, identity, authorization, tenant boundaries, callbacks, OCR, sensitive data, legal evidence, auditability, or any path that could create a high-impact production or security incident.

If unsure, classify upward and validate more heavily.

## 2. Contract Truth Source

Official provider documentation and provider-confirmed samples are the contract truth source. Production logs, sandbox observations, SDK DTOs, old code, nearby interfaces, generated frontend expectations, or historical artifacts are evidence only after the domain source ledger records how they relate to the official contract.

Before changing production code, collect or reference the smallest authoritative source set for the active capability:

- Official request documentation.
- Official response documentation.
- Official callback or webhook resource structure.
- Official error code and status documentation.
- Provider examples or provider-confirmed samples when available.
- Current sandbox or production evidence only when clearly labeled as evidence, not contract truth.

Cleaned contract material must live in the matching domain directory when the provider has a domain standard, for example `.github/standards/domains/wechat-payment/` or `.github/standards/domains/baofu-payment/`. If no domain directory exists, store the minimal source matrix, field matrix, or evidence note under the narrowest existing backend or domain standard path instead of burying it in chat history or code comments.

## 3. Required Field Matrix

Every external API contract that affects production behavior must have an explicit field matrix before code relies on it. The matrix can be a domain README section, a dedicated matrix file, or a structured table in a provider standard.

For each request, response, callback, and error-code structure, record:

- Provider capability group and endpoint identity.
- HTTP method and path or SDK operation name.
- Field name using provider spelling.
- JSON/XML/form nesting path.
- Type and unit, including amount units, timestamps, and boolean encoding.
- Required, optional, conditional-required, or provider-generated status.
- Conditional rules and mode restrictions.
- Enum values, status values, error codes, and aliases.
- Description and business purpose.
- LocalLife contract type, parser, validator, or error mapper that owns the field.
- Source reference: official page, source ledger row, sample fixture, or evidence note.

Do not treat "the code compiles" or "the JSON unmarshals" as proof that the contract is correct.

## 4. Implementation Rules

Provider-specific DTOs, request builders, response parsers, callback verification, error classifiers, and enum definitions must stay inside the provider integration boundary. Business logic, handlers, workers, schedulers, and frontend-facing DTOs should depend on LocalLife capability methods and stable internal semantics, not raw provider payloads.

When adding or changing a provider field:

- Update the contract matrix and source evidence in the same change.
- Update request DTOs, response DTOs, callback DTOs, parser, validator, and error mapping together when affected.
- Preserve provider field spelling, enum values, endpoint paths, callback ACKs, amount units, and version constants from the matrix.
- Add focused tests or fixtures that lock provider field names, enum values, required fields, and parse or validation behavior.
- Check all callers that consume the mapped internal semantics, including logic, workers, schedulers, APIs, Swagger, and frontend surfaces when relevant.

## 5. Prohibited Shortcuts

Do not:

- Guess field names, nesting, types, enum values, amount units, or callback resource structure.
- Infer a provider contract from old code, an adjacent endpoint, a different provider, a different merchant mode, or frontend expectations.
- Copy SDK DTOs through business layers as the LocalLife public contract.
- Silently serialize empty optional fields, guessed defaults, proxy-mode fields, upload-file fields, or provider-mode-specific fields outside the current operating mode.
- Treat sync responses as terminal success unless the domain standard explicitly says the provider field is terminal.
- Treat unknown enum values, missing required fields, malformed payloads, signature failures, or provider-side errors as success.
- Hide provider failures behind nil, empty DTOs, best-effort parsing, or generic no-op states.

## 6. Downgrade Policy

Implicit downgrade is forbidden.

A downgrade is allowed only when all of these are true:

1. The contract explicitly says the field or capability is optional or safely unavailable.
2. The degraded business semantics remain correct.
3. The downgrade is observable through structured logs or metrics.
4. The caller-facing response or UI state gives a stable, business-readable next step.
5. A focused test covers the downgrade branch.
6. The hand-off names the degraded behavior and remaining risk.

Otherwise, fail fast with a stable mapped error and log the provider detail at the correct logging boundary.

## 7. Error Handling And User Guidance

Provider errors must be classified and mapped deliberately.

- Unexpected provider failures, malformed payloads, signature failures, unknown enums, timeout, and 5xx/transport failures must reach one structured logging boundary with enough context to diagnose the provider, endpoint, capability group, correlation id, local object id, and error class.
- Logs must not include secrets, full credentials, certificate material, raw sensitive payloads, identity numbers, bank cards, tokens, or provider fields that domain standards mark as sensitive.
- Public API responses must use stable LocalLife semantics. Do not expose raw provider text, SQL errors, Go driver errors, stack traces, untranslated English diagnostics, or unstable provider payloads to frontend surfaces. When a provider returns a user-actionable validation reason, expose it only through a deliberate classifier/sanitizer that removes sensitive/internal fields and preserves a clear next step.
- Frontend-visible failures must give semantically clear guidance: what failed, whether retry is possible, whether the operator must wait, re-submit, verify configuration, contact support, or check provider-side processing.
- Business validation errors can map to stable 4xx semantics. Infrastructure, provider, timeout, or malformed-provider-data failures must remain observable and must not be converted into vague business success.

## 8. Drift Review And Refresh

External API contracts are living dependencies. Domain owners must periodically re-check active provider documentation, especially before changing a provider integration, enabling a dormant capability, upgrading an SDK, responding to provider-side failures, or closing a release that depends on provider behavior.

At review time, verify whether:

- The official page, source matrix, and field matrix still agree.
- New provider fields, removed fields, enum values, status values, or error codes need code or test updates.
- Sandbox or production evidence contradicts the documented matrix.
- The evidence is provider-confirmed or only observed behavior.
- Domain README, fixtures, parsers, validators, and error mappers were updated together.

If the provider contract changed but code cannot be updated immediately, record the drift, affected capability, current production risk, mitigation, and re-evaluation trigger in the relevant domain standard or risk ledger. Do not leave contract drift only in chat or a temporary plan.

## 9. Review Checklist

For any external API change, review must check:

- Is the active provider and capability group named?
- Are official docs, samples, source matrix, and field matrix referenced?
- Are field names, types, requiredness, conditional rules, enum values, amount units, and error codes explicit?
- Are provider DTOs contained inside the provider boundary?
- Are unknown enum, missing field, malformed payload, timeout, and provider error paths mapped deliberately?
- Are unexpected failures logged once with safe structured context?
- Does the frontend or caller receive stable business-readable guidance?
- Are downgrade branches explicit, tested, and contract-backed?
- Were parsers, validators, request builders, callback handling, fixtures, and callers updated together?
- Is any unverified provider behavior called out as concrete residual risk?
