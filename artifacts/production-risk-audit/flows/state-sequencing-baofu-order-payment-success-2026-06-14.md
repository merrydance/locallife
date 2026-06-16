# State Sequencing Audit: Baofoo Order Payment Success

Date: 2026-06-14
Risk theme: state sequencing
Risk class: G3 - Baofoo callback, money movement, order state, async recovery, and profit-sharing boundary
Status: source-only audit complete

## Flow

Baofoo aggregate payment success callback for `payment_orders.business_type =
order` advances the local payment order, applies order-domain payment success,
creates the order-payment outbox, and prepares the profit-sharing bill boundary.

This flow starts at:

- `POST /v1/webhooks/baofu/payment`, registered in `locallife/api/server.go`.
- Handler: `locallife/api/baofu_callback.go` `handleBaofuPaymentNotify`.

Primary input slice:

- `artifacts/codegraph/backend/baofu-payment/aggregate-payment.slice.md`.

## Explicit Non-Scope

- Reservation payment and reservation add-on terminalization.
- Rider deposit, claim recovery, Baofoo verify fee, and direct WeChat owners.
- Refund, refund recovery, post-share refund/return, and replace-order orchestration.
- Profit-sharing result callback internals beyond the bill/command boundary.
- Provider contract drift review beyond using current Baofoo domain standards.

## State Transition Summary

| Entity | Transition | Guard |
| --- | --- | --- |
| `external_payment_facts` | new fact or duplicate same-shape fact | `dedupe_key` unique index; conflict only returns existing row when provider/channel/capability/source/object/state/amount shape matches |
| `external_payment_fact_applications` | `pending/failed -> processing -> applied/failed` | claim and mark SQL require the expected previous status |
| `payment_orders.status` | `pending -> paid` | `UpdatePaymentOrderToPaid` requires `status = 'pending'`; already-paid conflicts are reloaded and accepted only when amount still matches |
| `orders.status` | `pending -> paid` | `processOrderPaymentWithQueries` locks the order, treats already-paid as idempotent, and rejects non-pending states |
| `payment_orders.processed_at` | `NULL -> now()` | set only after order-domain success in `ProcessPaymentSuccessTx`; SQL requires `status='paid' AND processed_at IS NULL` |
| `payment_domain_outbox` | `pending/failed -> processing -> published/failed` | event/aggregate unique key plus claim/mark status guards |
| `profit_sharing_orders` bill | absent -> pending bill, or matching existing bill reused/refreshed | `EnsureBaofuProfitSharingBillTx` locks the payment order, blocks active refund, checks successful-refund net amount, and reuses identical existing bills |

## Confirmed Production Path

1. Baofoo callback route is registered at `locallife/api/server.go:555`.
2. `handleBaofuPaymentNotify` reads and parses the callback, validates collect identity, loads the local payment order by `outTradeNo` or `tradeNo`, records the payment fact, and attempts to enqueue the application task.
3. If enqueue fails after fact/application persistence, the handler logs a warning and still returns `OK`; retry is owned by `PaymentFactApplicationScheduler`.
4. `BaofuPaymentService.RecordPaymentFact` writes `external_payment_facts` and creates a terminal `external_payment_fact_applications` row for `order_domain/payment_order`.
5. `payment:process_fact_application` calls `PaymentFactService.ApplyExternalPaymentFactApplication`.
6. The application service claims the application, loads the fact, applies the domain transition, creates the payment-domain outbox, marks the fact terminalized, and marks the application applied.
7. For Baofoo order payment success, `markBaofuPaymentOrderPaid` validates upstream success amount against local `payment_orders.amount`, then uses `UpdatePaymentOrderToPaid`.
8. `ProcessPaymentSuccessTx` locks the payment order and returns early unless the payment order is paid and unprocessed.
9. For `business_type='order'`, `processOrderPaymentWithQueries` locks the order, treats already-paid as idempotent, rejects non-pending states, decrements inventory under row locks, and updates the order to paid.
10. After the transaction succeeds, `createOrderPaymentOutbox` ensures the Baofoo profit-sharing bill when required and creates one `order_payment_succeeded` outbox.
11. `PaymentDomainOutboxScheduler` periodically enqueues pending/failed outbox rows; dispatch sends notifications and can auto-accept/print, but it does not directly run Baofoo share.
12. Baofoo share command dispatch remains gated until order completion through completion/recovery paths.

