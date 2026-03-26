---
applyTo: "locallife/scheduler/**"
---

# Backend Scheduler Instructions

Apply these rules for files under `locallife/scheduler/`.

## Role Of This Layer

- Keep this layer responsible for recurring jobs, scheduler registration, and time-based orchestration.
- Use scheduler code to trigger domain work on a schedule, not to duplicate core business rules that should live in `locallife/logic/` or worker flows.

## Scheduler Conventions

- Preserve existing manager and runnable scheduler patterns.
- Keep schedule-triggered actions explicit and easy to trace back to their downstream logic or worker entrypoints.
- Avoid embedding transport concerns or user-facing response shaping in scheduler code.
- Reuse existing logging and failure-handling patterns instead of adding ad hoc debug output.

## Boundary Checks

- New scheduled jobs should be registered through the existing scheduler wiring instead of remaining as dead code.
- Scheduled work should have a clear downstream effect in logic, worker, or persistence layers.
- Time-based branches and cutoff logic should be covered by focused tests when behavior is not trivial.

## Validation Defaults

- Prefer focused scheduler tests where available, especially for registration and time-based branching.
- Run `make test-unit` when scheduler behavior changes unless the task explicitly requires broader coverage.