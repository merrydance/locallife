# Baofoo Aggregate Payment Slice

Status: first LocalLife-aware codegraph slice; covers one concrete variant, not every order/payment path
Risk class: G3 - funds, Baofoo callback, async recovery, and profit sharing
Scope: Baofoo aggregate payment success callback -> normal order payment application -> profit-sharing bill/command -> share callback application

## Variant Coverage

This slice covers the `business_type = order` Baofoo aggregate payment path that is eligible for Baofoo profit sharing.

It does not claim to cover:

- Reservation payment or reservation add-on payment terminalization.
- Rider deposit, claim recovery, Baofoo account verify fee, or other direct-payment owners.
- Non-Baofoo direct WeChat payment callback/query paths.
- Refund, refund recovery, or post-share return/refund paths.
- Order creation/cancel/replacement flows that may create or consume payment/refund records before this callback path.

## Capability Groups

- Baofoo aggregate WeChat JSAPI payment: `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md`, section 4.
- Share after pay: `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md`, section 5.
- Provider contract boundary: `locallife/baofu/aggregatepay/**`.
- Business orchestration: `locallife/api/baofu_callback.go`, `locallife/logic/**`, `locallife/worker/**`, `locallife/scheduler/**`, `locallife/db/sqlc/**`.

## Primary Chain

1. Baofoo calls `POST /v1/webhooks/baofu/payment`.
   Route registration is in `locallife/api/server.go`; the handler is `handleBaofuPaymentNotify` in `locallife/api/baofu_callback.go:190`.

2. `handleBaofuPaymentNotify` reads the webhook body, parses it through `baofuPaymentNotificationParser.ParsePaymentNotification`, validates Baofoo collect identity, loads the local `payment_orders` row by `outTradeNo` or `tradeNo`, records an external payment fact, and enqueues a payment fact application if the fact is terminal.
   Evidence: `locallife/api/baofu_callback.go:196`, `locallife/api/baofu_callback.go:202`, `locallife/api/baofu_callback.go:213`, `locallife/api/baofu_callback.go:218`, `locallife/api/baofu_callback.go:227`, `locallife/api/baofu_callback.go:241`.

3. `BaofuPaymentService.RecordPaymentFact` writes `external_payment_facts` with:
   provider `baofu`, channel `baofu_aggregate`, capability `baofu_payment`, source `callback`, object type `baofu_payment_order`, business object `payment_order`.
   It also records actual provider payment fee when Baofoo supplies `feeAmt`.
   Terminal facts create `external_payment_fact_applications` for the appropriate consumer; normal order payments use `order_domain`.
   Evidence: `locallife/logic/baofu_payment_service.go:218`, `locallife/logic/baofu_payment_service.go:251`, `locallife/logic/baofu_payment_service.go:269`, `locallife/logic/baofu_payment_service.go:272`.

4. `payment:process_fact_application` applies the fact asynchronously.
   The task is enqueued by `enqueueOrderPaymentFactApplication`; the processor calls `PaymentFactService.ApplyExternalPaymentFactApplication`.
   Evidence: `locallife/api/baofu_callback.go:241`, `locallife/worker/task_payment_fact_application.go:13`, `locallife/worker/task_payment_fact_application.go:47`, `locallife/worker/task_payment_fact_application.go:73`.

5. `ApplyExternalPaymentFactApplication` claims the application, loads the fact, dispatches by consumer/object type, creates any payment-domain outbox, then marks both the fact and application as processed/applied.
   Evidence: `locallife/logic/payment_fact_application_service.go:193`, `locallife/logic/payment_fact_application_service.go:203`, `locallife/logic/payment_fact_application_service.go:212`, `locallife/logic/payment_fact_application_service.go:221`, `locallife/logic/payment_fact_application_service.go:227`, `locallife/logic/payment_fact_application_service.go:236`.

6. For `order_domain/payment_order`, `applyOrderPaymentFact` first marks the Baofoo payment order paid and validates paid amount before applying order-domain success.
   `UpdatePaymentOrderToPaid` is conditional on `status = 'pending'`, making the paid transition compare-and-set.
   Evidence: `locallife/logic/payment_fact_application_service.go:267`, `locallife/logic/payment_fact_application_service.go:328`, `locallife/logic/baofu_payment_fact_application.go:14`, `locallife/logic/baofu_payment_fact_application.go:31`, `locallife/db/query/payment_order.sql:153`.

