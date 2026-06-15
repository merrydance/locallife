# Production Risk Audit Handoff

Date: 2026-06-14
Updated: 2026-06-15
Scope: reusable flows under `artifacts/codegraph`
Mode: documentation-only, read-only production code

## Completed Artifacts

- Flow inventory and six-phase coverage matrix:
  `artifacts/production-risk-audit/flow-inventory-2026-06.md`
- State sequencing ledger:
  `artifacts/production-risk-audit/state-sequencing-ledger-2026-06.md`
- Idempotency/retry ledger:
  `artifacts/production-risk-audit/idempotency-retry-ledger-2026-06.md`
- Authorization boundary ledger:
  `artifacts/production-risk-audit/authorization-boundary-ledger-2026-06.md`
- Transaction consistency ledger:
  `artifacts/production-risk-audit/transaction-consistency-ledger-2026-06.md`
- External dependency ledger:
  `artifacts/production-risk-audit/external-dependency-ledger-2026-06.md`
- Release configuration ledger:
  `artifacts/production-risk-audit/release-configuration-ledger-2026-06.md`
- Dedicated source-level state sequencing flow material:
  `artifacts/production-risk-audit/flows/state-sequencing-baofu-order-payment-success-2026-06-14.md`
- Execution cards and release gates added after the seven-issue sync:
  - `artifacts/production-risk-audit/flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md`
  - `artifacts/production-risk-audit/flows/state-sequencing-customer-dine-in-checkout-convergence-2026-06-15.md`
  - `artifacts/production-risk-audit/flows/state-sequencing-customer-takeout-checkout-rehydration-payment-2026-06-15.md`
  - `artifacts/production-risk-audit/flows/state-sequencing-customer-reservation-checkout-addon-noshow-2026-06-15.md`
  - `artifacts/production-risk-audit/flows/idempotency-merchant-order-actions-concurrent-validation-2026-06-15.md`
  - `artifacts/production-risk-audit/flows/release-scheduler-worker-readiness-gate-2026-06-15.md`
- Phase 1 source/config/test-surface recon added for the two highest-priority
  gates:
  - Baofu provider evidence gate now names the exact local fact/application,
    callback, probe, and drift-guard anchors that must be proven before C4 or
    production-readiness claims.
  - Scheduler/worker readiness gate now names the production startup,
    Redis/Asynq, scheduler registry, worker handler, and provider-client
    assertions that a future release smoke must check.
- Focused local validation recorded for those two gates:
  `make check-baofu-contract` passed, and targeted `api`, `logic`, `worker`,
  and `scheduler` Go tests passed after adding `/usr/local/go/bin` to `PATH`.

## Coverage Summary

The reusable codegraph inventory contains 40 `*.slice.md` files and 40 matching
`*.edges.json` files.

All 40 flows are now recorded across the six risk themes:

1. State sequencing
2. Idempotency and retry
3. Authorization boundaries
4. Transaction consistency
5. External dependencies
6. Release configuration

The Baofoo aggregate payment success flow has a dedicated source-level state
sequencing document. The other 39 flows are `slice-ledgered`: documented from
reviewed codegraph slices and evidence anchors, and should be promoted to
dedicated source-level flow material before production code changes.

After the seven-issue code/document sync, two earlier findings are classified
as stale/fixed by current code evidence: rider deposit withdrawal request
idempotency and rider income/Baofu withdrawal create request idempotency. Five
remaining real follow-up areas now have execution cards or release gates. These
cards do not promote the full 40-flow matrix from `slice-ledgered`; they define
the next source-audit and validation targets before production changes.

## Highest-Value Follow-Ups

