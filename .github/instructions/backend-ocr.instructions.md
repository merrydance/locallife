---
applyTo: "locallife/ocr/**"
---

# Backend OCR Instructions

Apply these rules for files under `locallife/ocr/`.

## Read First

- `.github/standards/domains/ocr/OCR_REFACTOR_EXECUTION_PLAN_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_ACCEPTANCE_CHECKLIST_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_RELEASE_RUNBOOK_2026-03-25.md`
- `.github/standards/domains/ocr/OCR_TEST_ENV_E2E_CHECKLIST_2026-03-25.md`

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

## Validation Defaults

- Prefer focused package tests for provider behavior, routing, retry logic, and service orchestration.
- If OCR changes affect job schema, generated queries, worker payloads, or API contracts, run the necessary regeneration and validation commands as part of the change.