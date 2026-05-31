# Merchant State Flow Codegraph Audit Plan

Status: planning standard for merchant-side state-flow audit
Date: 2026-05-30
Scope: merchant-side Mini Program flows and their backend state paths

## Purpose

This document defines the standard for auditing merchant-side "small flows" that are not always the main transaction path but still mutate important state.

Examples include automatic business open/close, manual open/close, merchant profile settings, membership settings, dish status changes, inventory changes, order operation state changes, notification settings, printing settings, and other configuration or status flows.

The goal is to produce a complete enough artifact for future debugging and refactoring work, including:

- backend truth-source checks
- stale or zombie function discovery
- unused branch discovery
- idempotency and duplicate-submit checks
- authorization and tenant-boundary checks
- async recovery and scheduler/worker checks
- frontend draft-state versus backend-truth alignment
- refactor boundary discovery

This is not a lightweight UI inventory. Each flow must be traced as a state-changing product workflow.

## Relationship To Existing Codegraph Work

Existing `artifacts/codegraph/backend/baofu-payment/` slices are high-risk payment-domain codegraph slices. They are variant-specific and record real route, call, write, enqueue, scheduler, transaction, callback, and provider-call edges.

Merchant state-flow audit should reuse that method, but should not merge unrelated merchant flows into the Baofoo payment directory.

Recommended artifact layout:

- `artifacts/merchant-state-flow-audit.md`
  - overall inventory, status matrix, findings, risk ranking, and remediation queue
- `artifacts/merchant-state-flow-remediation-log-YYYY-MM-DD.md`
  - durable repair ledger for fixes made after audit findings are implemented
  - every implemented fix should record commit id, affected flow, validation, and remaining risk
- `artifacts/codegraph/merchant-state-flows/`
  - per-flow `*.slice.md` files for human-audited chain narratives
  - per-flow `*.edges.json` files for machine-readable graph edges

## Completeness Standard

Do not use "infinite depth" as the completion standard. It has no stable stopping point. Instead use "complete closed-loop tracing":

1. Trace the full forward path from user-facing merchant entry to durable state and final observable result.
2. Trace reverse references from important backend and frontend nodes to find old entrypoints, multiple writers, unused functions, unused branches, and hidden side paths.
3. Identify the truth source, all writers, all readers, and all async state convergence paths.
4. Check idempotency, authorization, tenant boundaries, duplicate submit behavior, and stale draft behavior.
5. Stop only when all required endpoints below are either traced or explicitly marked not applicable.

## Required Forward Trace

For each flow, trace and record:

1. Frontend page or component entry.
2. User action handler.
3. Frontend local draft state and validation.
4. Frontend API wrapper and request shape.
5. Backend route registration.
6. Backend handler.
7. Logic/service layer, if any.
8. Transaction boundary, if any.
9. SQL query names and durable tables/fields.
10. Generated sqlc methods only as evidence, not as manually edited sources.
11. Worker, scheduler, callback, outbox, websocket, push, or polling path if the flow has async or automatic state changes.
12. Backend response builder.
13. Frontend response handling and authoritative rehydration.
14. Downstream pages or components that consume the changed state.
15. User-visible failure, retry, and re-entry behavior.

## Required Reverse Trace

For each important node discovered in the forward trace, reverse-search references and record:

1. Other frontend entrypoints that call the same API or mutate the same local state.
2. Other backend handlers that write the same table/field.
3. Other logic functions, transactions, workers, schedulers, callbacks, or maintenance jobs that write the same state.
4. Legacy functions or branches that appear unused.
5. Queries that read the same state and may rely on different semantics.
6. Tests that cover the same behavior.
7. Missing tests for high-risk or previously broken branches.

If reverse tracing finds additional state-changing paths, either include them in the same flow if they are part of the same product workflow, or add a linked follow-up flow row.

## Per-Flow Audit Matrix

Each audited flow must have a row in the overall audit with these fields:

