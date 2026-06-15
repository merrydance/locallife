# Release Gate: Scheduler And Worker Readiness

Date: 2026-06-15
Risk theme: release configuration
Risk class: G3 where schedulers/workers converge payment, refund, withdrawal, delivery, claims, and order state
Status: execution card, documentation-only
Phase 1 status: runtime registration/config/test-surface recon completed; focused local validation passed; no production code changed

## Decision

Add a standard scheduler/worker readiness gate before production releases that
touch callback/provider flows, payment/refund/withdrawal recovery, order
timeouts, print processing, or merchant open-state automation.

This card does not implement the smoke. It defines the release gate so the next
code/config task has a concrete target.

## Runtime Scheduler/Worker Surface

`locallife/main.go` registers or starts these convergence paths:

- Redis task processor and task distributor:
  `locallife/main.go:444` through `:517`.
- Core schedulers:
  `auto-tag`, `session-cleanup`, `payment-recovery`,
  `wechat-notification-recovery`.
- Payment convergence:
  `payment-fact-application`, `payment-domain-outbox`,
  `baofu-payment-recovery`, `refund-recovery`.
- Baofu account/withdrawal/report convergence:
  `baofu-account-opening-recovery`, `baofu-withdrawal-recovery`,
  `baofu-merchant-report-recovery`.
- Claims and recovery:
  `claim-payout-recovery`, `claim-behavior-action-recovery`,
  `claim-recovery`.
- Order/delivery/lifecycle:
  `order-timeout`, `takeout-auto-complete`, `data-cleanup`.
- Printer status:
  `cloud-printer-status-poll` when supported.
- Merchant availability:
  `merchant-open-status` is registered in `runGinServer`.

Evidence: `locallife/main.go:97` through `:112`,
`locallife/main.go:160` through `:179`,
`locallife/main.go:218` through `:326`,
`locallife/main.go:389` through `:417`,
`locallife/main.go:444` through `:517`, and
`locallife/main.go:551` through `:553`.

The worker task processor registers the corresponding Asynq task handlers at
`locallife/worker/processor.go:318` through `:366`.

## Current Gap

The repository has backend CI, safety scripts, Baofu contract drift checks, and
generated-artifact checks, but no standard release smoke that proves:

- the task processor starts with the production-like Redis config;
- required queues are reachable;
- scheduler registration matches the expected production feature set;
- provider-dependent schedulers fail closed or explicitly warn when disabled;
- a sample pending application/outbox/recovery row can be claimed or enqueued;
- merchant-open-status and order-timeout schedulers are live in the deployed
  process shape.

Existing files checked:

- `.github/workflows/*.yml`
- `locallife/scripts/check_baofu_contract_drift.sh`
- `locallife/scripts/check_generated.sh`
- `locallife/scripts/test_safety.sh`
- `locallife/scripts/traffic_watch.sh`

## Phase 1 Source Recon 2026-06-15

Confirmed local readiness:

- Production startup fails fast when `REDIS_ADDRESS` is missing, avoiding the
  financial-task `NoopTaskDistributor` downgrade in production.
- Production startup validates Baofu runtime config for main-business payments.
- Redis reachability is checked during startup through weather cache
  construction when Redis is configured.
- `runTaskProcessor` constructs a real Redis distributor and starts the Asynq
  task processor when Redis is configured.
- Worker `Start` registers task handlers for payment timeout, order payment
  timeout, refund processing, payment fact application, payment domain outbox,
  Baofu profit sharing, Baofu account opening, Baofu withdrawal fact
  application, Baofu withdrawal command dispatch, OCR, notifications, claims,
  and print tasks.
- Scheduler registration in `main.go` covers payment fact application, payment
  outbox, Baofu payment recovery, Baofu account opening recovery, Baofu
  withdrawal recovery, Baofu merchant report recovery, refund recovery, claim
  recovery, order timeout, takeout auto-complete, data cleanup, and merchant
  open status.

Still not proven by CI/release gates:

- A release command does not report the full scheduler set that started in the
  deployed process.
- No standard smoke enqueues a harmless Asynq probe or validates queue
  reachability with production-like Redis.
- No process-level gate proves `payment-fact-application`,
  `payment-domain-outbox`, `baofu-payment-recovery`,
  `baofu-withdrawal-recovery`, `refund-recovery`, `order-timeout`, and
  `merchant-open-status` are all active in the deployed process shape.
- No gate fails the release when provider-dependent recovery schedulers are
  disabled while Baofu main business is enabled.
- No release smoke proves a pending fixture row can be claimed or enqueued by
  the recovery/application paths without mutating production money state.

## Phase 1 Implementation Target

The next code/config task should add a release-oriented readiness command or
script. It should be safe by default, read-only where possible, and require an
explicit fixture mode before inserting disposable rows.

