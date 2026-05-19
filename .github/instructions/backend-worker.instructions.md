---
applyTo: "locallife/worker/**"
---

# Backend Worker Instructions

Apply these rules for files under `locallife/worker/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Layer

- Keep this layer responsible for asynchronous task distribution, processing, retries, and worker-specific orchestration.
- Use workers for background execution boundaries, not as a place to hide general business logic that should live in `locallife/logic/`.

## Worker Conventions

- Preserve existing task distributor and task processor patterns.
- Keep payloads minimal and identifier-based where existing code does so.
- Make task handlers safe for retries and re-entry; do not assume a task runs exactly once.
- Reuse existing logging and observability helpers instead of ad hoc prints or panic-based debugging.
- Prefer task handlers to delegate business decisions to logic or store-backed services instead of embedding large domain rule sets inline.
- Keep goroutine and channel ownership explicit inside worker code; do not add background fan-out without a clear owner, cancellation path, and duplicate-delivery story.

## Boundary Checks

- New tasks should have a clear enqueue path from production code and a clear processing path in the worker package.
- Worker handlers should call into store, logic, or service dependencies explicitly rather than embedding unrelated domain rules inline.
- Status changes or side effects triggered by workers should be reflected in persistence and covered by tests when the branch is non-trivial.

## Validation Defaults

- Prefer focused worker tests for retry, failure, and idempotency-sensitive behavior.
- Run `make test-unit` when worker behavior changes unless the task explicitly requires broader integration coverage.