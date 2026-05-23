---
applyTo: "locallife/baofu/**"
---

# Backend Baofoo/Baofu Instructions

Apply these rules for files under `locallife/baofu/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Read First

- `.github/standards/domains/baofu-payment/README.md`

Use `.github/standards/domains/baofu-payment/README.md` as the single Baofoo/Baofu domain source of truth inside the repository.

Before implementing or reviewing any Baofoo/Baofu account, aggregate payment, merchant report, callback, refund, share, withdrawal, or smoke-script contract change, identify the active capability group and read the smallest matching contract source:

- `CONTRACT_SOURCE_MATRIX.md` for official page groups, evidence labels, source truth rules, and interface coverage.
- `BAOCAITONG_FIELD_CONTRACT_MATRIX.md` for request, response, callback, enum, error-code, ACK, endpoint, amount-unit, and conditional-required field details.
- `API_CONTRACT_COVERAGE_AUDIT.md` for C0-C4 status and known evidence gaps.
- `CONTRACT_IMPLEMENTATION_MAP.md` when checking local DTO/parser/service/test coverage.
- `SANDBOX_EVIDENCE_LEDGER.md` before making any C4 or sandbox/production evidence claim.

## Integration Boundary

- Keep Baofoo provider transport, envelope, signing, DTOs, parsers, provider error mapping, and provider-specific validation inside `locallife/baofu/**`.
- Do not let Baofoo DTOs, raw upstream payloads, signatures, certificate material, full identifiers, bank cards, identity numbers, or provider error text leak into logic, API responses, frontend state, or unsafe logs.
- Treat official docs as contract truth. Sandbox, production logs, Java demo code, and smoke scripts are evidence only unless the domain source matrix records a provider-confirmed rule.
- Do not implement by analogy from WeChat payment, old Baofoo demos, or nearby DTOs. Check the official page row and field matrix first.
- When adding or changing a provider field, update DTO/parser/validator tests and the matrix row in the same change.

## Drift Gates

- Optional and conditional Baofoo fields must stay explicit. Do not silently serialize empty fields, guessed defaults, proxy-mode fields, upload-file fields, or qualification fields outside the current LocalLife operating mode.
- Request builders must preserve field names, enum values, amount units, callback ACKs, and version constants from the field matrix.
- Callback parsers must fail closed on missing signature, unsupported `notifyType`, route/type mismatch, unsupported terminal state, or undocumented success interpretation.
- Payment, refund, share, withdrawal, and account-opening sync responses must not be treated as terminal success unless the domain standard explicitly says that field is terminal.
- If a real provider sample differs from the matrix, record it as sandbox/production evidence and update the source matrix before changing production contract semantics.

## Validation Defaults

- Run focused Baofoo package tests for changed DTO, client, signing, parser, or error-classifier behavior.
- Run `make check-baofu-contract` from `locallife/` for any Baofoo contract-boundary change.
- Run `make check-generated` when SQL, sqlc, mocks, routes, or Swagger outputs are affected by the Baofoo change.
- If validation is skipped, state the exact unverified contract row, callback branch, provider state, or amount/unit conversion that remains risky.
