# External Dependency Gate: Baofu Provider Positive Evidence

Date: 2026-06-15
Risk theme: external dependencies / release configuration
Risk class: G3 - provider money movement, callbacks, query recovery, funds action
Status: evidence gate in progress
Phase 1 status: source/config/test-surface recon completed; focused local validation passed; local payment, profit-sharing, refund, and withdrawal evidence collectors added for post-callback/query DB rows

## Decision

Do not change Baofu provider semantics until the touched capability has current
official-source traceability and masked positive provider evidence. Parser tests,
shape-only sandbox probes, fake-order errors, or standalone smoke orders are not
enough to claim payment, refund, share, withdrawal, or callback convergence.

## Scope

This card covers the remaining evidence gap shared by:

- `BAOFU-AGGREGATE-PAYMENT`
- `BAOFU-REFUND`
- merchant/rider/operator/platform Baofu withdrawal flows
- Baofu profit-sharing callbacks and query recovery

It is not a replacement for the Baofu domain source matrix. The canonical
contract source remains `.github/standards/domains/baofu-payment/`.

## Current Evidence Boundary

| Capability | Current evidence | Still missing |
| --- | --- | --- |
| Account open/query/balance | Positive account query and balance evidence exists in the sandbox ledger. | Real account-open callback sample remains useful but is not the top blocker for order funds flow. |
| Merchant report/query/bind | Positive sandbox merchant report, query, and APPLET bind evidence exists. | Production channel readiness remains environment-specific. |
| Unified order/order query | Sandbox unified order and order query parse successfully. Baofoo has confirmed sandbox does not prove real payment. | Production first-order or Baofoo real-transaction payment callback persisted against a LocalLife `payment_orders` row. |
| Payment callback | Parser reached local order lookup for a standalone smoke order. | Fact persistence and application from a real LocalLife order callback. |
| Share after pay/query/callback | Fake-order request/query probes are safely classified. | Real paid-order share command, positive share query, and share callback evidence. |
| Refund before share/query/callback | Fake-order probes are safely classified after failure-classification fixes. | Real pre-share refund command, positive refund query, and refund callback evidence. |
| Withdrawal/query/callback | Fake withdrawal query reaches provider and classifies missing serial safely. | Real small-amount or Baofoo-approved funds-action withdrawal, withdrawal query, and withdrawal callback evidence. |

## Evidence Anchors

- Baofu domain README and validation rule:
  `.github/standards/domains/baofu-payment/README.md`.
- Capability grouping and drift traps:
  `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md`.
- Contract source truth and evidence labels:
  `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md`.
- Contract coverage and implementation map:
  `.github/standards/domains/baofu-payment/API_CONTRACT_COVERAGE_AUDIT.md`
  and `.github/standards/domains/baofu-payment/CONTRACT_IMPLEMENTATION_MAP.md`.
- Current sandbox/live evidence ledger:
  `.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`.
- Remaining non-claim areas:
  `CONTRACT_SOURCE_MATRIX.md` records real positive callbacks for
  payment/share/refund/withdrawal as remaining gaps.
- Callback routes:
  `locallife/api/server.go:552` through `locallife/api/server.go:557`.
- Payment callback source-level state audit:
  `artifacts/production-risk-audit/flows/state-sequencing-baofu-order-payment-success-2026-06-14.md`.
- Runtime callback routes:
  `locallife/api/server.go:548` through `locallife/api/server.go:557`.
- Callback fact persistence and enqueue points:
  `locallife/api/baofu_callback.go:230`,
  `locallife/api/baofu_callback.go:241`,
  `locallife/api/baofu_callback.go:254`,
  `locallife/api/baofu_callback.go:363`,
  `locallife/api/baofu_callback.go:374`,
  `locallife/api/baofu_callback.go:387`,
  `locallife/api/baofu_callback.go:500`,
  and `locallife/api/baofu_callback.go:515`.
- Fact application worker:
  `locallife/worker/processor.go:347` and
  `locallife/worker/task_payment_fact_application.go:77`.
- Existing provider probes:
  `locallife/cmd/baofu_unified_order_smoke/main.go`,
  `locallife/cmd/baofu_order_query_smoke/main.go`,
  `locallife/cmd/baofu_independent_probe/main.go`,
  and `locallife/cmd/baofu_error_probe/main.go`.
