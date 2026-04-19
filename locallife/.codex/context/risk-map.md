# Backend Risk Map (Compatibility Pointer)

Canonical source:

- `../../.github/standards/backend/README.md`

This legacy `.codex` file remains only as a compatibility entrypoint for older backend prompts and notes.

Use the canonical `.github` document for:

- repo-specific high-risk production chains
- merge-time invariant checks
- review bias for transaction boundaries, callback or worker idempotency, and recovery paths
- future updates when a new recurring backend failure mode is discovered

If backend risk knowledge evolves, update the `.github` canonical doc first instead of restoring a second full risk map here.
