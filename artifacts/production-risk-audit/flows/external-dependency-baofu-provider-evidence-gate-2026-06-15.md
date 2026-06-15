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
funds action and how the amount was bounded. If a withdrawal `funds-action`
evidence row records a callback ACK, the wrapper treats that ACK as callback
evidence and requires the endpoint to match `/v1/webhooks/baofu/withdraw`.

## Provider Evidence Runbook

Use this runbook only for a controlled provider evidence run. It is not part of
ordinary CI and must not run hidden funds actions.

Preflight:

1. Name the capability: payment, share, refund, or withdrawal.
2. Check the matching capability row in
   `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md` and run
   `make check-baofu-contract`.
3. Confirm environment and config without printing secrets: Baofu endpoint,
   callback URL, S(10) cert indexes, public key, Redis, and
   `DATA_ENCRYPTION_KEY`.
4. Create or identify the LocalLife business row before provider evidence is
   claimed. Standalone smoke rows do not count.
5. For withdrawal, record the approver, maximum amount, owner, account binding,
   and rollback/monitoring owner before any funds action is attempted.

Execution:

1. Run the provider action or wait for the provider callback in the controlled
   environment.
2. Record masked provider identifiers only: out trade/order/refund/request no,
   trade no, refund id, withdrawal no, or secondary merchant id as applicable.
3. Confirm callback ACK exactly as observed. For query recovery, record the
   query endpoint and terminal upstream state instead of inventing an ACK.
   Callback evidence must use the LocalLife webhook path for the same
   capability: `/v1/webhooks/baofu/payment`, `/v1/webhooks/baofu/share`,
   `/v1/webhooks/baofu/refund`, or `/v1/webhooks/baofu/withdraw`. Withdrawal
   `funds-action` evidence that records `--ledger-ack` is held to the same
   withdrawal webhook endpoint check.
4. Verify local durable rows after the provider event:
   `external_payment_facts`, `external_payment_fact_applications` when the flow
   uses applications, command row, and the relevant business row.
5. Run `scripts/baofu_provider_evidence_gate.sh` with the exact DB row IDs. The
   wrapper runs the contract drift guard and static release readiness preflight
   before the matching read-only collector. Use `--ledger-row` only after all
   runtime context fields are known.

Writeback:

1. Paste the generated row into the matching section of
   `.github/standards/domains/baofu-payment/SANDBOX_EVIDENCE_LEDGER.md`.
2. Label the row honestly as C4, production first-order, sandbox-shape,
   fake-order, provider-error, or funds-action evidence.
3. Keep raw payloads, secrets, full provider identifiers, phone, card, identity,
   certificate, signature, and private key material out of the repository.
4. If any required local row is missing, record a negative evidence row or an
   open gap. Do not promote the capability.

Non-claims:

- A collector pass is not provider reachability.
- A candidate ledger row is not C4 until it is backed by the controlled runtime
  event and written to the ledger with masked identifiers.
- A fake-order `ORDER_NOT_EXIST` or withdrawal missing-serial response is
  transport/error-classification evidence only.
- A withdrawal query result is not approval to execute a withdrawal.

## Phase 1 Evidence Gate Wrapper

Use the wrapper as the preferred local entrypoint after the controlled provider
event has already produced the relevant DB rows:

```bash
PATH="/usr/local/go/bin:$PATH" scripts/baofu_provider_evidence_gate.sh \
  --capability payment \
  --fact-id <external_payment_facts.id> \
  --application-id <external_payment_fact_applications.id> \
  --payment-order-id <payment_orders.id> \
  --ledger-row \
  --evidence-kind callback \
  --ledger-date <yyyy-mm-dd> \
  --ledger-env <sandbox|production|provider-real-transaction-env> \
  --ledger-endpoint <callback-url> \
  --ledger-ack OK \
  --ledger-commit <commit-sha> \
  --ledger-notes <controlled-run-notes>
```