7. `ProcessPaymentSuccessTx` runs the business-domain success transaction. For `business_type = 'order'`, it requires `payment_order.order_id`, then calls `processOrderPaymentWithQueries` and finally marks `payment_orders.processed_at`.
   Evidence: `locallife/logic/payment_fact_application_service.go:343`, `locallife/db/sqlc/tx_payment_success.go:37`, `locallife/db/sqlc/tx_payment_success.go:211`, `locallife/db/sqlc/tx_payment_success.go:231`, `locallife/db/sqlc/tx_payment_success.go:306`.

8. After the order payment is processed, `createOrderPaymentOutbox` ensures a Baofoo profit-sharing bill if the payment order requires profit sharing, then creates one `order_payment_succeeded` outbox for downstream effects.
   Evidence: `locallife/logic/payment_fact_application_service.go:1171`, `locallife/logic/payment_fact_application_service.go:1176`, `locallife/logic/payment_fact_application_service.go:1192`, `locallife/logic/payment_fact_application_service.go:1205`, `locallife/logic/payment_fact_application_service.go:1242`.

9. The payment-domain outbox scheduler runs every minute and enqueues `payment:process_domain_outbox` tasks for `order_payment_succeeded` and other event types.
   Its order-payment dispatch path sends paid notifications and can auto-accept/print orders, but it does not directly create the Baofoo share command.
   Evidence: `locallife/worker/payment_domain_outbox_scheduler.go:21`, `locallife/worker/payment_domain_outbox_scheduler.go:85`, `locallife/worker/payment_domain_outbox_scheduler.go:136`, `locallife/worker/task_payment_domain_outbox.go:57`, `locallife/worker/task_payment_domain_outbox.go:96`, `locallife/worker/task_payment_domain_outbox.go:119`.

10. Baofoo share command scheduling happens after the paid order becomes completed. There are two observed paths:
    - `TakeoutAutoCompleteScheduler.scheduleBaofuProfitSharing` resolves a completed order's share bill and enqueues `baofu:process_profit_sharing`.
    - `BaofuPaymentRecoveryScheduler.createReadyProfitSharingOrders` periodically finds paid, completed, refund-safe Baofoo payment orders, creates/refreshes share bills when needed, and enqueues ready share orders.
    Evidence: `locallife/scheduler/takeout_auto_complete.go:142`, `locallife/logic/baofu_profit_sharing_trigger.go:13`, `locallife/worker/baofu_payment_recovery_scheduler.go:92`, `locallife/worker/baofu_payment_recovery_scheduler.go:118`, `locallife/worker/baofu_payment_recovery_scheduler.go:167`, `locallife/worker/baofu_payment_recovery_scheduler.go:180`.

11. `baofu:process_profit_sharing` validates the profit-sharing order, moves it from pending/failed to processing through `PrepareBaofuProfitSharingCommandTx`, builds and validates the Baofoo share request, records an external command, calls `CreateProfitSharing`, records accepted/rejected/unknown command outcome, and stores the upstream share ID when available.
    Evidence: `locallife/worker/task_baofu_profit_sharing.go:73`, `locallife/worker/task_baofu_profit_sharing.go:89`, `locallife/worker/task_baofu_profit_sharing.go:106`, `locallife/worker/task_baofu_profit_sharing.go:118`, `locallife/worker/task_baofu_profit_sharing.go:127`, `locallife/worker/task_baofu_profit_sharing.go:147`, `locallife/worker/task_baofu_profit_sharing.go:169`, `locallife/worker/task_baofu_profit_sharing.go:172`.

12. Baofoo share result callback goes to `POST /v1/webhooks/baofu/share`. The handler parses and verifies the callback, loads the local `profit_sharing_orders` row, records a `baofu_profit_sharing` fact, and enqueues the same payment fact application worker.
    Evidence: `locallife/api/baofu_callback.go:315`, `locallife/api/baofu_callback.go:325`, `locallife/api/baofu_callback.go:330`, `locallife/api/baofu_callback.go:336`, `locallife/api/baofu_callback.go:350`.

13. `BaofuProfitSharingService.RecordShareFact` writes an external payment fact with provider `baofu`, channel `baofu_aggregate`, capability `baofu_profit_sharing`, object type `profit_sharing`, business owner `profit_sharing`, and creates a `profit_sharing_domain/profit_sharing_order` application for terminal facts.
    Evidence: `locallife/logic/baofu_profit_sharing_service.go:192`, `locallife/logic/baofu_profit_sharing_service.go:230`, `locallife/logic/baofu_profit_sharing_service.go:258`, `locallife/logic/baofu_profit_sharing_service.go:262`.

