# Baofoo Refund Slice

Status: first LocalLife-aware refund codegraph slice; covers selected Baofoo pre-share refund variants, not every refund path
Risk class: G3 - funds, Baofoo refund command, callback/query recovery, async fact application, and profit-sharing boundary
Scope: Baofoo pre-share refund command -> refund callback/query fact -> order/reservation refund terminalization -> outbox side effects

## Variant Coverage

This slice covers Baofoo aggregate pre-share refunds for:

- `business_type = order`, applied by `order_domain/refund_order`.
- `business_type = reservation`, applied by `reservation_domain/refund_order`.
- `business_type = reservation_addon`, routed as reservation-domain refund.

It does not claim to cover:

- Rider deposit refunds, which use WeChat direct refund facts.
- Claim recovery refunds.
- Replace-order Baofoo refund orchestration beyond the shared Baofoo refund command/fact contract.
- Merchant reject refund orchestration beyond the shared Baofoo refund command/fact contract.
- Post-share refund or profit-sharing return flows.
- Any refund path whose payment channel is not `baofu_aggregate`.

## Capability Groups

- Pre-share refund: `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md`, section 6.
- Provider contract boundary: `locallife/baofu/aggregatepay/**`.
- Business orchestration: `locallife/worker/task_process_payment.go`, `locallife/api/baofu_callback.go`, `locallife/worker/refund_recovery_scheduler.go`, `locallife/logic/payment_fact_application_service.go`, `locallife/db/sqlc/tx_refund.go`.

## Primary Chain

1. Refund command work starts in `ProcessTaskInitiateRefund`.
   For normal order refunds, it loads the paid payment order, creates or reuses a deterministic refund order, and dispatches Baofoo aggregate payments to `processBaofuAggregateRefund`.
   Evidence: `locallife/worker/task_process_payment.go:877`, `locallife/worker/task_process_payment.go:895`, `locallife/worker/task_process_payment.go:924`, `locallife/worker/task_process_payment.go:954`, `locallife/worker/task_process_payment.go:971`.

2. `CreateRefundOrderTx` is the first funds guard. It locks the `payment_orders` row, rejects non-paid payments, blocks Baofoo refunds after profit-sharing has started, and counts pending/processing/success refunds to prevent over-refund.
   Evidence: `locallife/db/sqlc/tx_refund.go:32`, `locallife/db/sqlc/tx_refund.go:56`, `locallife/db/sqlc/tx_refund.go:93`, `locallife/db/sqlc/tx_refund.go:97`, `locallife/db/sqlc/tx_refund.go:107`, `locallife/db/sqlc/tx_refund.go:112`.

3. `processBaofuAggregateRefund` builds a Baofoo `RefundBeforeShareRequest` with collect merchant identity, refund notify URL, refund amount, total amount, reason, and either original Baofoo trade number or original out trade number. It calls `CreateRefund`.
   Evidence: `locallife/worker/task_process_payment.go:1024`, `locallife/worker/task_process_payment.go:1049`, `locallife/worker/task_process_payment.go:1059`, `locallife/worker/task_process_payment.go:1065`.

4. Baofoo synchronous refund result is command acceptance, not business terminal truth.
   `SUCCESS` marks `refund_orders` as `processing`; `FAIL` marks failed; unknown result codes also stay `processing`. The worker records an `external_payment_commands` row for accepted/rejected/unknown command outcome.
   Evidence: `locallife/worker/task_process_payment.go:1087`, `locallife/worker/task_process_payment.go:1089`, `locallife/worker/task_process_payment.go:1095`, `locallife/worker/task_process_payment.go:1097`, `locallife/worker/task_process_payment.go:1103`, `locallife/worker/task_process_payment.go:1109`.

5. Baofoo calls `POST /v1/webhooks/baofu/refund`.
   Route registration is in `locallife/api/server.go`; the handler is `handleBaofuRefundNotify`.
   Evidence: `locallife/api/server.go:538`, `locallife/api/baofu_callback.go:435`.

6. `handleBaofuRefundNotify` reads the body, parses and verifies the Baofoo refund notification, validates collect identity, loads `refund_orders` by `out_refund_no`, loads the owning `payment_orders` row, records a refund fact, then enqueues the matching fact application worker.
   Evidence: `locallife/api/baofu_callback.go:441`, `locallife/api/baofu_callback.go:447`, `locallife/api/baofu_callback.go:458`, `locallife/api/baofu_callback.go:463`, `locallife/api/baofu_callback.go:469`, `locallife/api/baofu_callback.go:478`, `locallife/api/baofu_callback.go:487`.