| Area | Finding | Next Step |
| --- | --- | --- |
| Baofoo provider flows | Positive real callback evidence remains a gap for several Baofu payment/refund/share/withdrawal capabilities. | Phase 1 recon is now in `artifacts/production-risk-audit/flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md`; next implementation should add or run the release-safe evidence runner/runbook and write masked evidence to the Baofu evidence ledger. Production ledger rows must pass `make check-release-readiness-target-evidence evidence=...` and pass the same file to `locallife/scripts/baofu_provider_evidence_gate.sh --release-target-evidence`. |
| Scheduler-dependent convergence | Many flows depend on workers/schedulers being deployed and configured. | Release readiness smoke exists in `locallife/cmd/release_readiness_smoke`, wrapper script `locallife/scripts/release_readiness_smoke.sh`, and `artifacts/production-risk-audit/flows/release-scheduler-worker-readiness-gate-2026-06-15.md`; it now covers `dine-in-checkout-recovery` as part of the scheduler registry, fails target Redis/Asynq readiness when a required queue is paused, and has `make check-release-readiness-target-evidence evidence=...` to reject static/dry-run/template target proof and local alert-template references. Remaining work is target-environment execution with disposable fixture IDs, filled target evidence, and filled alert evidence for recovery failure metrics. |
| Dine-in checkout | Historical note: paid order -> session checkout convergence was ledgered but not source-audited in the original pass; later work source-audited it, fixed customer paid checkout authz drift, added backend paid-open-session recovery, and added Mini Program reload/polling contract proof. | Use `artifacts/production-risk-audit/flows/state-sequencing-customer-dine-in-checkout-convergence-2026-06-15.md` as the rerun checklist before changing dine-in checkout, shared payment result, or recovery behavior. |
| Customer checkout | Takeout/reservation stale draft -> backend rehydration -> payment callback/recovery deserves end-to-end proof. | Takeout now has `check:takeout-checkout-rehydration-payment-contract` for Mini Program stale snapshot rehydration, pricing-error blocking, payment-create failure recovery, payment-result re-entry, wrapper-copy drift, and stable order-create `Idempotency-Key` generation/reuse/rotation/clearing/header propagation. Backend same-order payment-create concurrency is now covered by `TestCreatePartnerPaymentTx_ConcurrentOrderPaymentAllowsSinglePendingPayment` and `TestPaymentOrderServiceCreatePaymentOrder_TxPendingConflictDoesNotCallBaofu`. Backend optional order-create request idempotency is covered by `TestCreateOrderTx_RequestIdempotencyReplayAndConflict`, `TestCreateOrderTx_ConcurrentSameIdempotencyKeyCreatesSingleOrder`, logic replay/conflict tests, and API header propagation. Reservation now has `artifacts/production-risk-audit/flows/state-sequencing-customer-reservation-checkout-addon-noshow-2026-06-15.md` and `check:reservation-checkout-addon-recovery-contract`; `check:customer-checkout-provider-e2e-evidence` rejects template-only/non-pass/placeholder provider-device evidence. Remaining gaps are filled takeout/reservation real provider callback/recovery evidence and target-device/E2E proof. |
| Rider deposit withdrawal | The earlier repeated-POST finding was stale: current code requires `Idempotency-Key`, stores `rider_deposit_withdrawal_requests`, and replays same user/key/hash refund plans. | Keep focused idempotency tests in the pre-change checklist; no new idempotency design is needed from this audit sync. |
| Rider income withdrawal | The earlier no-idempotency finding was stale: current shared Baofu withdrawal create stores key/hash on `baofu_withdrawal_orders` and writes a submitted provider command before async dispatch. | Focus next on real provider callback/funds evidence and ambiguous-create/manual-recovery drills. |
| Merchant order actions | Status actions rely on conditional transitions and readback, not request idempotency keys; Flutter duplicate accept/print coalescing has code/test evidence, while cross-client API-level concurrency remains valuable. | Use `artifacts/production-risk-audit/flows/idempotency-merchant-order-actions-concurrent-validation-2026-06-15.md` before action API or status transaction changes. |

## Seven-Issue Code/Document Sync Check