14. `applyProfitSharingFact` applies the share terminal fact. Success requires the local share amount to match expected receiver amounts, then moves the order to `finished`; failed/closed moves processing orders to `failed`.
    Evidence: `locallife/logic/payment_fact_application_service.go:815`, `locallife/logic/payment_fact_application_service.go:849`, `locallife/logic/payment_fact_application_service.go:881`, `locallife/logic/payment_fact_application_service.go:895`.

## SQL And State Boundaries

- `payment_orders.status`: `pending -> paid` via `UpdatePaymentOrderToPaid`, conditional on pending only.
- `payment_orders.processed_at`: set by `ProcessPaymentSuccessTx` after domain processing succeeds.
- `external_payment_facts`: deduped by `dedupe_key`; duplicate facts must match the same provider/channel/capability/source/object/state/amount shape.
- `external_payment_fact_applications`: claimed and retried independently; scheduler re-enqueues pending/failed targets every minute.
- `payment_domain_outbox`: deduped by event type, aggregate type, aggregate id, and payload equivalence ignoring audit IDs.
- `profit_sharing_orders.status`: `pending/failed -> processing -> finished|failed`.
- `external_payment_commands`: records Baofoo share command submission and accepted/rejected/unknown outcome.

Key SQL files:

- `locallife/db/query/payment_order.sql`
- `locallife/db/query/external_payment_fact.sql`
- `locallife/db/query/profit_sharing_order.sql`
- `locallife/db/sqlc/tx_payment_success.go`
- `locallife/db/sqlc/tx_baofu_profit_sharing.go`

## Trust And Idempotency Checks

- Payment callback parser is configured from Baofoo public key in `api/server.go`; handler treats parser errors as verification failure.
- Payment callback identity requires collect merchant/terminal identity presence.
- Payment callback can recover `outTradeNo` from `tradeNo` by querying Baofoo when local transaction lookup misses.
- Payment fact dedupe key is callback-aware: callback facts prefer `notifyId`; other sources fall back to secondary key or upstream state.
- Payment success amount must match local `payment_orders.amount` for Baofoo main-business payment facts.
- Payment success transition uses `UPDATE ... WHERE status = 'pending'`.
- Share command preparation blocks active refund and stale net-amount conditions in `PrepareBaofuProfitSharingCommandTx`.
- Share result success amount must match calculated local receiver amount before `profit_sharing_orders` is finished.

## Recovery Paths

- If callback fact application enqueue fails, `PaymentFactApplicationScheduler` re-enqueues pending/failed applications every minute.
- If payment-domain outbox enqueue or processing fails, `PaymentDomainOutboxScheduler` re-enqueues pending/failed outboxes every minute.
- `BaofuPaymentRecoveryScheduler` runs every five minutes and covers:
  - ready completed orders missing or waiting on share command,
  - processing share orders requiring provider query recovery,
  - pending Baofoo payment orders requiring provider query recovery.

## Test Coverage Signals

Observed focused coverage:

- Provider notification parser tests: `locallife/baofu/aggregatepay/notification/notification_test.go`.
- Baofoo payment fact recording tests: `locallife/logic/baofu_payment_service_test.go`.
- Payment fact application tests including Baofoo order success and profit-sharing transitions: `locallife/logic/payment_fact_application_service_test.go`.
- SQL transaction tests for payment success: `locallife/db/sqlc/tx_payment_success_test.go`.
- SQL transaction tests for Baofoo share bill/command guards: `locallife/db/sqlc/tx_baofu_profit_sharing_test.go`.
- Baofoo payment recovery scheduler tests: `locallife/worker/baofu_payment_recovery_scheduler_test.go`.
- Baofoo profit-sharing worker tests: `locallife/worker/task_baofu_profit_sharing_test.go`.
- Payment-domain outbox scheduler/dispatch tests: `locallife/worker/task_payment_domain_outbox_test.go`.

Potential gap to inspect next:

- API-level Baofoo payment callback tests exist in `locallife/api/baofu_callback_test.go`, but this slice did not run or exhaustively review every scenario. Next review should specifically confirm malformed signature, missing `outTradeNo` with `tradeNo` fallback, duplicate notify ID, amount mismatch, and enqueue-failure behavior.

## Refactor Notes

- Do not collapse callback parsing, fact recording, and domain application into one synchronous path. The current architecture intentionally separates provider facts from local state transitions.
- Do not trigger Baofoo share command directly from the payment callback. The share command waits for order completion and refund safety.
- Preserve `external_payment_facts` and `external_payment_fact_applications` as the audit/retry boundary.
- Preserve the `payment_domain_outbox` boundary for order-paid side effects.
- Any refactor around Baofoo payments must keep the payment fact, order-domain transaction, share bill, share command, share callback, and recovery scheduler as separately auditable steps.