7. `recordBaofuRefundCallbackFact` writes an `external_payment_facts` row with provider `baofu`, channel `baofu_aggregate`, capability `baofu_refund`, source `callback`, object type `refund`, and business object `refund_order`.
   It routes normal orders to `order_domain/refund_order` and reservation/refund add-ons to `reservation_domain/refund_order`.
   Evidence: `locallife/api/baofu_callback.go:501`, `locallife/api/baofu_callback.go:511`, `locallife/api/baofu_callback.go:520`, `locallife/api/baofu_callback.go:541`, `locallife/api/baofu_callback.go:553`.

8. If callback is missing or enqueue fails, `RefundRecoveryScheduler` queries stuck processing refunds. For Baofoo aggregate payments it calls `QueryRefund`, normalizes `refundState` or `resultCode`, records a query-sourced Baofoo refund fact, and enqueues the same fact application worker.
   Evidence: `locallife/worker/refund_recovery_scheduler.go:325`, `locallife/worker/refund_recovery_scheduler.go:335`, `locallife/worker/refund_recovery_scheduler.go:363`, `locallife/worker/refund_recovery_scheduler.go:449`, `locallife/worker/refund_recovery_scheduler.go:453`, `locallife/worker/refund_recovery_scheduler.go:514`, `locallife/worker/refund_recovery_scheduler.go:537`, `locallife/worker/refund_recovery_scheduler.go:570`.

9. `payment:process_fact_application` calls `PaymentFactService.ApplyExternalPaymentFactApplication`.
   The service claims the application, loads the fact, dispatches by consumer/object type, creates a payment-domain outbox when needed, then marks fact/application terminalized/applied.
   Evidence: `locallife/worker/task_payment_fact_application.go:47`, `locallife/worker/task_payment_fact_application.go:73`, `locallife/logic/payment_fact_application_service.go:187`, `locallife/logic/payment_fact_application_service.go:193`, `locallife/logic/payment_fact_application_service.go:212`, `locallife/logic/payment_fact_application_service.go:221`, `locallife/logic/payment_fact_application_service.go:227`, `locallife/logic/payment_fact_application_service.go:236`.

10. For `order_domain/refund_order`, `applyOrderRefundFact` validates Baofoo refund facts, loads the refund/payment orders, checks the payment order really belongs to the order domain, then terminalizes `refund_orders`.
    Success updates refund order to `success` and may mark the payment order `refunded` only when total successful refunds cover the payment amount. Closed and failed statuses are terminal but do not regress an already different terminal state.
    Evidence: `locallife/logic/payment_fact_application_service.go:518`, `locallife/logic/payment_fact_application_service.go:524`, `locallife/logic/payment_fact_application_service.go:532`, `locallife/logic/payment_fact_application_service.go:536`, `locallife/logic/payment_fact_application_service.go:539`, `locallife/logic/payment_fact_application_service.go:545`, `locallife/logic/payment_fact_application_service.go:548`, `locallife/logic/payment_fact_application_service.go:568`, `locallife/logic/payment_fact_application_service.go:611`, `locallife/logic/payment_fact_application_service.go:761`.

11. For `reservation_domain/refund_order`, `applyReservationRefundFact` uses the same Baofoo refund fact contract. On success it updates the refund order, may mark payment refunded, and subtracts the successful refund amount from reservation prepaid amount once.
    Evidence: `locallife/logic/payment_fact_application_service.go:633`, `locallife/logic/payment_fact_application_service.go:647`, `locallife/logic/payment_fact_application_service.go:651`, `locallife/logic/payment_fact_application_service.go:655`, `locallife/logic/payment_fact_application_service.go:662`, `locallife/logic/payment_fact_application_service.go:665`, `locallife/logic/payment_fact_application_service.go:734`.

