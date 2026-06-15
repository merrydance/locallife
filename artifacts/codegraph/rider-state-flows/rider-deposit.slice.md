# Rider Deposit Slice

Status: rider-state flow slice created 2026-06-08
Risk class: G3 - rider money movement, WeChat direct payment, refundable deposit credits, refund facts, deposit ledger reconciliation, active delivery/frozen deposit gates, abnormal refund alerts
Scope: Mini Program rider deposit page -> rider deposit/recharge/withdraw APIs -> payment/refund facts -> rider deposit/refund transactions -> credit expiry schedulers and legacy dead paths

## Variant Coverage

This slice covers:

- Rider deposit page balance, ledger, pending recharge recovery, recharge payment, pending withdrawal recovery, and withdrawal submission.
- Backend `/v1/rider/deposit`, `/deposits`, `/withdraw`, and `/withdrawals/status`.
- Rider deposit direct WeChat recharge payment order creation and payment fact application.
- Generic payment recovery reads used by the rider deposit page: `GET /v1/payments/:id/query` first, then `GET /v1/payments/:id` fallback when remote query is unsupported/unavailable.
- Rider deposit refundable credit creation, expiry reminder, expiry marking, withdrawal/refund preparation, refund callback/query facts, terminal refund application, and abnormal alert outbox.
- Delivery grab/complete deposit freeze/unfreeze only as durable side effects; delivery state transitions are covered by `rider-delivery-lifecycle`.

This slice does not fully cover:

- Baofu income withdrawal; rider income cash-out is covered by `rider-income-and-baofu-withdrawal`.
- General payment APIs and WeChat callback signature internals outside the rider deposit business owner branch.
- Platform/manual reconciliation tooling beyond the rider-visible anomaly status and alert handoff discovered here.

## Product Invariant

Rider deposit balance must converge through one durable ledger:

- Recharge creates or reuses a pending `payment_orders(business_type='rider_deposit')` row and only credits the rider after terminal payment fact application.
- Payment success must create exactly one visible `rider_deposits(type='deposit')` log and one refundable `rider_deposit_credits` row per paid deposit order.
- Withdrawal is allowed only when the rider exists, is approved/active, has no frozen deposit, has enough available deposit after pending refunds, and has no active deliveries.
- Withdrawal preparation reserves refundable credits oldest-first, creates refund orders, writes hidden freeze logs, and increases rider `frozen_deposit`.
- Refund terminal truth must be applied through payment fact application; success drains deposit/frozen/credit, while failed/closed/abnormal restores credit and unfreezes.

## Primary Forward Chain

1. Rider deposit page loads balance, rider status, deposit ledger, pending recharge state, and pending withdrawal state on load/show/refresh.
   Evidence: `weapp/miniprogram/pages/rider/deposit/index.ts:147`, `weapp/miniprogram/pages/rider/deposit/index.ts:192`, `weapp/miniprogram/pages/rider/deposit/index.ts:211`, `weapp/miniprogram/pages/rider/deposit/index.ts:224`, `weapp/miniprogram/pages/rider/deposit/index.ts:233`, `weapp/miniprogram/pages/rider/deposit/index.ts:269`.

2. Frontend rider service calls `GET /v1/rider/deposit`, `GET /v1/rider/deposits`, `POST /v1/rider/deposit`, `POST /v1/rider/withdraw`, and `GET /v1/rider/withdrawals/status`.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:150`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:154`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:162`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:170`, `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts:178`.