## Evidence

- Callback route: `locallife/api/server.go:551` through `locallife/api/server.go:556`.
- Callback handler and enqueue fallback: `locallife/api/baofu_callback.go:202` through `locallife/api/baofu_callback.go:262`.
- Payment order callback lookup fallback: `locallife/api/baofu_callback.go:265` through `locallife/api/baofu_callback.go:312`.
- Payment fact record/application creation: `locallife/logic/baofu_payment_service.go:218` through `locallife/logic/baofu_payment_service.go:283`.
- Fact/application SQL guards: `locallife/db/query/external_payment_fact.sql:156` through `locallife/db/query/external_payment_fact.sql:325`.
- Fact/application unique indexes: `locallife/db/migration/000217_external_payment_facts.up.sql:87` and `locallife/db/migration/000217_external_payment_facts.up.sql:117`.
- Application worker: `locallife/worker/task_payment_fact_application.go:77` through `locallife/worker/task_payment_fact_application.go:139`.
- Application scheduler: `locallife/worker/payment_fact_application_scheduler.go:93` through `locallife/worker/payment_fact_application_scheduler.go:142`.
- Domain application: `locallife/logic/payment_fact_application_service.go:190` through `locallife/logic/payment_fact_application_service.go:249`.
- Baofoo payment paid transition and amount guard: `locallife/logic/baofu_payment_fact_application.go:14` through `locallife/logic/baofu_payment_fact_application.go:67`.
- Payment order paid SQL: `locallife/db/query/payment_order.sql:153` through `locallife/db/query/payment_order.sql:160`.
- Payment success transaction: `locallife/db/sqlc/tx_payment_success.go:37` through `locallife/db/sqlc/tx_payment_success.go:55` and `locallife/db/sqlc/tx_payment_success.go:214` through `locallife/db/sqlc/tx_payment_success.go:337`.
- Payment order processed guard: `locallife/db/query/payment_order.sql:211` through `locallife/db/query/payment_order.sql:216`.
- Order payment processing: `locallife/db/sqlc/tx_create_order.go:304` through `locallife/db/sqlc/tx_create_order.go:431`.
- Order paid SQL: `locallife/db/query/order.sql:224` through `locallife/db/query/order.sql:233`.
- Order payment outbox and profit-sharing bill boundary: `locallife/logic/payment_fact_application_service.go:1176` through `locallife/logic/payment_fact_application_service.go:1263`.
- Outbox SQL guards: `locallife/db/query/external_payment_fact.sql:355` through `locallife/db/query/external_payment_fact.sql:426`.
- Outbox unique index: `locallife/db/migration/000219_add_payment_domain_outbox_event_aggregate_unique_index.up.sql:1`.
- Outbox scheduler/dispatch: `locallife/worker/payment_domain_outbox_scheduler.go:115` through `locallife/worker/payment_domain_outbox_scheduler.go:163`; `locallife/worker/task_payment_domain_outbox.go:120` through `locallife/worker/task_payment_domain_outbox.go:181`.
- Baofoo profit-sharing bill ensure: `locallife/db/sqlc/tx_baofu_profit_sharing.go:113` through `locallife/db/sqlc/tx_baofu_profit_sharing.go:171`.

## State Sequencing Assessment

No immediate production-code change is recommended from this source-only
state-sequencing pass.

Reasons:

- Provider callback ingestion is separated from domain mutation through durable
  facts and applications.
- Duplicate callback/application processing is guarded at both unique-key and
  state-transition levels.
- Payment order success uses compare-and-set from `pending` to `paid`.
- Order-domain application is transaction-owned, locks both payment and order
  rows, and rejects non-pending order states before inventory and paid-state
  mutation.
