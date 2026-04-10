# LocalLife Backend Architecture (Compatibility Pointer)

Canonical source:

- `../../.github/standards/backend/RUNTIME_ARCHITECTURE.md`

This legacy `.codex` file remains only as a compatibility entrypoint for older backend prompts and notes.

Use the canonical `.github` document for:

- real composition roots such as `main.go` and `api/server.go`
- startup order and production guards
- runtime layer boundaries across `api`, `logic`, `db/sqlc`, `worker`, and `scheduler`
- high-signal domain chains like order, payment, refund, delivery, media, and OCR
- generated artifact touchpoints that affect runtime behavior

If the runtime architecture changes, update the `.github` canonical doc first instead of restoring a second full copy here.
