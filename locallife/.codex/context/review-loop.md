# Review Loop (Compatibility Pointer)

Canonical source:

- `../../.github/standards/backend/FORMAL_REVIEW_DURABILITY.md`

This legacy `.codex` file remains only as a compatibility entrypoint for older backend prompts and notes.

Canonical review policy and active ledgers now live in `.github`:

- active findings: `../../.github/review/open-findings.md`
- audit log: `../../.github/review/audit-log.md`

When a formal backend review reveals a recurring bug class or a reusable backend risk, update:

- `../../.github/standards/backend/README.md` or the matching domain README

Use the canonical `.github` document for:

- deciding when a backend review is formal enough to require durable outputs
- deciding what must be written back into standards, prompts, workflows, tests, or runbooks
- defining closeout expectations and scope bias for funds, state transitions, ownership, and regeneration gaps