- Current contract drift guard:
  `locallife/scripts/check_baofu_contract_drift.sh`.

## Phase 1 Source Recon 2026-06-15

This recon keeps the original decision unchanged: contract/parser tests and
sandbox probes are useful, but they are not a substitute for a real callback or
funds-action fact that reaches LocalLife persistence.

Confirmed local readiness:

- `make check-baofu-contract` already exists and guards the known drift classes:
  `bizContent` vs `dataContent`, old endpoints, `SHARING` vs `SHARE`,
  optional `outTradeNo`, `subMchId` vs `sharingMerId`, S(10) certificate
  indexes, official account query version, official withdrawal `tradeTime`,
  and callback enum validation.
- Provider probes exist for unified order, order query, fake share/refund/close,
  fake withdrawal query, account balance, and provider error classification.
- Runtime callback handlers persist facts before business application for Baofu
  payment, share, refund, account-open, and withdrawal callbacks.
- Payment/share/refund callback handlers enqueue payment fact application and
  can fall back to schedulers if enqueue fails.
- Production startup validates Baofu runtime config when main-business payments
  are enabled and production now requires Redis for financial task queues.

Still not proven:

- A Baofoo real payment callback persisted against a LocalLife-created
  `payment_orders` row and produced an `external_payment_fact_applications`
  row that applied successfully.
- A real paid-order `share_after_pay` command, positive `share_query`, and
  `SHARING` callback against local `profit_sharing_orders`.
- A real pre-share refund command, positive `refund_query`, and `REFUND`
  callback against local `refund_orders`.
- A real withdrawal create/query/callback or Baofoo-approved funds-action drill
  against local `baofu_withdrawal_orders`.

Non-claims to preserve:

- Sandbox `unified_order` and `order_query` prove request/response parsing only
  because Baofoo confirmed sandbox does not support real payment.
- Fake share/refund/withdraw probes prove transport and failure classification
  only.
- Standalone smoke orders do not prove LocalLife business convergence unless the
  callback links to local durable rows and application rows.

## Phase 1 Implementation Target

Add release-safe evidence runners or runbooks that produce masked evidence
bundles per capability. Keep provider-touching and funds-action runs separate
from ordinary CI.

| Step | Required output | Current source anchor |
| --- | --- | --- |
| Contract preflight | `make check-baofu-contract` passes and the touched capability row is named. | `locallife/scripts/check_baofu_contract_drift.sh` and `CONTRACT_SOURCE_MATRIX.md`. |
| Config preflight | Baofu env, callback URLs, S(10) serials, public key, Redis, and `DATA_ENCRYPTION_KEY` are validated without printing secrets. | `locallife/util/config.go:241` through `:330`; `locallife/main.go:97` through `:112`. |
| Local fact fixture | A LocalLife-created `payment_orders` row exists before provider callback evidence is claimed. | `locallife/api/baofu_callback.go:230` through `:262`. |
| Callback persistence proof | Evidence references fact id, application id, business row id, callback type, terminal status, and ACK, with provider identifiers masked. | `external_payment_facts`, `external_payment_fact_applications`, `payment_orders`, `profit_sharing_orders`, `refund_orders`, `baofu_withdrawal_orders`. |
| Query recovery proof | Missing-callback recovery records a query fact and applies the same terminal business state. | `locallife/worker/baofu_payment_recovery_scheduler.go`, `locallife/worker/refund_recovery_scheduler.go`, and Baofu withdrawal recovery files. |
| Evidence writeback | `SANDBOX_EVIDENCE_LEDGER.md` receives a masked row and explicitly states whether the run is C4, sandbox-shape, fake-order, or funds-action evidence. | `.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`. |

Implemented local collector:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_payment_evidence \
  -fact-id <external_payment_facts.id> \
  -application-id <external_payment_fact_applications.id> \
  -payment-order-id <payment_orders.id> \
  -profit-sharing-order-id <optional profit_sharing_orders.id>