| Field | Meaning |
| --- | --- |
| Flow ID | Stable short id, for example `merchant-business-hours-auto-open`. |
| User Task | What the merchant is trying to do. |
| Frontend Entry | Page/component and handler. |
| API Contract | API wrapper, route, request and response fields. |
| Backend Owner | Handler/logic/transaction/scheduler owner. |
| Truth Source | Durable table/field or provider fact that owns the state. |
| Writers | Every discovered writer. |
| Readers | Important consumers of the state. |
| Async Paths | Scheduler, worker, callback, outbox, websocket, polling, or none. |
| Idempotency | Duplicate submit/retry behavior and protection. |
| Authorization | Authn/authz and tenant-boundary checks. |
| Draft/Rehydration | Whether frontend uses backend truth after save or re-entry. |
| Tests | Existing focused tests and missing high-value tests. |
| Zombie/Drift Signals | Unused functions, old branches, duplicate logic, contract drift. |
| Risk | G0/G1/G2/G3 with reason. |
| Status | `clean`, `needs-fix`, `partially-fixed`, `needs-test`, `needs-slice`, or `blocked`. |

## Per-Flow Slice Format

Create a codegraph slice when a flow is complex, async, high-risk, has multiple writers, has hidden recovery paths, or is likely to be refactored.

Slice path:

`artifacts/codegraph/merchant-state-flows/<flow-id>.slice.md`

Minimum slice sections:

1. Title, status, risk class, and scope.
2. Variant coverage and explicit non-coverage.
3. Product invariant.
4. Primary forward chain with file and line evidence.
5. Reverse-reference findings.
6. SQL and durable state boundaries.
7. Trust, authorization, and tenant-boundary checks.
8. Idempotency and duplicate-submit checks.
9. Recovery and async convergence paths.
10. Frontend draft and backend rehydration behavior.
11. Test coverage signals.
12. Gaps, suspected zombie code, and refactor notes.

## Branch Exhaustion Pass

After the first complete closed-loop tracing pass, run a second per-flow branch-exhaustion pass. This pass does not add new product flows unless a truly independent state-changing workflow is found. It deepens each existing slice until the branch surface is explicit enough for future zombie-function cleanup, dead-branch removal, idempotency fixes, authorization review, and refactoring.

Add a `Branch Exhaustion` section to every `*.slice.md` and cover these checkpoints:

1. Entry branches: every Mini Program and merchant App user-facing entry, plus shared components, popup actions, pull-refresh/retry paths, websocket/push/polling entrypoints, and local-only App state that can trigger backend writes.
2. Request branches: create/update/delete/status/list/detail/manual/automatic variants, optional fields, enum values, route aliases, stale wrappers, and API paths that are exported but not called by current UI.
3. Backend state branches: every handler, logic path, transaction branch, generated SQL writer, provider command, and conditional state transition that can touch the same truth source.
4. Async branches: scheduler, worker, callback, outbox, websocket, push, polling, local notification, foreground service, and recovery jobs, including missing or intentionally absent recovery.
5. Failure and retry branches: duplicate taps, ambiguous network failures, provider accepted-but-local-failed, local committed-but-response-failed, retry/replay behavior, stale draft recovery, and user-visible re-entry.
6. Reader/consumer branches: important downstream readers and validators that rely on the changed state, especially customer checkout/order/reservation, merchant dashboard/kitchen/order views, finance/reporting, public search/detail, and operator/recovery surfaces.
7. Authorization and tenant branches: owner/manager/cashier/chef/staff distinctions, public callback/verify routes, role drift, group/merchant affiliation, and media/document ownership boundaries.
8. Zombie and unreachable branches: unused SQL, unused wrappers, legacy route names, stale frontend DTO fields, generated methods with no runtime caller, comment/code drift, and paths that appear reachable but are blocked by enum/route/order semantics.
9. Test-proof branches: branch claims that need a focused regression or search proof before deletion/refactor, including cross-client convergence and race/idempotency cases.

For each checkpoint, either list concrete branches and evidence or state `None found in current Mini Program/App scope` with the search surface used. Do not mark a flow as branch-exhausted just because the main path is documented.

Recommended order for the branch-exhaustion pass:

1. G3 payment/refund/money and order-impacting flows: `merchant-order-operations`, `merchant-finance-withdrawal`, `merchant-claim-recovery`, `merchant-member-balance-adjust`.
2. G3 auth, private-material, organization, and onboarding flows: `merchant-app-bind-and-device`, `merchant-application-onboarding`, `merchant-staff-and-group`.
3. G3/G2 shared table/reservation and automatic state flows: `merchant-reservation-and-table`, `merchant-business-hours-auto-open`, `merchant-manual-open-status`, `merchant-device-display-config`.
4. Remaining merchant configuration/catalog/marketing/content flows: `merchant-dish-status-and-inventory`, `merchant-combo-and-catalog`, `merchant-marketing-rules`, `merchant-profile-update`, `merchant-membership-settings`, `merchant-review-reply`.

