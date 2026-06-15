# Release Gate: Scheduler And Worker Readiness

Date: 2026-06-15
Risk theme: release configuration
Risk class: G3 where schedulers/workers converge payment, refund, withdrawal, delivery, claims, and order state
Status: release readiness smoke implemented; release-environment execution still required
Phase 1 status: static registration, config, Redis/Asynq, provider-client, rollback-only fixture claimability surfaces, and release wrapper implemented; focused local validation passed

## Decision

Run the standard scheduler/worker readiness gate before production releases that
touch callback/provider flows, payment/refund/withdrawal recovery, order
timeouts, print processing, or merchant open-state automation.

This card records the gate and the Phase 1 implementation surface. The command
is safe by default, with Redis/provider/fixture checks enabled only through
explicit flags.

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
  `order-timeout`, `takeout-auto-complete`,
  `dine-in-checkout-recovery`, `data-cleanup`.
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

## Original Gap

The repository has backend CI, safety scripts, Baofu contract drift checks, and
generated-artifact checks, but previously had no standard release smoke that
proved:

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
  recovery, order timeout, takeout auto-complete, dine-in checkout recovery,
  data cleanup, and merchant open status.

Still not proven by CI/release gates:

- A release command does not report the full scheduler set that started in the
  deployed process.
- No standard smoke enqueues a harmless Asynq probe or validates queue
  reachability with production-like Redis.
- No process-level gate proves `payment-fact-application`,
  `payment-domain-outbox`, `baofu-payment-recovery`,
  `baofu-withdrawal-recovery`, `refund-recovery`, `order-timeout`,
  `dine-in-checkout-recovery`, and `merchant-open-status` are all active in
  the deployed process shape.
- No gate fails the release when provider-dependent recovery schedulers are
  disabled while Baofu main business is enabled.
- No release smoke proves a pending fixture row can be claimed or enqueued by
  the recovery/application paths without mutating production money state.

## Phase 1 Implemented Target

The release-oriented readiness command is safe by default, read-only where
possible, and requires explicit fixture mode plus known disposable row IDs
before exercising DB claimability.

| Area | Readiness assertion | Current anchor |
| --- | --- | --- |
| Production fail-fast | `ENVIRONMENT=production` cannot boot with missing Redis, missing Baofu runtime config, missing callback URLs, or missing data encryption key. | `locallife/main.go:97` through `:112`; `locallife/main.go:209` through `:216`; `locallife/util/config.go:308` through `:330`. |
| Redis/Asynq | Redis ping succeeds; distributor is `RedisTaskDistributor`, not noop; critical/default queues are reachable by dry-run or harmless probe. | `locallife/main.go:160` through `:179`; `locallife/main.go:444` through `:517`; `locallife/worker/processor.go:318` through `:366`. |
| Payment fact application | Scheduler is registered; `payment:process_fact_application` handler is registered; pending application fixture can be enqueued or reported claimable. | `locallife/main.go:243` through `:247`; `locallife/worker/processor.go:347`. |
| Payment outbox | Scheduler and `payment:process_domain_outbox` handler are present. | `locallife/main.go:248` through `:252`; `locallife/worker/processor.go:348`. |
| Baofu recovery | Aggregate/account/merchant-report clients are configured when Baofu main business is enabled; disabled branches are release failures or explicit approved warnings. | `locallife/main.go:253` through `:296`; `locallife/main.go:402` through `:441`. |
| Withdrawal recovery | `baofu-withdrawal-recovery`, withdrawal command dispatch, and withdrawal fact application are present together. | `locallife/main.go:281` through `:285`; `locallife/worker/processor.go:351` through `:352`. |
| Refund recovery | Direct and Baofu branches match payment mode and do not silently run without provider capability. | `locallife/main.go:298` through `:311`. |
| Order lifecycle | `order-timeout`, `takeout-auto-complete`, `dine-in-checkout-recovery`, and order payment timeout worker handlers are live. | `locallife/main.go:320` through `:322`; `locallife/worker/processor.go:322` through `:324`. |
| Merchant availability | `merchant-open-status` starts inside `runGinServer` with the server publisher. | `locallife/main.go:551` through `:553`. |
| Cleanup/reconciliation | `data-cleanup` starts with a non-nil publisher strategy in the intended environment. | `locallife/main.go:325`. |

