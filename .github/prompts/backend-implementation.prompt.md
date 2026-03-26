# Backend Implementation Template

Use this template when asking for a concrete backend change in `locallife/`.

## Backend Feature Or Fix

Request:

- Implement <feature or fix>
- Keep the change complete across handler, logic, store, route, DTO, and tests when those layers are affected
- Reuse nearby patterns before introducing a new abstraction
- Use constants from `db/sqlc/constants.go` instead of new magic strings
- Tell me whether the change requires `make sqlc`, `make mock`, or `make swagger`
- Run the smallest relevant validation command and report what was executed

Required context:

- Target package or endpoint: <path>
- Expected contract or behavior: <details>

Optional context:

- Existing reference implementation: <path>
- Related domain docs: `.github/standards/backend/AGENT.md`, `.github/standards/backend/SYSTEM_PROMPT.md`, `.github/standards/backend/API_CONTRACT_STANDARDS.md`

Acceptance checklist:

- Request binding, auth extraction, and error mapping remain in handler only
- Business rules live in logic or a service, not in handler
- Persistence changes are wired through sqlc/store and actually used by logic
- Required generation steps were identified and run if source files changed
- Tests cover the new branch, failure path, or contract edge case