---
applyTo: "locallife/cmd/**"
---

# Backend Command Instructions

Apply these rules for files under `locallife/cmd/`.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions and prompt, and confirm the task scope before continuing. Do not keep relying on stale context.

## Role Of This Layer

- Keep this directory for command-line entrypoints, maintenance tools, audits, imports, and operational utilities.
- Use command packages as thin orchestration layers that wire config, dependencies, and execution flow, not as a place for core domain logic to live permanently.

## Command Conventions

- Preserve the existing one-command-per-directory structure.
- Keep flags, config loading, and output behavior explicit and predictable.
- Reuse logic, store, and service layers instead of duplicating business rules inside command binaries.
- Prefer structured or intentionally formatted operator-facing output over ad hoc debug prints.

## Boundary Checks

- New command behavior should have a clear operational purpose and a bounded execution path.
- If a command mutates data or depends on generated query surfaces, ensure the underlying logic and persistence layers are updated instead of hiding behavior in the entrypoint.
- One-off migration or audit utilities should still respect existing config, dependency injection, and error-handling patterns.

## Validation Defaults

- Run the smallest relevant validation command for the layers touched by the command.
- If a command depends on changed SQL or generated interfaces, run `make sqlc` and `make mock` as needed before validation.