## Phase 1 Static Readiness Smoke

Implemented command:

```bash
PATH="/usr/local/go/bin:$PATH" scripts/release_readiness_smoke.sh --static --format text
PATH="/usr/local/go/bin:$PATH" PAYMENT_FACT_APPLICATION_FIXTURE_ID=<id> PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=<id> scripts/release_readiness_smoke.sh --target --format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -format json
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -include-config -format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -include-config -include-redis -format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -include-config -include-provider-clients -format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -include-config -include-fixture-claimability -payment-fact-application-fixture-id <id> -payment-domain-outbox-fixture-id <id> -format text
```

The command parses `main.go`, `worker/processor.go`, and `worker/*.go` with Go
AST only. It does not boot the service, connect to Redis/Postgres, enqueue
tasks, or call any provider. With `-include-config`, it also loads the same
`util.Config` source used by startup and checks production fail-fast readiness
without printing secrets. With `-include-redis`, it pings Redis and reads
Asynq queue stats for the `critical` and `default` queues without enqueueing
tasks. With `-include-provider-clients`, it constructs Baofu root, aggregate,
account, and merchant-report clients without making provider requests. With
`-include-fixture-claimability`, it opens a rollback-only DB transaction and
claims explicit disposable fixture rows by ID, proving claim SQL reaches
`processing` without committing the state change. A non-pass report exits
non-zero.

The wrapper `scripts/release_readiness_smoke.sh` is the preferred release
entrypoint. `--static` keeps the side-effect-free source report. `--target`
requires `PAYMENT_FACT_APPLICATION_FIXTURE_ID` and
`PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID`, then always runs config, Redis/Asynq,
Baofu provider-client, and rollback-only fixture claimability checks together.
The wrapper rejects missing, zero, or non-numeric fixture IDs before invoking
the Go smoke command. The Go command also rejects non-positive fixture IDs
before opening a DB connection when run directly with
`-include-fixture-claimability`.

Current rows:

| Row group | Status values | Notes |
| --- | --- | --- |
| `scheduler:<name>` | `pass/fail` | Confirms required scheduler names are registered through production startup source. |
| `worker:<task_type>` | `pass/fail` | Confirms required task constants match expected queue names and are registered to expected processor handlers. |
| `config:production_allowed_origins` | `pass/fail` | Confirms production CORS allowlist is non-empty and not wildcard when `-include-config` is used. |
| `config:production_redis_address` | `pass/fail` | Confirms production has a Redis address when `-include-config` is used. |
| `config:production_data_encryption_key` | `pass/fail` | Confirms production has `DATA_ENCRYPTION_KEY` when `-include-config` is used. |
| `config:production_payment_runtime` | `pass/fail` | Reuses `ValidateBaofuConfig` and confirms Baofu main-business runtime readiness when `-include-config` is used. |
| `redis:connection` | `pass/fail` | Confirms Redis ping succeeds when `-include-redis` is used. |
| `asynq:queue:<name>` | `pass/fail` | Confirms Asynq inspector can read queue stats or an empty queue namespace without enqueueing tasks. |
| `provider:baofu:<client>` | `pass/fail` | Confirms Baofu provider clients can be locally constructed from config without provider requests. |
| `fixture:payment_fact_application` | `pass/fail` | Confirms an explicit `external_payment_fact_applications` fixture row can be claimed inside a rollback-only transaction. |
| `fixture:payment_domain_outbox` | `pass/fail` | Confirms an explicit `payment_domain_outbox` fixture row can be claimed inside a rollback-only transaction. |