3. Recharge flow saves a pending payment context in local storage, invokes WeChat payment, queries remote/local payment truth after ambiguity, and clears pending context after paid/failed.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:40`, `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:72`, `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:91`, `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:133`, `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:156`.

4. Pending recharge recovery calls `queryPaymentOrder(payment_order_id)` and falls back to `getPaymentDetail(payment_order_id)`; the shared payment route enforces owner scope, can query direct WeChat by `out_trade_no`, records a query fact, and may immediately apply the rider-deposit payment fact.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:185`, `weapp/miniprogram/pages/rider/_main_shared/services/rider-deposit-payment.ts:187`, `weapp/miniprogram/pages/rider/_main_shared/api/payment.ts:576`, `weapp/miniprogram/pages/rider/_main_shared/api/payment.ts:583`, `locallife/api/server.go:1107`, `locallife/api/server.go:1108`, `locallife/api/payment_order.go:389`, `locallife/api/payment_order.go:402`, `locallife/logic/payment_order_query_wechat.go:19`, `locallife/logic/payment_order_query_wechat.go:63`, `locallife/logic/payment_order_query_wechat.go:304`, `locallife/logic/payment_order_query_wechat.go:392`, `locallife/logic/payment_order_query_wechat.go:382`.

5. Backend deposit routes are registered under `/v1/rider`.
   Evidence: `locallife/api/server.go:1136`, `locallife/api/server.go:1137`, `locallife/api/server.go:1140`, `locallife/api/server.go:1141`, `locallife/api/server.go:1142`.

6. Deposit balance requires a rider and current region, subtracts pending rider-deposit refunds, reads the effective region deposit threshold, and returns total/frozen/delivery-frozen/withdrawal-processing/available values.
   Evidence: `locallife/api/rider.go:595`, `locallife/api/rider.go:608`, `locallife/api/rider.go:612`, `locallife/api/rider.go:618`, `locallife/api/rider.go:624`, `locallife/db/query/refund_order.sql:174`.

7. Deposit ledger lists `rider_deposits` for the current rider and intentionally hides withdrawal-freeze intermediate logs.
   Evidence: `locallife/api/rider.go:661`, `locallife/api/rider.go:688`, `locallife/api/rider.go:698`, `locallife/db/query/rider_deposit.sql:23`, `locallife/db/query/rider_deposit.sql:33`.

8. Recharge requires rider status `approved` or `active`, direct payment client configuration, and creates or reuses an unexpired pending payment order for same user/business/amount.
   Evidence: `locallife/api/rider.go:369`, `locallife/api/rider.go:394`, `locallife/api/rider.go:398`, `locallife/api/rider.go:415`, `locallife/api/rider.go:428`.

9. Recharge calls WeChat direct JSAPI payment, records accepted/rejected external payment command audit, closes the local payment order if create fails, and returns `pay_params`.
   Evidence: `locallife/api/rider.go:450`, `locallife/api/rider.go:477`, `locallife/api/rider.go:492`, `locallife/api/rider.go:499`.

10. WeChat direct payment callback records a rider deposit payment fact/application with consumer `rider_deposit_domain`.
   Evidence: `locallife/api/payment_callback.go:319`, `locallife/api/payment_callback.go:329`, `locallife/api/payment_callback.go:343`, `locallife/api/payment_callback.go:364`, `locallife/worker/direct_payment_fact.go:60`.

11. Payment callback and payment query facts converge through the same `PaymentFactService` application path; `ProcessPaymentSuccessTx` rider deposit branch is idempotent and creates rider deposit log, refundable credit, and operational status reconciliation.
    Evidence: `locallife/logic/payment_fact_application_service.go:348`, `locallife/db/sqlc/tx_payment_success.go:55`, `locallife/db/sqlc/tx_payment_success.go:63`, `locallife/db/sqlc/tx_payment_success.go:76`, `locallife/db/sqlc/tx_payment_success.go:88`, `locallife/db/sqlc/tx_payment_success.go:100`, `locallife/db/sqlc/tx_payment_success.go:115`.

12. Withdrawal submission service rejects missing rider, inactive rider status, any frozen deposit, insufficient available deposit after pending refunds, and active deliveries.
    Evidence: `locallife/api/rider.go:521`, `locallife/logic/rider_deposit_refund_service.go:71`, `locallife/logic/rider_deposit_refund_service.go:78`, `locallife/logic/rider_deposit_refund_service.go:86`, `locallife/logic/rider_deposit_refund_service.go:94`, `locallife/logic/rider_deposit_refund_service.go:99`, `locallife/logic/rider_deposit_refund_service.go:104`.