| Area | Readiness assertion | Current anchor |
| --- | --- | --- |
| Production fail-fast | `ENVIRONMENT=production` cannot boot with missing Redis, missing Baofu runtime config, missing callback URLs, or missing data encryption key. | `locallife/main.go:97` through `:112`; `locallife/main.go:209` through `:216`; `locallife/util/config.go:308` through `:330`. |
| Redis/Asynq | Redis ping succeeds; distributor is `RedisTaskDistributor`, not noop; critical/default queues are reachable by dry-run or harmless probe. | `locallife/main.go:160` through `:179`; `locallife/main.go:444` through `:517`; `locallife/worker/processor.go:318` through `:366`. |
| Payment fact application | Scheduler is registered; `payment:process_fact_application` handler is registered; pending application fixture can be enqueued or reported claimable. | `locallife/main.go:243` through `:247`; `locallife/worker/processor.go:347`. |
| Payment outbox | Scheduler and `payment:process_domain_outbox` handler are present. | `locallife/main.go:248` through `:252`; `locallife/worker/processor.go:348`. |
| Baofu recovery | Aggregate/account/merchant-report clients are configured when Baofu main business is enabled; disabled branches are release failures or explicit approved warnings. | `locallife/main.go:253` through `:296`; `locallife/main.go:402` through `:441`. |
| Withdrawal recovery | `baofu-withdrawal-recovery`, withdrawal command dispatch, and withdrawal fact application are present together. | `locallife/main.go:281` through `:285`; `locallife/worker/processor.go:351` through `:352`. |
| Refund recovery | Direct and Baofu branches match payment mode and do not silently run without provider capability. | `locallife/main.go:298` through `:311`. |
| Order lifecycle | `order-timeout`, `takeout-auto-complete`, and order payment timeout worker handlers are live. | `locallife/main.go:320` through `:321`; `locallife/worker/processor.go:322` through `:324`. |
| Merchant availability | `merchant-open-status` starts inside `runGinServer` with the server publisher. | `locallife/main.go:551` through `:553`. |
| Cleanup/reconciliation | `data-cleanup` starts with a non-nil publisher strategy in the intended environment. | `locallife/main.go:325`. |

## Proposed Readiness Report Shape

The future command should print a concise matrix, not rely on log scraping:

| Row | Status values | Notes |
| --- | --- | --- |
| `config.production_fail_fast` | `pass/fail` | Redact all secrets and provider identifiers. |
| `redis.connection` | `pass/fail` | Include host label only, not password. |
| `asynq.queues` | `pass/fail/warn` | Verify `critical` and `default`. |
| `worker.handlers` | `pass/fail` | Include missing task type names. |
| `scheduler.registry` | `pass/fail` | Include missing scheduler names. |
| `provider.clients` | `pass/fail/warn` | Fail when Baofu main business is enabled but required client is absent. |
| `fixture.claimability` | `pass/fail/skipped` | Skipped unless fixture mode is explicitly enabled. |

Minimum missing task or scheduler names should be literal names from the current
runtime, for example `payment-fact-application`,
`payment-domain-outbox`, `baofu-payment-recovery`,
`baofu-withdrawal-recovery`, `refund-recovery`, `order-timeout`,
`merchant-open-status`, `payment:process_fact_application`,
`baofu:process_profit_sharing`, and
`baofu:process_withdrawal_fact_application`.

## Phase 1 Validation Run 2026-06-15

Commands run from `locallife/`:

```bash
PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'TestBaofuPaymentRecoveryScheduler|TestProcessTaskBaofuProfitSharing|TestBaofuWithdrawal|TestRefundRecovery|TestPaymentFactApplicationSchedulerRunOnce|TestPaymentDomainOutboxScheduler|TestBaofuWithdrawalRecoveryScheduler' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./scheduler -run 'Test.*OrderTimeout|Test.*TakeoutAutoComplete|Test.*MerchantOpenStatus|Test.*DataCleanup' -count=1
```

Observed result:

- Focused `worker` and `scheduler` packages returned `ok`.
- The initial `go test` attempt failed because `go` was not in the default
  shell `PATH`; rerun with `/usr/local/go/bin` prepended succeeded.

What this proves:

- Existing focused unit tests for the checked recovery, outbox, payment fact,
  timeout, auto-complete, merchant-open, and cleanup scheduler patterns are
  currently green.

What this still does not prove:

- No deployed process was started.
- No Redis/Asynq queue reachability smoke was executed.
- No process-level scheduler registry or worker-handler report was generated.
- No disposable fixture row was claimed or enqueued.
- The release readiness smoke described by this card is still not implemented.

## Proposed Release Smoke Contract

The future smoke should be read-only or use disposable fixture rows. It should
report a pass/fail matrix with these rows:

| Area | Minimum smoke assertion |
| --- | --- |
| Redis/Asynq | Redis reachable; task distributor is not noop in production; queues can enqueue a harmless probe or dry-run equivalent. |
| Payment facts | `payment-fact-application` scheduler and worker task type are registered; pending/failed application rows are claimable in a fixture environment. |
| Payment outbox | `payment-domain-outbox` scheduler and task type are registered. |
| Baofu recovery | Aggregate/account/merchant-report clients are configured when Baofu main business is enabled; disabled branches are explicit warnings. |
| Refund recovery | Direct and Baofu refund recovery branches are configured according to payment mode. |
| Withdrawal recovery | Baofu withdrawal recovery scheduler and command/fact workers are present when withdrawal is enabled. |
| Order timeout | `order-timeout` and order-payment timeout workers are present. |
| Merchant open status | `merchant-open-status` scheduler starts with websocket publisher available. |
| Data cleanup | credit expiry, stale print anomaly, stale delivery cleanup, and related cleanup jobs are in the deployed scheduler set. |

## Focused Validation To Run Before Code Changes

From `locallife/`:

```bash
go test ./worker -run 'TestPaymentFactApplicationSchedulerRunOnce|TestPaymentDomainOutboxScheduler|TestBaofuPaymentRecoverySchedulerRunOnce|TestBaofuWithdrawalRecoveryScheduler|TestRefundRecoveryScheduler' -count=1
go test ./scheduler -run 'Test.*OrderTimeout|Test.*TakeoutAutoComplete|Test.*MerchantOpenStatus|Test.*DataCleanup' -count=1
```

Then add a release-oriented smoke script or command that validates process-level
registration and config readiness without mutating production money state.

## Remaining Real Issue

Many audited flows are safe only if callback facts, outbox rows, recovery rows,
timeouts, and cleanup schedulers actually run in the deployed environment.
Current docs and CI do not prove that release readiness. This remains a real
cross-flow release risk.
