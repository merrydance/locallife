# LocalLife Codegraph Artifacts

This directory is the durable source of truth for LocalLife codegraph review
artifacts.

The CodeGraph CLI and its local SQLite index are auxiliary discovery tools. They
can help generate, refresh, or cross-check these artifacts, but their output is
not final business-semantics truth until it has been reviewed against the
current code, SQL, route wiring, and domain standards, then written back here.

## Source-Of-Truth Order

Use this order when sources disagree:

1. Current runtime code, SQL query sources, route registration, generated code,
   and domain standards.
2. Human-audited `artifacts/codegraph/**/*.slice.md` and `*.edges.json`
   artifacts after they have been updated against current code.
3. CodeGraph CLI and `.codegraph/codegraph.db` output as discovery evidence.

For `G2` and `G3` flows, especially payment, refund, profit sharing, withdrawal,
provider callbacks, recovery schedulers, OCR, identity, authorization, and
delivery state machines, do not treat CodeGraph output alone as sufficient.
Those flows need LocalLife-aware review of SQL effects, status transitions,
idempotency, trust boundaries, async convergence, and user-visible recovery.

## How To Use CodeGraph

Use CodeGraph first when it can reduce search time:

- Broad symbol discovery and likely call paths.
- Route-to-handler and interface-to-implementation hints.
- Duplicate or stale frontend wrapper detection.
- Unused private helper and unreachable entrypoint candidates.
- Blast-radius and focused-test surface hints.
- Mechanical drift checks for artifact file and line anchors.

Then verify findings before changing artifacts:

- Confirm full route paths, including Gin group prefixes and middleware context,
  from source code.
- Confirm SQL table effects from `locallife/db/query/*.sql`, transactions, and
  generated sqlc call sites.
- Confirm provider callback and scheduler semantics from the owning backend
  domain docs and production path.
- Keep conceptual nodes, provider nodes, table nodes, route-group nodes, and SQL
  source nodes clearly labeled in `*.edges.json`.
- Put warnings, non-relationships, and unresolved questions in the Markdown
  slice, not as graph edges.

## Updating A Slice

When creating or changing a codegraph slice:

1. Name the exact flow variant and boundary being covered.
2. Use CodeGraph to find candidate symbols, wrappers, routes, and call paths.
3. Trace the production path in source, including SQL and async side effects.
4. Update the Markdown slice with invariants, recovery paths, evidence, gaps,
   validation, and residual risk.
5. Update the `*.edges.json` only with real page, route, call, write, enqueue,
   scheduler, transaction, or provider-call relationships.
6. Refresh stale file and line anchors touched by the slice.
7. Validate edge JSON with `jq empty` for the touched `*.edges.json` files.

## Cross-Check Evidence

The first third-party CodeGraph cross-check is recorded in
`codegraph-tool-crosscheck-2026-06-13.md`.

That evidence supports the current workflow:

- CodeGraph is useful for static structure and drift discovery.
- LocalLife artifacts remain the durable source for business semantics.
- High-risk flow correctness still depends on LocalLife-aware review, not direct
  CodeGraph edge overlap.