13. Withdrawal preparation requires a client/server `Idempotency-Key`, stores a request hash in `rider_deposit_withdrawal_requests`, and replays the same user/key/request by loading the original refund-order plan instead of creating new refund orders. A same key with a different request hash is rejected before refund creation. First-time preparation locks the rider, subtracts pending refunds, locks refundable credits oldest-first, consumes refundable amounts, creates `refund_orders`, writes withdrawal-freeze logs, records the refund order ids on the request row, and increases `riders.frozen_deposit`.
    Evidence: `locallife/api/rider.go:551`, `locallife/api/rider.go:559`, `locallife/api/rider.go:574`, `locallife/logic/rider_deposit_refund_service.go:79`, `locallife/logic/rider_deposit_refund_service.go:83`, `locallife/logic/rider_deposit_refund_service.go:119`, `locallife/db/sqlc/tx_rider_refund.go:76`, `locallife/db/sqlc/tx_rider_refund.go:85`, `locallife/db/sqlc/tx_rider_refund.go:90`, `locallife/db/sqlc/tx_rider_refund.go:93`, `locallife/db/sqlc/tx_rider_refund.go:112`, `locallife/db/sqlc/tx_rider_refund.go:184`, `locallife/db/sqlc/tx_rider_refund.go:233`, `locallife/db/sqlc/tx_rider_refund.go:242`, `locallife/db/migration/000268_add_rider_deposit_withdrawal_idempotency.up.sql:1`.

14. Withdrawal submits one direct refund request per source payment credit. Create failure is compensated by resolving the refund as failed; provider success/processing/closed/abnormal responses are marked processing and converge through facts.
    Evidence: `locallife/logic/rider_deposit_refund_service.go:124`, `locallife/logic/rider_deposit_refund_service.go:149`, `locallife/logic/rider_deposit_refund_service.go:164`, `locallife/logic/rider_deposit_refund_service.go:177`, `locallife/logic/rider_deposit_refund_service.go:183`, `locallife/logic/rider_deposit_refund_service.go:189`, `locallife/logic/rider_deposit_refund_service.go:195`.

15. Refund callback and refund recovery scheduler both record rider deposit direct refund facts/applications for terminal convergence.
    Evidence: `locallife/api/payment_callback.go:376`, `locallife/api/payment_callback.go:1331`, `locallife/api/payment_callback.go:1337`, `locallife/worker/refund_recovery_scheduler.go:344`, `locallife/worker/refund_recovery_scheduler.go:481`.

16. Rider deposit refund fact validation requires terminal WeChat direct refund fact, business owner `rider_deposit`, object type `refund_order`, and matching business object id.
    Evidence: `locallife/logic/payment_fact_application_service.go:503`, `locallife/logic/payment_fact_application_service.go:506`, `locallife/logic/payment_fact_application_service.go:509`, `locallife/logic/payment_fact_application_service.go:512`, `locallife/logic/payment_fact_application_service.go:515`, `locallife/logic/payment_fact_application_service.go:518`.

17. Refund fact application uses `ResolveRiderDepositRefundTx`; success drains deposit/frozen/credit and writes a withdraw log, while failed/closed/abnormal restores credit, unfreezes deposit, and writes an unfreeze log.
    Evidence: `locallife/logic/payment_fact_application_service.go:464`, `locallife/logic/payment_fact_application_service.go:488`, `locallife/db/sqlc/tx_rider_refund.go:188`, `locallife/db/sqlc/tx_rider_refund.go:229`, `locallife/db/sqlc/tx_rider_refund.go:264`, `locallife/db/sqlc/tx_rider_refund.go:284`, `locallife/db/sqlc/tx_rider_refund.go:312`, `locallife/db/sqlc/tx_rider_refund.go:332`, `locallife/db/sqlc/tx_rider_refund.go:351`.