```

The command is read-only and does not call Baofoo. It loads the local fact,
application, payment order, and optional profit-sharing bill rows, then emits a
masked JSON summary. It fails when the local convergence evidence is incomplete,
for example non-callback/query/manual fact source, non-success terminal status,
fact not terminalized, application not applied, payment order not paid/processed,
non-Baofu channel, missing business object link, missing amount, or amount
mismatch. This proves local DB evidence only after a real callback/query/recovery
event has already created the rows.

To generate an explicit candidate row for
`.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`, rerun the
same command with `-ledger-row` and supply the observed runtime context rather
than inferring it from DB rows:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_payment_evidence \
  -fact-id <external_payment_facts.id> \
  -application-id <external_payment_fact_applications.id> \
  -payment-order-id <payment_orders.id> \
  -profit-sharing-order-id <optional profit_sharing_orders.id> \
  -ledger-row \
  -ledger-date <yyyy-mm-dd> \
  -ledger-env <sandbox|production|provider-real-transaction-env> \
  -ledger-endpoint <callback-url-or-order-query-endpoint> \
  -ledger-commit <commit-sha> \
  -ledger-notes <controlled-run-notes> \
  -ledger-ack <OK-for-callback-evidence>
```

The ledger row renderer rejects failing summaries and rejects callback evidence
without an explicit observed ACK.

Implemented local profit-sharing collector:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_profit_sharing_evidence \
  -fact-id <external_payment_facts.id> \
  -application-id <external_payment_fact_applications.id> \
  -profit-sharing-order-id <profit_sharing_orders.id> \
  -command-id <optional external_payment_commands.id>
```

The command is read-only and does not call Baofoo. It loads the local share
fact, application, profit-sharing order, and create-share command proof. When
`-command-id` is omitted it attempts a read-only command lookup by the
Baofu profit-sharing external object key (`profit_sharing_orders.out_order_no`).
It emits a masked JSON summary and fails incomplete convergence evidence such as
non-callback/query/manual fact source, non-success terminal status,
non-terminalized fact, unapplied application, unfinished profit-sharing order,
missing `finished_at`, missing or mismatched out-order/external object key,
mismatched amount, missing command proof, or a command row that is not accepted.

This proves local share DB convergence only after a real `SHARING` callback,
positive `share_query`, or manual recovery fact has already created the rows. It
does not prove provider reachability, callback ACK, or C4 status by itself.

To generate an explicit candidate row for
`.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`, rerun the
same command with `-ledger-row` and supply the observed runtime context rather
than inferring it from DB rows:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_profit_sharing_evidence \
  -fact-id <external_payment_facts.id> \
  -application-id <external_payment_fact_applications.id> \
  -profit-sharing-order-id <profit_sharing_orders.id> \
  -command-id <optional external_payment_commands.id> \
  -ledger-row \
  -ledger-date <yyyy-mm-dd> \
  -ledger-env <sandbox|production|provider-real-transaction-env> \
  -ledger-endpoint <callback-url-or-share-query-endpoint> \
  -ledger-commit <commit-sha> \
  -ledger-notes <controlled-run-notes> \
  -ledger-ack <OK-for-callback-evidence>
```

The ledger row renderer rejects failing summaries and rejects share callback
evidence without an explicit observed ACK.

Implemented local refund collector:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_refund_evidence \
  -fact-id <external_payment_facts.id> \
  -application-id <external_payment_fact_applications.id> \
  -refund-order-id <refund_orders.id> \
  -payment-order-id <payment_orders.id> \
  -command-id <optional external_payment_commands.id>
```

The command is read-only and does not call Baofoo. It loads the local refund
fact, application, refund order, payment order, and create-refund command proof.
When `-command-id` is omitted it attempts a read-only command lookup by the
Baofu refund external object key (`refund_orders.out_refund_no`). It emits a
masked JSON summary and fails incomplete convergence evidence such as
non-callback/query/manual fact source, non-success terminal status,
non-terminalized fact, unapplied application, refund order not `success`, missing
`refunded_at`, missing or mismatched `out_refund_no`/external object key,
mismatched amount, non-Baofu payment order, unsupported order/reservation owner,
missing command proof, or a command row that is not accepted.

This proves local successful refund DB convergence only after a real `REFUND`
callback, positive `refund_query`, or recovery fact has already created the rows.
It does not prove provider reachability, callback ACK, or C4 status by itself.
Closed/failed refund evidence remains a separate negative-evidence use case and
is intentionally not treated as a passing positive refund proof here.

To generate an explicit candidate row for
`.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`, rerun the
same command with `-ledger-row` and supply the observed runtime context rather
than inferring it from DB rows:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_refund_evidence \
  -fact-id <external_payment_facts.id> \
  -application-id <external_payment_fact_applications.id> \
  -refund-order-id <refund_orders.id> \
  -payment-order-id <payment_orders.id> \
  -command-id <optional external_payment_commands.id> \
  -ledger-row \
  -ledger-date <yyyy-mm-dd> \
  -ledger-env <sandbox|production|provider-real-transaction-env> \
  -ledger-endpoint <callback-url-or-refund-query-endpoint> \
  -ledger-commit <commit-sha> \
  -ledger-notes <controlled-run-notes> \
  -ledger-ack <OK-for-callback-evidence>
```

