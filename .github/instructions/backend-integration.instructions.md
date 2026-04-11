---
applyTo: "locallife/integration/**"
---

# Backend Integration Test Instructions

Apply these rules for files under `locallife/integration/`.

## Role Of This Layer

- Keep this directory focused on end-to-end and database-backed integration coverage for real workflows.
- Use integration tests to verify cross-layer behavior that cannot be trusted from handler, logic, or store unit tests alone.

## Test Conventions

- Prefer complete workflow coverage over isolated helper assertions when the test already pays the cost of integration setup.
- Reuse existing journey-style and scenario-style patterns in nearby tests before creating a new harness shape.
- Keep setup explicit enough that schema, persistence, and API or service expectations remain understandable.
- Avoid moving unit-test-only logic into integration tests when a focused unit test would be cheaper and clearer.

## Boundary Checks

- New integration tests should exercise a real cross-layer path such as API to logic to persistence, or scheduler and worker effects that depend on real state transitions.
- If a change affects a database-backed workflow, decide explicitly whether `make test-integration` is required instead of assuming unit coverage is sufficient.
- Integration assertions should validate business outcomes and persisted state, not just the immediate function return.

## Validation Defaults

- Run `make test-integration` for changes that touch integration flows or database-backed behavior.
- Prefer `go test -v -cover -count=1 -p 1 ./integration` only when you need the direct command form from the project root.