18. Deposit credit scheduler sends expiring-credit reminders and marks expired credits, publishing platform alerts for due/expired batches.
    Evidence: `locallife/scheduler/data_cleanup.go:689`, `locallife/scheduler/data_cleanup.go:703`, `locallife/scheduler/data_cleanup.go:725`, `locallife/scheduler/data_cleanup.go:731`, `locallife/scheduler/data_cleanup.go:754`, `locallife/scheduler/data_cleanup.go:768`, `locallife/scheduler/data_cleanup.go:791`, `locallife/scheduler/data_cleanup.go:811`.

19. Abnormal rider deposit refunds produce a payment-domain outbox and critical alert for manual intervention.
    Evidence: `locallife/logic/payment_fact_application_service.go:1386`, `locallife/logic/payment_fact_application_service.go:1397`, `locallife/worker/task_payment_domain_outbox.go:398`, `locallife/worker/task_payment_domain_outbox.go:422`, `locallife/worker/task_payment_domain_outbox.go:437`, `locallife/worker/task_payment_domain_outbox.go:449`.

## SQL And Durable State Boundaries

- `payment_orders`: rider deposit recharge payment truth with `business_type='rider_deposit'`, direct channel, out trade no, prepay id, paid/processed timestamps.
- `external_payment_commands`: WeChat payment/refund command audit for accepted/rejected/unknown provider calls.
- `external_payment_facts` and `external_payment_fact_applications`: callback/query terminal fact truth and retryable application queue, including Mini Program-initiated payment-query recovery facts.
- `riders`: aggregate deposit amount, frozen deposit, current operational status, active delivery status inputs.
- `rider_deposits`: visible deposit/withdraw/unfreeze/deduct ledger plus hidden withdrawal-freeze records.
- `rider_deposit_credits`: refundable credit source, active/partially_refunded/fully_refunded/expired statuses, paid/refundable-until/reminder timestamps.
- `rider_deposit_withdrawal_requests`: durable request-level idempotency source for rider deposit withdrawal, unique by `(user_id, idempotency_key)`, with request hash, requested/accepted amounts, and linked refund order ids.
- `refund_orders`: direct refund intent and terminal status for rider deposit withdrawals.
- `payment_domain_outbox`: abnormal refund alert handoff.

## Trust, Authorization, And Tenant Checks

- Deposit routes resolve rider by authenticated user id.
- Generic payment query/detail routes scope `payment_orders.user_id` to the authenticated user before exposing status or applying query facts.
- Withdrawal status query filters refund order ids by the current user and `business_type/refund_type='rider_deposit'`.
- Refund callback resolves local refund order by out refund no, then verifies payment order business type before recording rider deposit facts.
- Fact application validates provider/channel/capability/business owner/object id before mutating deposit/refund rows.

## Idempotency And Duplicate-Submit Checks

- Recharge reuses pending payment order by user/business/amount.
- Payment success transaction checks existing deposit log and credit before creating missing artifacts.
- Payment query fact dedupe uses provider/out-trade/status keys; query-triggered application may no-op when the order is already processed.
- Withdrawal POST now requires `Idempotency-Key`. The backend stores a request hash in `rider_deposit_withdrawal_requests`; same user/key/hash replays the original refund order ids, and same user/key with a different hash returns conflict. The Mini Program deposit page keeps a stable draft key and stores it with the pending withdrawal context for re-entry.
- Refund terminal application is idempotent for already success/failed/closed rows.
- Fact dedupe keys are callback/query based; scheduler retries applications every minute.

## Recovery And Async Convergence Paths

- Frontend can continue a pending recharge and query remote payment status.
- If direct remote query is unsupported/unavailable, frontend falls back to local payment detail and keeps the pending context until terminal truth is visible.
- Payment callback and payment recovery both record facts; payment fact application scheduler retries.
- Frontend stores pending withdrawal refund ids and can wait/poll terminal status.
- Refund callback and refund recovery both record terminal refund facts.
- Abnormal refund fact preserves a payment-domain alert handoff; the rider-facing recovery path remains deposit/withdrawal status rehydration.
- Deposit credit expiry reminder/expiry schedulers handle refundable window lifecycle.

## Frontend Draft And Backend Rehydration

