---
name: "后端 SQL 审查模板"
description: "Use when drafting a backend review request focused on SQL queries, migrations, sqlc propagation, indexing, and persistence semantics. Trigger phrases: review SQL change, inspect query or migration, check sqlc propagation, review db/query change, review db/sqlc change. 适用于审查 SQL、migration、sqlc 与持久层传播完整性。"
---
# Backend SQL Review Template

Use this template when asking for a backend review centered on `db/query/`, `db/sqlc/`, migrations, or SQL-driven behavior changes.

## SQL Change Review

Target area: `locallife/`

Request:

- Review this backend SQL-related change with findings first, ordered by severity
- Prioritize broken query propagation, schema regressions, unsafe write scope, missing index or constraint follow-up, transaction drift, and missing validation evidence
- Check whether SQL source changes in `db/query/`, generated code in `db/sqlc/`, logic callers, handlers, workers, schedulers, and tests still form a complete path
- Flag manual edits to generated sqlc files, orphaned queries, or new fields or statuses that stop at the SQL layer
- Call out missing regeneration steps such as `make sqlc`, `make mock`, or migration verification when the change requires them
- Check whether write queries preserve tenant or owner scoping, status preconditions, and transaction semantics where correctness depends on them
- If a query introduces filtering, sorting, pagination, aggregation, or hot-path reads, call out missing `ORDER BY`, likely index gaps, or unverified execution-plan risk when evidence is absent
- If no findings are discovered, say so explicitly and mention residual risk or unverified SQL behavior

Optional context:

- Changed files or PR scope: <paths>
- Expected behavior after the change: <details>
- Known hot tables or risk areas: <details>

## Schema And Migration Review

Request:

- Review whether schema or migration changes remain forward-compatible and are reflected in callers, tests, and rollout expectations
- Call out destructive changes, unsafe backfills, lock-risk operations, or rollback assumptions that are not supported by the implementation evidence
- Flag any migration or schema change that should have been treated as higher risk because it affects production data semantics, locking, or access scope

Optional context:

- Migration files: <paths>
- Release or rollout expectations: <details>

Related docs:

- `.github/standards/backend/SQL_STANDARDS.md`
- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/instructions/review.instructions.md`