Supported `--capability` values are `payment`, `profit-sharing`, `refund`, and
`withdrawal`. `callback` and `query` evidence kinds are valid for every
capability. `manual-reconciliation` and `funds-action` are withdrawal-only
evidence kinds; the wrapper rejects those labels for payment, profit-sharing,
and refund so a normal query/callback row cannot be promoted under a withdrawal
operations label. Callback evidence requires `--ledger-ack`; query evidence
rejects ACK input so the row cannot invent a callback observation. Callback
ledger endpoints must also match the selected capability's LocalLife webhook
path exactly after URL parsing, so a query endpoint, another Baofu callback
route, or a prefixed fake route such as `payment-extra` cannot be recorded as
positive callback evidence for the wrong capability. Withdrawal
`--evidence-kind funds-action` also requires `--withdrawal-approver`,
`--withdrawal-amount-bound`, and `--withdrawal-monitoring-owner`, and still does
not authorize or execute the withdrawal. Both the wrapper and local collector
reject malformed ledger runtime context: `--ledger-date` must be `yyyy-mm-dd`,
`--ledger-env` must be one of `sandbox`, `production`, or
`provider-real-transaction-env`, and `--ledger-commit` must be a 7-40 character
hex git SHA.

For withdrawal ambiguous-create or timeout recovery, use
`--evidence-kind manual-reconciliation` only after an operator has checked the
provider query or other Baofoo-authoritative source. The wrapper requires
`--manual-recovery-owner` and `--provider-query-result`; those values are added
to ledger notes so manual recovery cannot be recorded as an unexplained success.

```bash
PATH="/usr/local/go/bin:$PATH" scripts/baofu_provider_evidence_gate.sh \
  --capability withdrawal \
  --fact-id <external_payment_facts.id> \
  --withdrawal-order-id <baofu_withdrawal_orders.id> \
  --command-id <external_payment_commands.id> \
  --ledger-row \
  --evidence-kind manual-reconciliation \
  --ledger-date <yyyy-mm-dd> \
  --ledger-env <sandbox|production|provider-real-transaction-env> \
  --ledger-endpoint <withdrawal-query-endpoint> \
  --ledger-commit <commit-sha> \
  --ledger-notes <controlled-manual-recovery-notes> \
  --manual-recovery-owner <owner-or-ticket> \
  --provider-query-result <masked-terminal-query-result>
```

The wrapper has a `--dry-run` mode for release/runbook review. Dry-run prints
the exact commands and performs no DB, provider, or funds-action work.

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
- If a withdrawal manual-reconciliation path is claimed, who owned the manual
  recovery and what masked provider query result backed it?
- Did `scripts/baofu_provider_evidence_gate.sh` run, or was its `--dry-run`
  output reviewed before the controlled evidence run?
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

## Phase 1 Wrapper Validation Run 2026-06-15

Commands run from `locallife/`:

```bash
make check-baofu-provider-evidence-gate
```

Observed result:

- The wrapper contract test printed
  `baofu provider evidence gate wrapper contract passed`.

What this proves:

- The wrapper dry-run includes `make check-baofu-contract`,
  `scripts/release_readiness_smoke.sh --static --format text`, and the matching
  read-only collector command.
- Callback ledger rows are rejected without explicit ACK.
- Callback ledger rows are rejected when the endpoint does not match the
  selected capability's exact LocalLife Baofu webhook path; prefix lookalikes
  such as `payment-extra` are rejected.
- Query evidence does not receive a synthetic ACK.
- Payment, profit-sharing, and refund evidence reject withdrawal-only
  `manual-reconciliation` and `funds-action` labels.
- Ledger rows reject malformed date, environment, and commit context before a
  candidate evidence row can be rendered.
- Withdrawal funds-action evidence is rejected unless approval, amount bound,
  and monitoring owner are supplied.
- Withdrawal manual-reconciliation evidence is rejected unless a manual recovery
  owner and masked provider query result are supplied.

What this still does not prove:

- No real Baofoo provider callback, query, or funds action was executed by the
  wrapper test.
- No new C4 evidence row was added to `SANDBOX_EVIDENCE_LEDGER.md`.

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