| # | Issue Category | Sync Result | Remaining Real Issue |
| --- | --- | --- | --- |
| 1 | Baofoo provider flows positive callback evidence | Synchronized. Phase 1 recon now maps the evidence gap to callback handlers, fact/application rows, probe commands, and the drift guard. The wrapper rejects withdrawal-only `manual-reconciliation` and `funds-action` evidence labels for payment, profit-sharing, and refund; the wrapper and local collector also reject malformed ledger date/env/commit context; callback ledger endpoints must match the selected capability's exact LocalLife Baofu webhook path, including withdrawal `funds-action` rows that record a callback ACK, and prefix lookalikes such as `payment-extra` plus old share/withdrawal route names are rejected. Direct collector rendering also rejects `ACK` on query/manual rows. Production ledger rows now require a filled release target evidence file that passes `make check-release-readiness-target-evidence evidence=...` and is supplied to the wrapper with `--release-target-evidence`. `make check-baofu-provider-evidence-gate` now runs the wrapper contract. | Covered by `artifacts/production-risk-audit/flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md`; real positive callback/funds-action evidence remains limited for payment/refund/share/withdrawal provider paths. |
| 2 | Dine-in paid order -> session checkout convergence | Synchronized. Source audit, backend recovery, Mini Program reload/polling contract proof, release evidence gates that reject template-only/non-pass device and alert proof, and Prometheus recovery failure counters are now covered. | Covered by `artifacts/production-risk-audit/flows/state-sequencing-customer-dine-in-checkout-convergence-2026-06-15.md`; still needs actual device/E2E proof and filled target-environment alert evidence before treating the release path as fully drilled. |
| 3 | Customer stale draft -> backend rehydrate -> payment callback/recovery -> visible status | Synchronized with request-idempotency closure for takeout and source-level reservation carding. Takeout Mini Program contract proof now covers stale snapshot rehydration, backend pricing replacement, pricing-error submit blocking, payment-create failure recovery, payment-result re-entry readback, wrapper-copy drift, and stable order-create `Idempotency-Key` generation/reuse/rotation/clearing/header propagation. Backend same-order payment-create concurrency proof now covers the order-lock transaction boundary and no-upstream-call conflict path. Backend optional order-create request idempotency now covers same-key replay, conflicting reuse, and concurrent same-key single-order creation when `Idempotency-Key` is supplied. Reservation checkout/add-on/refund/no-show now has a dedicated source-level card with backend transaction anchors, pre-change validation commands, and `check:reservation-checkout-addon-recovery-contract` for the customer Mini Program recovery contract. `check:customer-checkout-provider-e2e-evidence` now gates filled provider/device evidence files for takeout, reservation, and reservation add-on flows. | Takeout and reservation still need filled real provider callback/recovery E2E evidence before payment/add-on UI changes. |
| 4 | Rider deposit withdrawal request idempotency | Synchronized as stale/fixed. Current code requires `Idempotency-Key`, stores `rider_deposit_withdrawal_requests`, replays same user/key/hash, and rejects conflicting reuse. | No active duplicate-withdrawal design issue found; rerun focused idempotency tests before changing the flow. |
| 5 | Rider income/Baofu withdrawal create idempotency | Synchronized as request-idempotency fixed. Current shared Baofu create stores key/hash on `baofu_withdrawal_orders`, replays same owner/key/request, and dispatches provider create asynchronously from a submitted command. | Remaining risk is provider positive evidence, ambiguous create recovery, and manual reconciliation drills. |
| 6 | Merchant order actions idempotency/concurrency | Synchronized with nuance. Flutter duplicate accept/print coalescing has code/test evidence; backend status actions intentionally rely on conditional transitions and readback. | Covered by `artifacts/production-risk-audit/flows/idempotency-merchant-order-actions-concurrent-validation-2026-06-15.md`; cross-client/API-level concurrent accept/ready/reject validation remains useful before action API changes. |
| 7 | Scheduler-dependent convergence release readiness | Synchronized. The release readiness smoke maps production fail-fast, Redis/Asynq, scheduler registration, worker handlers, provider-client readiness, fixture claimability, and the dine-in recovery alert evidence requirement; Redis/Asynq target checks now fail paused required queues; the wrapper enforces target-mode Redis/provider/fixture checks, positive-integer fixture IDs, and `ENVIRONMENT=production`; direct Go command fixture mode also rejects non-positive IDs before DB access; the default scheduler registry now includes `dine-in-checkout-recovery`; `make check-release-readiness-target-evidence evidence=...` now rejects template-only, static, dry-run, non-production, bad fixture, non-pass target evidence, and local alert-template references by running the Mini Program alert evidence checker for local alert `.md` files. | Covered by `artifacts/production-risk-audit/flows/release-scheduler-worker-readiness-gate-2026-06-15.md`; the remaining release issue is executing the wrapper in the target production environment with disposable fixture IDs, validating the filled target evidence file, and supplying filled alert evidence for repeated recovery failure metrics. |