Minimum missing task or scheduler names should be literal names from the current
runtime, for example `payment-fact-application`,
`payment-domain-outbox`, `baofu-payment-recovery`,
`baofu-withdrawal-recovery`, `refund-recovery`, `order-timeout`,
`dine-in-checkout-recovery`, `merchant-open-status`,
`payment:process_fact_application`,
`baofu:process_profit_sharing`, and
`baofu:process_withdrawal_fact_application`.

Fixture claimability mode requires the release operator to supply known
disposable fixture IDs. It does not auto-select arbitrary pending production
money rows.

## Phase 1 Implementation Run 2026-06-15

Commands run from `locallife/`:

```bash
PATH="/usr/local/go/bin:$PATH" go test ./internal/releasereadiness -run TestCheck -count=1
PATH="/usr/local/go/bin:$PATH" go test ./internal/releasereadiness ./cmd/release_readiness_smoke -count=1
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -format json
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -include-config -format text
PATH="/usr/local/go/bin:$PATH" go run ./cmd/release_readiness_smoke -include-config -include-provider-clients -format text
PATH="/usr/local/go/bin:$PATH" go build ./cmd/release_readiness_smoke
PATH="/usr/local/go/bin:$PATH" go test ./internal/releasereadiness -run TestCheckRedisAsynqReadiness -count=1
PATH="/usr/local/go/bin:$PATH" go test ./internal/releasereadiness -run TestCheckBaofuProviderClientReadiness -count=1
PATH="/usr/local/go/bin:$PATH" go test ./internal/releasereadiness -run TestCheckFixtureClaimability -count=1
PATH="/usr/local/go/bin:$PATH" bash scripts/test_release_readiness_smoke.sh
PATH="/usr/local/go/bin:$PATH" bash scripts/release_readiness_smoke.sh --static --format text
PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'TestBaofuPaymentRecoveryScheduler|TestPaymentFactApplicationSchedulerRunOnce|TestPaymentDomainOutboxScheduler|TestBaofuWithdrawalRecoveryScheduler|TestRefundRecoveryScheduler' -count=1
PATH="/usr/local/go/bin:$PATH" go test ./scheduler -run 'Test.*OrderTimeout|Test.*TakeoutAutoComplete|Test.*MerchantOpenStatus|Test.*DataCleanup' -count=1
```

Observed result:

- The release readiness package tests returned `ok`.
- The command package builds and runs.
- The generated text and JSON reports returned `status=pass` for the current
  static scheduler and worker registration set.
- The explicit config mode returned `status=pass` in the local non-production
  environment with production-only rows marked as skipped.
- Redis/Asynq readiness is covered by a focused miniredis-backed unit test; the
  local command run did not use `-include-redis` because the local config does
  not point at a release Redis instance.
- Baofu provider-client readiness is covered by a focused unit test; the local
  command can construct clients when release-like Baofu config is supplied, but
  does not call Baofoo.
- Fixture claimability is covered by focused unit tests for explicit fixture
  IDs, successful claim returns, and unclaimable rows.
- The wrapper contract test proves target mode includes config, Redis/Asynq,
  Baofu provider-client, and rollback-only fixture checks and refuses to run
  without positive-integer fixture IDs.
- The Go command test proves direct `-include-fixture-claimability` runs reject
  non-positive fixture IDs before DB claimability checks.
- Focused worker and scheduler package tests returned `ok`.

What this proves:

- A release operator can now run a side-effect-free static smoke that fails
  when required scheduler registrations or worker handler/task-type bindings
  drift out of production startup source.
- With `-include-config`, a release operator can also fail closed on the same
  production CORS, Redis address, data-encryption-key, and Baofu runtime config
  prerequisites checked during startup, without printing secret values.
- With `-include-redis`, a release operator can prove Redis ping and read-only
  Asynq queue namespace access for `critical` and `default` without enqueueing
  tasks.
