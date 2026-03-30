---
applyTo: "locallife/ocr/**"
---

# Backend OCR Instructions

Apply these rules for files under `locallife/ocr/`.

## Read First

- `.github/standards/domains/ocr/README.md`
- `.github/standards/domains/ocr/OCR_OPERATIONS_RUNBOOK_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_ALIYUN_RAM_STS_MIN_PERMISSION_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_BASELINE_EVALUATION_2026-03-25.md`

## Historical Rollout References

- Consult `.github/standards/domains/ocr/historical/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`, `.github/standards/domains/ocr/historical/OCR_RELEASE_RUNBOOK_2026-03-25.md`, `.github/standards/domains/ocr/historical/OCR_TEST_ENV_E2E_CHECKLIST_2026-03-25.md`, `.github/standards/domains/ocr/historical/OCR_UNIFIED_API_CUTOVER_CHECKLIST_2026-03-25.md`, `.github/standards/domains/ocr/historical/OCR_ROLLBACK_GUIDELINES_2026-03-25.md`, or `.github/standards/domains/ocr/historical/OCR_ACCEPTANCE_CHECKLIST_2026-03-25.md` only when the task changes rollout, release, cutover, rollback, or acceptance behavior.

## Role Of This Layer

- Keep this package responsible for OCR provider abstraction, routing, job orchestration, privacy-sensitive media reading, retry behavior, and normalized result handling.
- Treat OCR execution as a media-asset-based asynchronous workflow, not as a place to restore old multipart or local-path-based runtime flows.

## OCR Conventions

- Preserve provider and router abstractions instead of hardcoding vendor selection in callers.
- Keep job creation idempotent when the current service patterns already do so.
- Read binary content through configured readers and media-asset-based flows rather than introducing local file path dependencies.
- Preserve both raw provider output handling and normalized result handling where the existing design requires both.

## Boundary Checks

- New OCR behavior should thread through job persistence, worker execution, result retrieval, and caller updates instead of stopping at one layer.
- Do not leak provider-specific request shaping into handlers or unrelated business packages.
- Privacy-sensitive document handling must not rely on public URLs or weakened access assumptions.

## High-Risk OCR Gates

- Keep OCR job creation, leasing, retry, and repeated worker execution behavior explicit and idempotent. Do not assume a task runs exactly once.
- Preserve retryable versus non-retryable error classification deliberately. Provider auth failures, permission failures, missing media, and malformed inputs must not silently fall into the generic retry bucket.
- Keep provider credentials, endpoint assumptions, and feature flags aligned with the active security model. Do not introduce undocumented config fallback or client-visible secrets.
- Preserve both normalized business output and provider-specific raw result handling when the current design requires auditability or provider comparison.
- If an OCR change weakens ownership checks, private-media reads, or result visibility, treat it as a security regression unless the rules are intentionally redefined in the standards.

## Validation Defaults

- Prefer focused package tests for provider behavior, routing, retry logic, and service orchestration.
- If OCR changes affect job schema, generated queries, worker payloads, or API contracts, run the necessary regeneration and validation commands as part of the change.
- If an OCR task touches release or cutover material, report whether those documents are still active references or should be treated as historical rollout records.