The ledger row renderer rejects failing summaries and rejects refund callback
evidence without an explicit observed ACK.

Implemented local withdrawal collector:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_withdrawal_evidence \
  -fact-id <external_payment_facts.id> \
  -withdrawal-order-id <baofu_withdrawal_orders.id> \
  -command-id <optional external_payment_commands.id>
```

The command is read-only and does not call Baofoo, enqueue tasks, or execute a
funds action. It loads the local withdrawal fact, `baofu_withdrawal_orders` row,
and create-withdraw command proof. When `-command-id` is omitted it attempts a
read-only command lookup by the Baofu withdrawal external object key
(`baofu_withdrawal_orders.out_request_no`). It emits a masked JSON summary and
fails incomplete convergence evidence such as non-callback/query/manual fact
source, non-success terminal status, non-terminal fact, unsupported withdrawal
owner, mismatched business owner, missing or mismatched `out_request_no`,
missing `baofu_withdraw_no`, mismatched amount, withdrawal order not
`succeeded`, missing `finished_at`, missing command proof, or a command row that
is not accepted.

Unlike payment, profit sharing, and refund, the current withdrawal application
worker updates `baofu_withdrawal_orders` directly and does not create an
`external_payment_fact_applications` row. Withdrawal evidence therefore proves
local success convergence from fact + withdrawal order + accepted command rows.
It does not prove provider reachability, callback ACK, query recovery, C4
status, or authorization to perform a live withdrawal by itself. The funds-action
approval and bounded amount must still be recorded separately before any
withdrawal evidence can be claimed as a real provider run.

To generate an explicit candidate row for
`.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`, rerun the
same command with `-ledger-row` and supply the observed runtime context rather
than inferring it from DB rows:

```bash
PATH="/usr/local/go/bin:$PATH" go run ./cmd/baofu_withdrawal_evidence \
  -fact-id <external_payment_facts.id> \
  -withdrawal-order-id <baofu_withdrawal_orders.id> \
  -command-id <optional external_payment_commands.id> \
  -ledger-row \
  -ledger-date <yyyy-mm-dd> \
  -ledger-env <sandbox|production|provider-real-transaction-env> \
  -ledger-endpoint <callback-url-or-withdrawal-query-endpoint> \
  -ledger-commit <commit-sha> \
  -ledger-notes <controlled-run-notes-including-funds-action-approval> \
  -ledger-ack <OK-for-callback-evidence>