12. Refund terminalization can create payment-domain outbox events. Normal order refund success creates `order_refund_succeeded`; failed/abnormal creates `order_refund_abnormal`. The outbox scheduler later enqueues `payment:process_domain_outbox`.
    Evidence: `locallife/logic/payment_fact_application_service.go:1107`, `locallife/logic/payment_fact_application_service.go:1123`, `locallife/logic/payment_fact_application_service.go:1300`, `locallife/logic/payment_fact_application_service.go:1311`, `locallife/logic/payment_fact_application_service.go:1329`, `locallife/logic/payment_fact_application_service.go:1348`, `locallife/worker/payment_domain_outbox_scheduler.go:21`, `locallife/worker/payment_domain_outbox_scheduler.go:148`.

## SQL And State Boundaries

- `refund_orders.status`: `pending -> processing -> success|failed|closed`.
- `UpdateRefundOrderToProcessing` only works from `pending`.
- `UpdateRefundOrderToSuccess`, `UpdateRefundOrderToFailed`, and `UpdateRefundOrderToClosed` only work from `pending` or `processing`.
- `payment_orders.status`: set to `refunded` only after total successful refunds are greater than or equal to the payment amount.
- `CreateRefundOrderTx` counts `pending`, `processing`, and `success` refund orders as occupied refund amount.
- Baofoo refund command outcomes are stored as commands; final refund truth is stored as facts.
- Query facts dedupe by `baofu:query:refund:{outRefundNo}:{terminalStatus}`; callback facts dedupe by `baofu:callback:refund:{outRefundNo}:{notifyId or upstreamState}`.

Key SQL files:

- `locallife/db/query/refund_order.sql`
- `locallife/db/query/payment_order.sql`
- `locallife/db/query/external_payment_fact.sql`
- `locallife/db/sqlc/tx_refund.go`

## Trust And Idempotency Checks

- Callback parser verifies Baofoo signature before local fact recording.
- Callback identity must match configured collect merchant/terminal identity.
- Callback fact owner/consumer is derived from local `payment_orders`, not from provider text alone.
- Baofoo refund callback requires a local `refund_orders` row by `outRefundNo`.
- `CreateRefundOrderTx` serializes refund amount checks with `GetPaymentOrderForUpdate`.
- Baofoo refunds are blocked after profit-sharing command has started.
- Terminal refund facts do not overwrite an already different terminal `refund_orders` state.
- Reservation prepaid amount is decremented only when the reservation refund transitions to success during this application.

## Recovery Paths

- If callback fact application enqueue fails, `PaymentFactApplicationScheduler` re-enqueues pending/failed applications.
- If Baofoo refund stays `processing`, `RefundRecoveryScheduler` queries Baofoo and records a query fact for terminal statuses.
- If payment-domain outbox enqueue or processing fails, `PaymentDomainOutboxScheduler` re-enqueues pending/failed outboxes every minute.
- Unsupported refund recovery targets persist a critical platform alert instead of falling back to legacy result mutation.

## Test Coverage Signals

Observed focused coverage:

- Baofoo refund callback tests: `locallife/api/baofu_callback_test.go`.
- Baofoo refund recovery scheduler tests: `locallife/worker/refund_recovery_scheduler_test.go`.
- Baofoo refund worker tests: `locallife/worker/task_process_payment_reservation_refund_test.go` and related `task_process_payment_*_test.go`.
- Refund transaction guards: `locallife/db/sqlc/tx_refund_test.go`.
- Payment fact refund application tests: `locallife/logic/payment_fact_application_service_test.go`.
- Payment-domain outbox refund dispatch tests: `locallife/worker/task_payment_domain_outbox_rider_deposit_test.go`.

Potential gap to inspect next:

- API-level Baofoo refund callback scenarios should be reviewed against real provider evidence, especially missing `refundState` with `resultCode` fallback, duplicate `notifyId`, local `outRefundNo` miss, success amount mismatch expectations, and reservation add-on routing.

## Refactor Notes

- Do not treat Baofoo `CreateRefund` synchronous `SUCCESS` as final refund success. It is command acceptance and should not directly complete business refund state.
- Preserve the `refund_orders -> external_payment_facts -> external_payment_fact_applications` boundary for terminal provider facts.
- Do not allow refund and Baofoo profit-sharing command execution to race. `CreateRefundOrderTx` and profit-sharing preparation are the critical boundary.
- Keep callback and query recovery facts in the same application path so retry/idempotency behavior stays consistent.
- Be careful with `payment_orders.status = refunded`: partial refunds must not mark the full payment refunded.