- `processed_at` is written only after domain success, preserving retry
  visibility for partial failures.
- Post-payment notifications/auto-accept/printing are behind a durable outbox.
- Baofoo share is not triggered directly from the payment callback; it remains
  gated by order completion and refund-safety checks.

## Side Findings

| ID | Theme | Finding | Handling |
| --- | --- | --- | --- |
| SIDE-001 | Idempotency/retry | Callback enqueue failure returns `OK` after fact/application persistence. This is intentional only because `PaymentFactApplicationScheduler` scans pending/failed applications every minute. | Keep as state-sequencing evidence; expand during idempotency/retry phase if needed. |
| SIDE-002 | Provider ordering | Success facts after local closed/failed payment state are rejected by paid-transition conflict logic unless local state is already paid. | Covered as state sequencing; future fresh tests should pin exact callback-order scenarios if terminal ordering code changes. |
| SIDE-003 | Transaction consistency | Profit-sharing bill creation happens after order-domain transaction inside application processing. If bill/outbox creation fails, the application is failed and retried while `payment_orders.processed_at` is already set. | Acceptable with current retry design; revisit under transaction consistency if repeated bill failures occur. |
| SIDE-004 | Async side effects | Order paid notifications, auto-accept, and print scheduling are outbox side effects, not part of the payment success transaction. | Keep out of this flow's repair scope; audit under downstream fulfillment/notification slices if needed. |

## Existing Test Signals

Identified focused tests include:

- `locallife/api/baofu_callback_test.go`: Baofoo payment callback fact persistence, tradeNo fallback, identity mismatch, provider fallback errors.
- `locallife/logic/baofu_payment_service_test.go`: Baofoo payment fact recording and terminal application creation.
- `locallife/logic/payment_fact_application_service_test.go`: Baofoo order payment success, amount mismatch, closed/failed terminal facts, outbox retry, unclaimable application.
- `locallife/db/sqlc/external_payment_fact_test.go`: fact dedupe, application state transitions, outbox dedupe and lifecycle.
- `locallife/db/sqlc/tx_payment_success_test.go`: order payment success and payment success idempotency signals.
- `locallife/db/sqlc/tx_baofu_profit_sharing_test.go`: Baofoo bill/command refund and stale-amount guards.
- `locallife/worker/payment_fact_application_scheduler_test.go`: scheduler target enqueue behavior.
- `locallife/worker/task_payment_domain_outbox_test.go`: outbox dispatch lifecycle and order-payment side effects.
- `locallife/worker/baofu_payment_recovery_scheduler_test.go`: recovery query and fact application enqueue behavior.

These were identified by source search in this audit. They were not run in this
pass.

## Validation Performed In This Pass

- Read repository routing and backend/payment-domain guidance before auditing.
- Cross-checked the existing codegraph slice against current Go and SQL source.
- Validated the existing aggregate-payment edge JSON with `jq empty`.

## Not Validated

- No Go unit or integration tests were run.
- No live Baofoo callback, sandbox callback, provider retry, or production sample
  was exercised.
- No database migration was executed.
- No generated artifacts were regenerated, because no SQL query, schema,
  Swagger, or interface source was changed.

## Next Step

Before modifying this flow, run focused validation from `locallife/`:

```bash
go test ./api -run 'TestBaofuPaymentCallback' -count=1
go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuOrderPayment|TestBaofuPaymentServiceRecordPayment' -count=1
go test ./db/sqlc -run 'TestCreateExternalPaymentFact|TestExternalPaymentFactApplication|TestPaymentDomainOutbox|TestProcessPaymentSuccessTx_OrderSetsPaidFields|TestEnsureBaofuProfitSharingBillTx' -count=1
go test ./worker -run 'TestPaymentFactApplicationSchedulerRunOnce|TestProcessTaskPaymentDomainOutbox|TestBaofuPaymentRecoverySchedulerRunOnce' -count=1
```