- Deposit page derives withdraw availability from backend balance plus rider status active-delivery count.
- Recharge local pending context is only a UI recovery handle; backend `payment_orders` and facts are canonical.
- Withdrawal pending context is built from returned refund order ids; backend refund status is canonical.

## Test Coverage Signals

Observed tests:

- `locallife/api/rider_test.go` covers deposit balance, recharge, withdrawal, withdrawal status, and deposit ledger API branches.
- `locallife/db/sqlc/tx_rider_refund_test.go` covers refund preparation, request idempotency replay/conflict/split-plan loading, split credits, ledger anomalies, terminal success/closed restore, and stale credit reconciliation.
- `locallife/logic/rider_deposit_refund_service_test.go` covers required idempotency key, key/hash propagation, and replay handling that skips new WeChat refund create calls for already non-pending refund rows.
- `locallife/api/payment_callback_test.go`, `locallife/worker/payment_recovery_scheduler_test.go`, `locallife/worker/refund_recovery_scheduler_test.go`, and `locallife/logic/payment_fact_application_service_test.go` include rider deposit fact/application coverage.
- `locallife/scheduler/rider_deposit_credit_scheduler_test.go` covers reminders and expiry.
- `locallife/worker/task_process_payment_test.go` covers legacy refund-result skip for rider deposit.
- `weapp/scripts/check-rider-deposit-withdrawal-idempotency-contract.test.js` covers the Mini Program `Idempotency-Key` header, stable draft key, pending-withdrawal context persistence, and legacy pending-withdrawal recovery compatibility.

Missing high-value tests:

- Mini Program pending recharge and pending withdrawal recovery across app restart.
- Contract coverage that `/v1/payments/:id/query` applies rider-deposit query facts but falls back cleanly to `/v1/payments/:id` in the Mini Program when remote query is unsupported.
- End-to-end recharge paid callback -> fact application -> deposit visible balance refresh.
- End-to-end withdrawal processing -> refund callback/query -> terminal page sync.
- Ledger anomaly repair workflow after partial failures.

## Gaps And Refactor Notes

- Fixed before this synchronization pass: rider deposit withdrawal POST has durable request idempotency through `Idempotency-Key`, `rider_deposit_withdrawal_requests`, request hash replay/conflict checks, and Mini Program draft-key persistence. Before changing this path, run the focused API/logic/sqlc/weapp contract tests rather than relying on this documentation snapshot.
- Decide whether withdrawal freeze ledger should remain hidden from all rider-visible list variants or exposed in an audit-only view.
- Add operational docs for `rider_deposit_credits` expiry and stale credit reconciliation.

## Dead And Orphan Paths

- `locallife/worker/task_process_payment.go:794` through `locallife/worker/task_process_payment.go:797` intentionally skips rider deposit refund result application in the legacy refund worker; rider deposit refund results must be applied through payment fact application.
- `locallife/worker/task_process_payment.go:908` and later still contain older rider-deposit refund helper code paths, but terminal application is explicitly blocked at the legacy refund-result consumer.

## Branch Exhaustion

- Entry branches checked: deposit page load/show/refresh, balance error, ledger pagination, recharge dialog, continue pending recharge, payment status recovery, withdrawal dialog, withdrawal submit, wait terminal refund status, pending withdrawal manual refresh.
- Request branches checked: deposit balance, deposit ledger, recharge, withdraw, withdrawal status, direct payment callback/query fact, generic payment query/detail recovery, direct refund callback/query fact.
- Backend state branches checked: missing rider, missing region, inactive rider, no payment service, pending payment reuse, WeChat create failure, payment query unsupported/provider unavailable, paid but unprocessed payment, existing log/credit idempotency, insufficient available deposit, frozen deposit, active deliveries, credit split, refund create failure compensation, SUCCESS/PROCESSING/CLOSED/ABNORMAL upstream statuses, terminal success/failed/closed/abnormal application, expired credits.
- Async branches checked: payment fact scheduler, refund recovery scheduler, payment-domain outbox, credit reminder/expiry scheduler, frontend pending context recovery.
- Dead/orphan branches checked: legacy refund-result worker skip and older helper code path.