Edges path:

`artifacts/codegraph/merchant-state-flows/<flow-id>.edges.json`

Edges should represent real relationships only:

- route
- call
- write
- read
- transaction
- enqueue
- scheduler
- callback
- outbox
- websocket or push event
- frontend API call

Warnings and non-relationships belong in the markdown slice, not in edges.

## Risk Classification

Use project risk levels:

- G0: presentation-only or copy-only, no state semantics.
- G1: ordinary merchant configuration or CRUD with single writer and no async recovery.
- G2: status changes, scheduler/worker, retries, idempotency, cross-page recovery, or multiple writers.
- G3: payment, refund, withdrawal, provider callback, authz-sensitive tenant boundaries, private materials, or high-impact state transitions.

When unsure, classify upward.

## Execution Order

Start with merchant Mini Program flows and backend state paths.

Recommended first batch:

1. Merchant business hours and automatic open/close.
2. Manual merchant open/close.
3. Merchant profile and logo update.
4. Merchant membership settings.
5. Dish status, inventory, and customization edits.
6. Merchant order operations: accept, reject, prepare, complete, cancel where applicable.
7. Merchant notification, printing, and auto-accept settings.
8. Merchant finance and withdrawal entry settings if they appear in merchant-side UI.

For each batch:

1. Inventory frontend pages under `weapp/miniprogram/pages/merchant/**`.
2. Map frontend API wrappers under `weapp/miniprogram/api/**` and merchant page-local APIs.
3. Map backend routes under `locallife/api/server.go`.
4. Trace handlers, logic, transactions, SQL, workers, schedulers, callbacks, websocket, and outbox paths.
5. Reverse-search important state writers and readers.
6. Fill the audit matrix.
7. Create codegraph slices for flows marked `needs-slice`.
8. Run targeted validation only when code or tests are changed; audit-only work should state that no runtime validation was required.

## Remediation Ledger Rule

When a finding from this audit is fixed, do not rely on chat history or a single commit message as the durable memory.

Update all relevant artifacts in the same branch:

1. The repair ledger under `artifacts/merchant-state-flow-remediation-log-*.md`.
2. The parent flow row and next-step status in `artifacts/merchant-state-flow-audit.md`.
3. The matching `artifacts/codegraph/merchant-state-flows/<flow-id>.slice.md`.

Each repair entry should include:

- the commit id
- the flow id
- the risk class
- what was fixed
- what validation was run
- what risk remains

## Completion Criteria

The merchant-side audit is complete when:

1. Every merchant-side state-changing page/action has an audit row or is explicitly marked out of scope with reason.
2. Every audited flow names its durable truth source.
3. Every audited flow names all discovered writers and important readers.
4. Every async/automatic flow has its scheduler/worker/callback/outbox path traced or marked absent.
5. Every flow has authorization and tenant-boundary notes.
6. Every flow has idempotency and duplicate-submit notes.
7. Every flow states frontend draft and backend rehydration behavior.
8. Every flow has test coverage notes.
9. Every suspected zombie function, unused branch, duplicate writer, and contract drift is listed.
10. Every G2/G3 or multi-writer flow has a codegraph slice, unless explicitly deferred with reason.
11. Every slice has a `Branch Exhaustion` section using the checkpoint list above.
12. Every branch-exhaustion section states the remaining proof gaps, not only the known defects.

## Current Seed Finding

`merchant-business-hours-auto-open` is the seed example.

Observed chain:

- Frontend reads and writes `auto_open_by_business_hours` through `GET/PUT /v1/merchants/me/business-hours`.
- Backend persists `merchants.auto_open_by_business_hours`.
- Weekly business hours support multiple rows per day.
- Scheduler uses current day effective rows and opens the merchant when current local time falls within any non-closed row.
- Frontend "sync" action copies a configured day into local draft state only; the backend truth changes only after save.

This seed flow should become the first row in the audit matrix and likely the first merchant-state codegraph slice.