- With `-include-provider-clients`, a release operator can prove Baofu root,
  aggregate, account, and merchant-report clients can be locally constructed
  from loaded config without calling Baofoo.
- With `-include-fixture-claimability`, a release operator can prove explicit
  disposable payment fact application and payment-domain outbox fixture rows
  are claimable inside a rollback-only DB transaction.
- The static report covers payment fact application, payment outbox, Baofu
  payment/account/withdrawal/merchant-report recovery, refund recovery, order
  timeout, takeout auto-complete, dine-in checkout recovery, merchant-open
  status, data cleanup, claim recovery, OCR, notification, print, and risk task
  handler registration.

What this still does not prove:

- No deployed process was started.
- Redis/Asynq reachability was unit-tested with miniredis; it was not executed
  against a deployed Redis instance in this implementation run.
- No provider network request was made.
- No release-environment disposable fixture IDs were supplied in this local
  implementation run, so the DB fixture mode was not executed against a real
  deployed database.

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
- No disposable fixture row was claimed or enqueued.
- Before the implementation run above, no process-level scheduler registry or
  worker-handler report existed; that static report now exists.

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
| Order timeout | `order-timeout`, `dine-in-checkout-recovery`, and order-payment timeout workers are present. |
| Merchant open status | `merchant-open-status` scheduler starts with websocket publisher available. |
| Data cleanup | credit expiry, stale print anomaly, stale delivery cleanup, and related cleanup jobs are in the deployed scheduler set. |

## Focused Validation And Release Commands

From `locallife/`:

```bash
go test ./worker -run 'TestPaymentFactApplicationSchedulerRunOnce|TestPaymentDomainOutboxScheduler|TestBaofuPaymentRecoverySchedulerRunOnce|TestBaofuWithdrawalRecoveryScheduler|TestRefundRecoveryScheduler' -count=1
go test ./scheduler -run 'Test.*OrderTimeout|Test.*TakeoutAutoComplete|Test.*MerchantOpenStatus|Test.*DataCleanup' -count=1
```

Also run the static release smoke:

```bash
scripts/release_readiness_smoke.sh --static --format text
PAYMENT_FACT_APPLICATION_FIXTURE_ID=<id> PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=<id> scripts/release_readiness_smoke.sh --target --format text
go run ./cmd/release_readiness_smoke -format text
go run ./cmd/release_readiness_smoke -include-config -format text
go run ./cmd/release_readiness_smoke -include-config -include-redis -format text
go run ./cmd/release_readiness_smoke -include-config -include-provider-clients -format text
go run ./cmd/release_readiness_smoke -include-config -include-fixture-claimability -payment-fact-application-fixture-id <id> -payment-domain-outbox-fixture-id <id> -format text
```

Fixture IDs must point at disposable release-smoke rows prepared by the release
operator and must be positive integers; the command does not create rows or
select arbitrary pending production money records.

## Remaining Real Issue

Many audited flows are safe only if callback facts, outbox rows, recovery rows,
timeouts, and cleanup schedulers actually run in the deployed environment.
The static/config/Redis/provider-client smoke reduces source-level
registration, production fail-fast configuration, queue reachability, and local
provider-client construction drift risk. The fixture claimability mode adds a
rollback-only DB proof for explicit disposable rows, but this implementation
run did not execute it against a deployed release database. The remaining
release risk is operational: every release still needs prepared fixture IDs and
an actual smoke run in the target environment before claiming deployed runtime
readiness.

For dine-in checkout recovery specifically, the deployed Prometheus or
equivalent monitor must have a filled target-environment evidence file that
passes:

```bash
cd weapp
npm run check:dine-in-recovery-alert-evidence -- ../artifacts/production-risk-audit/flows/dine-in-checkout-recovery-alert-evidence-YYYY-MM-DD.md
```

That evidence must prove repeated increases of
`dine_in_checkout_recovery_scans_total{result="list_error"}` or
`dine_in_checkout_recovery_sessions_total{result="close_failed"}` page or
notify the accountable route.