```

The ledger row renderer rejects failing summaries and rejects withdrawal
callback evidence without an explicit observed ACK. It still does not authorize
or execute a withdrawal; the operator must separately record who approved the
funds action and how the amount was bounded.

## Phase 1 Release Gate Checklist

Before any Baofu-affecting release, answer every item below in the change
handoff.

- Which exact capability group is touched: account, merchant report, aggregate
  payment, share, refund, withdrawal, or callback?
- Which official source rows and field matrix rows were checked?
- Which local DTO/parser/client/service tests were run?
- Did `make check-baofu-contract` run after the change?
- Is the evidence being claimed C4, sandbox-shape, fake-order, or provider
  error classification?
- If a callback path is claimed, which local fact row and application row prove
  persistence and application?
- If a withdrawal path is claimed, who approved the funds action and how was
  the amount bounded?
- Which provider identifiers were masked in the evidence ledger?

## Phase 1 Validation Run 2026-06-15

Commands run from `locallife/`:

```bash
make check-baofu-contract
PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestBaofu.*Callback|TestHandleBaofu.*Notify' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'TestBaofu|TestPaymentFactServiceApplyExternalPaymentFactApplication' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'TestBaofuPaymentRecoveryScheduler|TestProcessTaskBaofuProfitSharing|TestBaofuWithdrawal|TestRefundRecovery|TestPaymentFactApplicationSchedulerRunOnce|TestPaymentDomainOutboxScheduler|TestBaofuWithdrawalRecoveryScheduler' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./internal/baofuevidence ./cmd/baofu_payment_evidence ./cmd/baofu_profit_sharing_evidence ./cmd/baofu_refund_evidence ./cmd/baofu_withdrawal_evidence -count=1
PATH="/usr/local/go/bin:$PATH" go build -o /tmp/baofu_payment_evidence ./cmd/baofu_payment_evidence
PATH="/usr/local/go/bin:$PATH" go build -o /tmp/baofu_profit_sharing_evidence ./cmd/baofu_profit_sharing_evidence
PATH="/usr/local/go/bin:$PATH" go build -o /tmp/baofu_refund_evidence ./cmd/baofu_refund_evidence
PATH="/usr/local/go/bin:$PATH" go build -o /tmp/baofu_withdrawal_evidence ./cmd/baofu_withdrawal_evidence
```

Observed result:

- `make check-baofu-contract` printed `baofu contract drift guard passed`.
- Focused `api`, `logic`, and `worker` packages returned `ok`.
- The local Baofu evidence collector package and the payment/profit-sharing/
  refund/withdrawal command packages returned `ok`; all four command builds
  succeeded.
- The initial `go test` attempt failed because `go` was not in the default
  shell `PATH`; rerun with `/usr/local/go/bin` prepended succeeded.

What this proves:

- Existing drift guards and focused local callback/fact/recovery tests are
  currently green for the checked package patterns.
- Read-only local evidence commands can now convert real payment,
  profit-sharing, refund, and withdrawal callback/query persistence rows into
  masked JSON summaries and fail incomplete local convergence evidence.
- With explicit operator-supplied runtime context, the command can render a
  candidate Payment Callback or Payment Query ledger row without inventing env,
  endpoint, ACK, or commit facts.

What this still does not prove:

- No real provider callback, live callback URL, production first-order,
  positive share/refund, withdrawal funds action, or masked evidence-ledger
  writeback was executed in this phase.
- No C4 or production-readiness claim is made by this validation run.
- The local collectors do not discover all rows automatically and do not prove
  provider reachability; operators must supply the DB row IDs from a controlled
  callback/query/recovery run. The profit-sharing, refund, and withdrawal
  collectors can only infer command proof by existing external object keys.
- The local collector currently does not read `payment_domain_outbox`
  automatically; outbox proof still needs either an explicit future query or
  manual evidence from the controlled run.

## Release Gate

Before a release that changes Baofu provider semantics, callback parsing,
query recovery, command dispatch, or local terminal-state mapping:

1. Identify the exact capability row in
   `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md`.
2. Confirm whether the existing evidence is `DOC_CONFIRMED`,
   `BAOFOO_CONFIRMED`, `SANDBOX_COMPATIBILITY`, or fake-order/transport-only.
3. If claiming C4 or production readiness, append masked request/response,
   callback, fact-persistence, and query-recovery evidence to
   `SANDBOX_EVIDENCE_LEDGER.md`.
4. Prove local persistence, not only provider reachability:
   `external_payment_facts`, `external_payment_fact_applications`, command rows,
   and the relevant business row must be referenced.
5. Keep funds-action tests explicitly opt-in. Withdrawal evidence must not run
   unless the environment is funded and the run is explicitly approved.

## Focused Validation To Run Before Code Changes

From `locallife/`:

```bash
make check-baofu-contract
go test ./api -run 'TestBaofu.*Callback|TestHandleBaofu.*Notify' -count=1
go test ./logic -run 'TestBaofu|TestPaymentFactServiceApplyExternalPaymentFactApplication' -count=1
go test ./worker -run 'TestBaofuPaymentRecoveryScheduler|TestProcessTaskBaofuProfitSharing|TestBaofuWithdrawal|TestRefundRecovery' -count=1
go test ./internal/baofuevidence ./cmd/baofu_payment_evidence ./cmd/baofu_profit_sharing_evidence ./cmd/baofu_refund_evidence ./cmd/baofu_withdrawal_evidence -count=1
```

Use narrower patterns if the target capability is smaller. Runtime provider
evidence still requires a controlled sandbox/live-like run and cannot be
inferred from these tests alone.

## Remaining Real Issue

This is still a real release risk: several Baofu paths are contract-tested and
shape-tested but not yet proven by positive real callback/funds-action evidence
against local durable facts and business rows.
