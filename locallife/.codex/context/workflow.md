# Local Development Workflow (Compatibility Pointer)

Canonical source:

- `../../.github/standards/backend/WORKFLOW_AND_VALIDATION.md`

This legacy `.codex` file remains only as a compatibility entrypoint for older backend prompts and notes.

Use the canonical `.github` document for:

- common backend commands and working directory rules
- DB-backed test behavior and migration assumptions
- regeneration triggers for `make sqlc`, `make swagger`, and `make check-generated`
- validation strategy, repo-specific heuristics, and high-risk regression defaults such as `make test-safety`

If the backend workflow changes, update the `.github` canonical doc first instead of rebuilding a second full workflow copy here.