## What Was Not Done

- No production source files were changed.
- No SQL, migration, generated sqlc, Swagger, mocks, frontend wrappers, or
  Flutter files were changed.
- No Mini Program, Flutter, web, provider, live callback, migration, database,
  or deployment smoke tests were run.
- No production-like Redis/Asynq queue smoke, scheduler registry report,
  worker-handler report, disposable fixture claim/enqueue, or process boot
  readiness command was run.
- No environment schema, provider credential, callback URL, scheduler boot, or
  deployment configuration was validated.
- No `artifacts/codegraph/**/*.edges.json` files were modified. Rider codegraph
  Markdown docs were updated only to synchronize stale withdrawal findings with
  current code.

## How To Continue

For any future implementation or fix:

1. Start from `artifacts/production-risk-audit/flow-inventory-2026-06.md`.
2. Open the relevant theme ledger and the original codegraph slice.
3. If the work matches one of the five remaining real follow-up areas, open the
   corresponding 2026-06-15 execution card first.
4. Promote the target flow to a dedicated source-level document under
   `artifacts/production-risk-audit/flows/`.
5. Trace the current runtime path against Go/SQL/frontend/worker/scheduler
   source before editing production code.
6. Define the smallest validation command set in the flow document.
7. Only then implement production changes if the user explicitly asks for code.

## Validation Commands For This Documentation Pass

```bash
find artifacts/codegraph -type f -name '*.edges.json' -print0 | xargs -0 -n1 jq empty
find artifacts/codegraph -type f -name '*.slice.md' | wc -l
find artifacts/codegraph -type f -name '*.edges.json' | wc -l
test -f artifacts/production-risk-audit/flow-inventory-2026-06.md
test -f artifacts/production-risk-audit/state-sequencing-ledger-2026-06.md
test -f artifacts/production-risk-audit/idempotency-retry-ledger-2026-06.md
test -f artifacts/production-risk-audit/authorization-boundary-ledger-2026-06.md
test -f artifacts/production-risk-audit/transaction-consistency-ledger-2026-06.md
test -f artifacts/production-risk-audit/external-dependency-ledger-2026-06.md
test -f artifacts/production-risk-audit/release-configuration-ledger-2026-06.md
test -f artifacts/production-risk-audit/flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md
test -f artifacts/production-risk-audit/flows/state-sequencing-customer-dine-in-checkout-convergence-2026-06-15.md
test -f artifacts/production-risk-audit/flows/state-sequencing-customer-takeout-checkout-rehydration-payment-2026-06-15.md
test -f artifacts/production-risk-audit/flows/state-sequencing-customer-reservation-checkout-addon-noshow-2026-06-15.md
test -f artifacts/production-risk-audit/flows/idempotency-merchant-order-actions-concurrent-validation-2026-06-15.md
test -f artifacts/production-risk-audit/flows/release-scheduler-worker-readiness-gate-2026-06-15.md
rg 'SS-RIDER-WORKBENCH-LOCATION-040|IR-RIDER-WORKBENCH-LOCATION|AB-RIDER-WORKBENCH-LOCATION|TC-RIDER-WORKBENCH-LOCATION|ED-RIDER-WORKBENCH-LOCATION|RC-RIDER-WORKBENCH-LOCATION' artifacts/production-risk-audit
cd locallife && make check-baofu-contract
cd locallife && PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofu.*Callback|TestHandleBaofu.*Notify' -count=1
cd locallife && PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofu|TestPaymentFactServiceApplyExternalPaymentFactApplication' -count=1
cd locallife && PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'TestBaofuPaymentRecoveryScheduler|TestProcessTaskBaofuProfitSharing|TestBaofuWithdrawal|TestRefundRecovery|TestPaymentFactApplicationSchedulerRunOnce|TestPaymentDomainOutboxScheduler|TestBaofuWithdrawalRecoveryScheduler' -count=1
cd locallife && PATH="/usr/local/go/bin:$PATH" go test ./scheduler -run 'Test.*OrderTimeout|Test.*TakeoutAutoComplete|Test.*MerchantOpenStatus|Test.*DataCleanup' -count=1
```
