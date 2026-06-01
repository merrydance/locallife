# Merchant Order Operations Slice

Status: merchant-state flow slice created; manual refund idempotency and pending order-refund recovery repaired 2026-05-31; reject refund submission truth repaired 2026-06-01
Risk class: G3 where merchant actions create or recover refunds; G2 for non-money order status and print transitions
Scope: merchant order list/detail/kitchen/print anomaly pages -> merchant and kitchen order APIs -> order status/refund/print durable state -> notification, websocket, worker, callback, and recovery paths

## Variant Coverage

This slice covers:

- Merchant Mini Program order list accept/reject/ready/complete actions.
- Merchant order detail status actions, print retry/status/manual print, and manual refund workflow.
- Kitchen board and kitchen detail preparing/ready actions.
- Flutter merchant App order list/detail/alert accept/reject/ready actions, including local auto-accept and BLE auto-print after accept.
- Backend merchant/kitchen order routes, handlers, logic, transactions, SQL writes, printing tasks, refund creation, refund callback/fact/outbox/recovery, and merchant websocket publish.

This slice does not fully cover:

- Customer-side order create/pay/cancel flows, except where they are downstream readers or recovery inputs.
- Rider/delivery lifecycle after merchant acceptance, except for the delivery pool enqueue on accepted takeout orders.
- Full merchant device/display/printer configuration. Printing behavior is traced here only where order operations consume that configuration.
- Full provider contract matrices for Baofu/WeChat. Refund terminal convergence is traced to existing payment-domain callback, fact, outbox, and recovery owners.

## Product Invariant

Merchant order operations must converge from merchant action to durable order/refund/print truth:

- Order status transitions must be ownership-checked and conditional on the expected current state.
- Status changes must write an audit/status log in the same transaction as the state mutation.
- Refund-triggering actions must not falsely report a refund as started or recoverable unless a durable refund row or compensating recovery path exists.
- Manual refund creation must use backend-required idempotency so duplicate taps and retries do not create duplicate refund orders.
- Print tasks should be best-effort but observable through `print_logs`, retry/status APIs, callbacks, and anomaly alerts.
- Frontend local state may update optimistically only when rehydrated from backend truth or followed by an explicit reload path.

## Primary Forward Chain

1. Merchant order list initializes by loading backend order truth, then connects websocket listeners.
   Evidence: `weapp/miniprogram/pages/merchant/orders/list/index.ts:152`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:159`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:164`.

2. Order list refreshes from `MerchantOrderManagementService.getOrderList`, maps backend `orders/total`, and preserves current rows when a silent refresh fails.
   Evidence: `weapp/miniprogram/pages/merchant/orders/list/index.ts:281`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:313`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:320`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:322`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:342`.

3. List page actions call accept, reject, ready, and complete wrappers. Reject copy now comes from the backend `refund_submission.message` when present.
   Evidence: `weapp/miniprogram/pages/merchant/orders/list/index.ts:523`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:529`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:536`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:541`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:554`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:560`.

4. `performAction` sets per-order submitting state, calls the wrapper, then reloads the list from backend truth.
   Evidence: `weapp/miniprogram/pages/merchant/orders/list/index.ts:566`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:570`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:572`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:573`.

5. Order detail loads the order, print jobs, payments, refunds, and refund-return records. It preserves last good print/refund subviews when secondary sync fails.
   Evidence: `weapp/miniprogram/pages/merchant/orders/detail/index.ts:103`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:116`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:158`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:162`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:181`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:207`.

6. Detail status actions call the same merchant order wrappers and then set the returned order, reload detail truth, and ask the previous list page to reload.
   Evidence: `weapp/miniprogram/pages/merchant/orders/detail/index.ts:329`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:333`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:354`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:358`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:550`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:553`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:558`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:563`.

7. Detail print actions retry a print log, query cloud status, or create a manual print task, then reload detail truth.
   Evidence: `weapp/miniprogram/pages/merchant/orders/detail/index.ts:362`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:376`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:388`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:397`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:411`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:424`.

8. Detail manual refund validates amount locally, calls `createRefund`, polls `GET /v1/refunds/:id` until terminal or timeout, then reloads detail.
   Evidence: `weapp/miniprogram/pages/merchant/orders/detail/index.ts:485`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:515`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:529`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts:530`, `weapp/miniprogram/pages/merchant/_main_shared/services/refund-workflow.ts:28`, `weapp/miniprogram/pages/merchant/_main_shared/services/refund-workflow.ts:43`.

9. Fixed 2026-05-31 in `6a19a9c0`: the Mini Program refund wrapper requires refund options and sends the backend-required `Idempotency-Key` header; order detail keeps a per-draft refund idempotency key and reuses it on retry of the unchanged draft.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/payment.ts`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts`, `weapp/scripts/check-merchant-manual-refund-idempotency-contract.test.js`.

10. Kitchen board listens for merchant open-status changes and notification-style order messages, loads kitchen order truth, and starts preparing or marks ready through kitchen wrappers.
    Evidence: `weapp/miniprogram/pages/merchant/kitchen/index.ts:254`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:258`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:268`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:329`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:345`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:439`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:451`.

11. Kitchen realtime disconnects if its open-status refresh fails.
    Evidence: `weapp/miniprogram/pages/merchant/kitchen/index.ts:319`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:321`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:325`.

12. Merchant order wrappers map status and print actions to `GET/POST /v1/merchant/orders/**`; kitchen wrappers map KDS actions to `GET/POST /v1/kitchen/orders/**`.
    Evidence: `weapp/miniprogram/pages/merchant/_api/order-management.ts:320`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:386`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:397`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:409`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:420`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:431`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:442`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:453`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:464`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:500`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:522`, `weapp/miniprogram/pages/merchant/_api/order-management.ts:533`.

13. Fixed 2026-06-01: the merchant reject wrapper accepts the compatible backend response shape with top-level order fields plus nested `order/refund_submission`, so Mini Program pages can display backend refund submission truth after reject.
    Evidence: `weapp/miniprogram/pages/merchant/_api/order-management.ts`, `weapp/miniprogram/pages/merchant/orders/list/index.ts`, `weapp/miniprogram/pages/merchant/orders/detail/index.ts`.

14. Flutter merchant App `OrderNotifier` reads the same merchant order list/detail APIs and calls the same accept/reject/ready endpoints. It uses per-order single-flight guards and readback confirmation when a mutation response does not include an order snapshot.
    Evidence: `merchant_app/lib/features/order/order_provider.dart:28`, `merchant_app/lib/features/order/order_provider.dart:45`, `merchant_app/lib/features/order/order_provider.dart:74`, `merchant_app/lib/features/order/order_provider.dart:96`, `merchant_app/lib/features/order/order_provider.dart:116`, `merchant_app/lib/features/order/order_provider.dart:135`, `merchant_app/lib/features/order/order_provider.dart:149`.

15. Fixed 2026-06-01: Flutter reject handling extracts nested `order` and backend `refund_submission.message`, then shows that message after a successful reject.
    Evidence: `merchant_app/lib/features/order/order_provider.dart`, `merchant_app/lib/features/order/order_detail_page.dart`, `merchant_app/lib/features/order/order_alert_page.dart`.

16. Flutter App can auto-accept a newly alerted order from push/poll/backfill if local `notificationSettings.autoAcceptEnabled` is true. This is an App-local SharedPreferences setting and does not read backend `order_display_configs.auto_accept_paid_orders`.
    Evidence: `merchant_app/lib/features/order/order_alert_coordinator.dart:127`, `merchant_app/lib/features/order/order_alert_coordinator.dart:140`, `merchant_app/lib/features/order/order_alert_coordinator.dart:378`, `merchant_app/lib/features/settings/notification_settings_provider.dart:41`, `merchant_app/lib/features/settings/notification_settings_provider.dart:68`.

17. Flutter App can print a local Bluetooth receipt after manual or automatic accept when local `autoPrintAfterAcceptEnabled` is true and a BLE printer is connected. This does not create backend `print_logs`.
    Evidence: `merchant_app/lib/features/order/order_alert_coordinator.dart:388`, `merchant_app/lib/features/order/order_alert_coordinator.dart:390`, `merchant_app/lib/features/order/order_detail_page.dart:513`, `merchant_app/lib/features/order/order_detail_page.dart:521`, `merchant_app/lib/features/order/order_list_page.dart:724`, `merchant_app/lib/features/order/order_list_page.dart:735`, `merchant_app/lib/features/printer/printer_provider.dart:146`, `merchant_app/lib/features/printer/printer_provider.dart:198`.

16. Backend merchant order routes are protected for `owner`, `manager`, and `cashier`; kitchen routes are protected for `owner`, `manager`, and `chef`; refund routes are under authenticated routes and rely on service ownership checks.
    Evidence: `locallife/api/server.go:988`, `locallife/api/server.go:989`, `locallife/api/server.go:992`, `locallife/api/server.go:999`, `locallife/api/server.go:1000`, `locallife/api/server.go:1001`, `locallife/api/server.go:1002`, `locallife/api/server.go:1003`, `locallife/api/server.go:1016`, `locallife/api/server.go:1017`, `locallife/api/server.go:1022`, `locallife/api/server.go:1023`, `locallife/api/server.go:1086`, `locallife/api/server.go:1088`.

17. Merchant status handlers resolve the current merchant and call the shared order command service.
    Evidence: `locallife/api/order.go:1300`, `locallife/api/order.go:1310`, `locallife/api/order.go:1320`, `locallife/api/order.go:1400`, `locallife/api/order.go:1416`, `locallife/api/order.go:1426`, `locallife/api/order.go:1502`, `locallife/api/order.go:1512`, `locallife/api/order.go:1522`, `locallife/api/order.go:1736`, `locallife/api/order.go:1746`, `locallife/api/order.go:1756`.

18. Kitchen handlers also resolve the merchant, verify order ownership, then call the same accept/ready service methods.
    Evidence: `locallife/api/kitchen.go:292`, `locallife/api/kitchen.go:303`, `locallife/api/kitchen.go:314`, `locallife/api/kitchen.go:325`, `locallife/api/kitchen.go:335`, `locallife/api/kitchen.go:373`, `locallife/api/kitchen.go:384`, `locallife/api/kitchen.go:395`.

19. Core merchant order logic locks the order, checks merchant ownership and expected status, then uses order transactions. Takeout accept/ready use takeout-specific transactions; non-takeout accept/ready use `UpdateOrderStatusTx`; complete uses `CompleteOrderTx`.
    Evidence: `locallife/logic/merchant_order.go:28`, `locallife/logic/merchant_order.go:29`, `locallife/logic/merchant_order.go:36`, `locallife/logic/merchant_order.go:46`, `locallife/logic/merchant_order.go:50`, `locallife/logic/merchant_order.go:68`, `locallife/logic/merchant_order.go:119`, `locallife/logic/merchant_order.go:130`, `locallife/logic/merchant_order.go:137`, `locallife/logic/merchant_order.go:157`, `locallife/logic/merchant_order.go:173`, `locallife/logic/merchant_order.go:184`, `locallife/logic/merchant_order.go:187`, `locallife/logic/merchant_order.go:191`.

20. `UpdateOrderStatusTx` writes `orders` and `order_status_logs` together. `CancelOrderTx` also rolls back vouchers, membership balance, delivery state, and inventory for paid orders. `CompleteOrderTx` writes completed status and log.
    Evidence: `locallife/db/sqlc/tx_order_status.go:33`, `locallife/db/sqlc/tx_order_status.go:46`, `locallife/db/sqlc/tx_order_status.go:62`, `locallife/db/sqlc/tx_order_status.go:97`, `locallife/db/sqlc/tx_order_status.go:104`, `locallife/db/sqlc/tx_order_status.go:110`, `locallife/db/sqlc/tx_order_status.go:145`, `locallife/db/sqlc/tx_order_status.go:178`, `locallife/db/sqlc/tx_order_status.go:188`, `locallife/db/sqlc/tx_order_status.go:256`, `locallife/db/sqlc/tx_order_status.go:287`, `locallife/db/sqlc/tx_order_status.go:329`.

21. SQL conditionally writes order status based on expected state for ordinary updates, but `UpdateOrderToCompleted` only excludes cancelled/completed at SQL level.
    Evidence: `locallife/db/query/order.sql:200`, `locallife/db/query/order.sql:206`, `locallife/db/query/order.sql:207`, `locallife/db/query/order.sql:235`, `locallife/db/query/order.sql:242`, `locallife/db/query/order.sql:243`.

22. `OrderService` sends customer notifications, publishes merchant order snapshots as message type `order_update`, enqueues takeout delivery-pool events, and schedules print tasks after accepted/ready.
    Evidence: `locallife/logic/order_service.go:553`, `locallife/logic/order_service.go:559`, `locallife/logic/order_service.go:574`, `locallife/logic/order_service.go:575`, `locallife/logic/order_service.go:577`, `locallife/logic/order_service.go:581`, `locallife/logic/order_service.go:627`, `locallife/logic/order_service.go:648`, `locallife/logic/order_service.go:652`.

23. Reject flow cancels the order first, then calls `ProcessMerchantRejectRefund`; any refund error is logged and the service still returns the cancelled order as success, but since 2026-06-01 the result also carries `RefundSubmission`.
    Evidence: `locallife/logic/merchant_order.go:85`, `locallife/logic/merchant_order.go:104`, `locallife/logic/merchant_order.go:115`, `locallife/logic/order_service.go:586`, `locallife/logic/order_service.go:607`, `locallife/logic/order_service.go:612`, `locallife/logic/order_service.go:620`, `locallife/logic/order_service.go:624`.

24. Merchant reject refund creates a refund order, then calls Baofu refund before share. Missing facade leaves the row pending and maps to `pending_recovery`; provider error marks failed and maps to `manual_required`; accepted provider request maps to `accepted`; no paid payment order maps to `not_needed`.
    Evidence: `locallife/logic/merchant_reject_refund.go:79`, `locallife/logic/merchant_reject_refund.go:92`, `locallife/logic/merchant_reject_refund.go:94`, `locallife/logic/merchant_reject_refund.go:108`, `locallife/logic/merchant_reject_refund.go:115`, `locallife/logic/merchant_reject_refund.go:132`, `locallife/logic/merchant_reject_refund.go:134`, `locallife/logic/merchant_reject_refund.go:144`, `locallife/logic/merchant_reject_refund.go:148`.

25. Manual refund backend requires `Idempotency-Key` in the handler and service before creating or replaying refund orders.
    Evidence: `locallife/api/payment_order.go:1192`, `locallife/api/payment_order.go:1202`, `locallife/api/payment_order.go:1220`, `locallife/api/payment_order.go:1221`, `locallife/api/payment_order.go:1236`, `locallife/api/payment_order.go:1242`, `locallife/logic/refund_service.go:77`, `locallife/logic/refund_service.go:78`, `locallife/logic/refund_service.go:91`, `locallife/logic/refund_service.go:92`.

26. Refund terminal convergence uses Baofu callback/query facts, payment fact application, refund outbox, and notification or alert workers for processing refund rows.
    Evidence: `locallife/logic/payment_fact_application_service.go:187`, `locallife/logic/payment_fact_application_service.go:221`, `locallife/logic/payment_fact_application_service.go:518`, `locallife/logic/payment_fact_application_service.go:536`, `locallife/logic/payment_fact_application_service.go:539`, `locallife/logic/payment_fact_application_service.go:573`, `locallife/logic/payment_fact_application_service.go:1300`, `locallife/logic/payment_fact_application_service.go:1348`, `locallife/worker/task_payment_domain_outbox.go:532`, `locallife/worker/task_payment_domain_outbox.go:594`, `locallife/worker/task_payment_domain_outbox.go:612`, `locallife/worker/task_payment_domain_outbox.go:670`.

27. Refund recovery scans cancelled order payments with no refund row, pending reservation refund rows, and stuck processing refund rows.
    Evidence: `locallife/worker/refund_recovery_scheduler.go:123`, `locallife/worker/refund_recovery_scheduler.go:129`, `locallife/db/query/payment_order.sql:224`, `locallife/db/query/payment_order.sql:233`, `locallife/worker/refund_recovery_scheduler.go:239`, `locallife/db/query/refund_order.sql:76`, `locallife/db/query/refund_order.sql:87`, `locallife/db/query/refund_order.sql:89`, `locallife/worker/refund_recovery_scheduler.go:285`, `locallife/worker/refund_recovery_scheduler.go:325`.

28. Order printing is scheduled by order service, manually scheduled by `PrintMerchantOrder`, executed by the Redis print worker, then updated by direct vendor response or Feieyun callback.
    Evidence: `locallife/logic/order_service.go:683`, `locallife/logic/order_service.go:701`, `locallife/logic/order_service.go:705`, `locallife/logic/order_service.go:709`, `locallife/logic/order_service.go:726`, `locallife/logic/order_service.go:733`, `locallife/logic/order_service.go:737`, `locallife/worker/task_print_order.go:178`, `locallife/worker/task_print_order.go:192`, `locallife/worker/task_print_order.go:204`, `locallife/worker/task_print_order.go:253`, `locallife/api/feieyun_callback.go:17`, `locallife/api/feieyun_callback.go:72`, `locallife/api/feieyun_callback.go:84`.

29. Timed print anomaly scheduler scans stale `print_logs` and publishes platform alerts.
    Evidence: `locallife/scheduler/data_cleanup.go:29`, `locallife/scheduler/data_cleanup.go:136`, `locallife/scheduler/data_cleanup.go:651`, `locallife/scheduler/data_cleanup.go:656`, `locallife/scheduler/data_cleanup.go:674`, `locallife/scheduler/data_cleanup.go:685`.

30. Backend publishes merchant order snapshots using the message type provided by the order service. The Mini Program websocket enum lacks `order_update`, and list/kitchen pages currently listen for `notification` messages shaped like order notifications.
    Evidence: `locallife/api/logic_adapters.go:53`, `locallife/api/logic_adapters.go:77`, `weapp/miniprogram/pages/merchant/_main_shared/utils/websocket.ts:10`, `weapp/miniprogram/pages/merchant/_main_shared/utils/websocket.ts:12`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:168`, `weapp/miniprogram/pages/merchant/orders/list/index.ts:172`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:268`, `weapp/miniprogram/pages/merchant/kitchen/index.ts:273`.

## Reverse-Reference Findings

- Merchant order list, merchant order detail, and kitchen board/detail intentionally converge on the same order service methods. This is a shared writer boundary, not a zombie path.
- Flutter merchant App order list, detail, alert modal, native push, websocket, and polling paths are another client of the same merchant order endpoints. Its local auto-accept setting can mutate backend order state independently of Mini Program display-config auto-accept.
- Kitchen `startPreparing` is semantically the same backend state transition as merchant accept: it calls `AcceptMerchantOrder`.
- `CompleteOrderTx` is broader than merchant completion logic. Merchant logic allows ready-only non-takeout completion, while the SQL primitive allows any non-cancelled/non-completed order. DB tests use it from non-merchant flows, so it should be documented as a broad shared primitive/refactor risk rather than immediately called a defect.
- Fixed 2026-05-31 in `6a19a9c0`: manual refund creation now satisfies the frontend/backend idempotency header contract and has a Mini Program wrapper contract test.
- Fixed 2026-06-01: reject-order refund remains a two-step product workflow, but the API/UI now exposes refund submission truth through `refund_submission` instead of overclaiming that refund submission always started.
- Partially fixed 2026-05-31 in `d3e84050`: refund rows that exist and are stuck in `pending` for cancelled paid normal orders are now recovered with their original `out_refund_no`. Existing terminal `failed` rows still need provider error-classification before automatic retry can be safe.
- Merchant order websocket status updates may not refresh across terminals because `order_update` is not in the Mini Program enum/subscribers.
- Kitchen realtime stops entirely when open-status refresh fails, which can turn an auxiliary status-check failure into stale kitchen order realtime.
- Printing has both order-operation consumers and configuration owners. Device/display configuration should be audited separately because `auto_accept_paid_orders`, `enable_print`, trigger mode, and printer registration are outside this flow.
- Flutter BLE printing is a local receipt side effect and is not observable in backend `print_logs`; it must not be mistaken for cloud-printer command truth.

## SQL And Durable State Boundaries

- `orders`: owns `status`, `fulfillment_status`, cancellation/completion timestamps, cancellation reason, and order-type behavior.
- `order_status_logs`: audit trail for status transitions created inside status transactions.
- `delivery_pool`: created during accepted takeout order transaction.
- `delivery` and rider deposit fields may be updated by cancellation transaction when a cancellable delivery exists.
- `user_vouchers`, `memberships`, `membership_transactions`, and `daily_inventory` can be rolled back by `CancelOrderTx` when cancelling paid orders.
- `payment_orders`: source for paid/refunded payment state and owner checks.
- `refund_orders`: source for manual and merchant-reject refund order state.
- `refund_request_idempotency`: source for manual refund create replay/conflict semantics.
- `external_payment_facts`, `external_payment_fact_applications`, and `payment_domain_outbox`: source for provider terminal facts and user/operator notification convergence.
- `print_logs`: source for print task observability, retry, cloud status, callback matching, and anomaly scans.

## Trust, Authorization, And Tenant Checks

- Merchant order routes require merchant staff role `owner`, `manager`, or `cashier`.
- Kitchen routes require merchant staff role `owner`, `manager`, or `chef`.
- Merchant order handlers resolve the current user's merchant before service calls; core logic rechecks `order.MerchantID`.
- Kitchen handlers resolve the current merchant and check order ownership before service calls.
- Refund `POST /v1/refunds` is registered under authenticated routes; the refund service resolves the actor's merchant and then validates the payment order belongs to that merchant before proceeding.
- Print APIs validate order ownership and print-log association before retry/status operations through the merchant order route group and service/query checks.
- Provider callbacks and payment facts should be treated as provider-truth boundaries. Signature/contract details are owned by the payment-domain standards, not by Mini Program state.

## Idempotency And Duplicate-Submit Checks

- Frontend list/detail/kitchen actions set local `submitting` or `actionOrderId/actionType` guards to suppress duplicate taps.
- Flutter order actions use per-order single-flight futures, and new-order alert processing uses message/order deduplication plus pending-alert stores.
- Backend status writes are conditional on expected status, so a repeated accept/ready/reject after the first successful transition fails instead of reapplying the same state.
- Merchant status actions do not accept request idempotency keys. Re-entry behavior depends on state-conditional updates and frontend reload.
- Manual refund backend requires `Idempotency-Key`, computes a request hash, and replays or rejects duplicate create attempts. Since `6a19a9c0`, the Mini Program wrapper sends the key and order detail reuses the same key for retry of an unchanged draft.
- Merchant reject refund path creates a new refund row directly and does not use the public manual-refund idempotency table. Since `d3e84050`, pending normal-order refund rows for cancelled paid orders are recovered by original `out_refund_no`; since 2026-06-01, provider submission outcome is surfaced as `refund_submission`; automatic retry of terminal `failed` rows remains intentionally open.
- Print worker dedupes accepted/ready re-entry per printer by task key; manual print intentionally creates a new task when manual mode is enabled.
- Feieyun callback updates by vendor order id. Unknown vendor order returns a retryable failure response to the provider.

## Recovery And Async Convergence Paths

- Same-terminal merchant actions rehydrate through HTTP: list reloads orders, detail reloads detail/print/refund state, kitchen reloads board state.
- Flutter App rehydrates with order list/detail HTTP reads and updates local rows from mutation responses; local auto-accept fetches orders after accept before optional BLE printing.
- Backend sends customer notifications for accept/reject/ready/complete and publishes merchant order snapshots for merchant realtime refresh.
- Takeout accept can publish a delivery-pool event for rider-side flows.
- Accepted/ready status transitions enqueue print tasks according to display/printer config; manual print can create a task only when manual trigger mode is enabled.
- Print execution writes `print_logs`, updates success/failed/pending, consumes Feieyun result callbacks, and surfaces anomalies through list/status/retry APIs and timed alert scheduler.
- Manual refunds poll the refund order after creation; terminal truth comes from provider callbacks/query facts applied by payment fact service and payment-domain outbox workers.
- Refund recovery covers cancelled order payments with no refund row, pending normal-order refund rows for cancelled paid orders, pending reservation refund rows, and stuck `processing` refund rows. It still does not automatically retry terminal `failed` rows after provider create failure.

## Frontend Draft And Backend Rehydration

- Order list has no long-lived draft. It marks a row submitting, then reloads backend truth after action success.
- Order detail temporarily applies the returned order after a status action, then reloads full detail, print, payment, and refund truth.
- Kitchen board syncs a returned order into local lists, then reloads kitchen order truth.
- Manual refund popup holds local amount/type/reason draft. After creation it closes the popup, polls the refund row, then reloads order detail truth.
- Detail secondary reads can preserve stale print/refund results with an explicit "保留上次结果" message, which is useful but means stale subflow state can remain visible until a later successful sync.
- Realtime refresh is incomplete for `order_update`, so backend truth may only be seen after manual refresh/re-entry when another terminal performs the action.
- Flutter's local auto-accept/auto-print toggles persist in SharedPreferences and are not rehydrated from backend display-config truth.

## Branch Exhaustion

Entry branches:

- Mini Program merchant order list: list/load/filter, accept, reject, ready, complete, websocket notification refresh, and pull/manual refresh paths are included.
- Mini Program order detail: load detail, load payments/refunds/refund returns, accept/reject/ready/complete, print retry/status/manual print, manual refund popup, refund polling, previous-page reload, and stale secondary subview preservation are included.
- Mini Program kitchen/KDS: board/detail reads, start-preparing as accept, mark-ready, open-status gate, notification-style realtime refresh, and realtime disconnect on open-status refresh failure are included.
- Mini Program print anomalies: anomaly list and retry routes are included through print-log retry/status surfaces.
- Flutter merchant App: order list/detail/alert modal actions, push/native notification tap, websocket/poll/backfill order alert coordinator, local auto-accept, and BLE auto-print after accept are included.
- Out of current scope by decision: Web merchant console. It was briefly scanned and maps to existing flows, but it is not part of this Mini Program/App branch-exhaustion completion gate.

Request branches:

- Merchant order state routes covered: `GET /v1/merchant/orders`, `GET /v1/merchant/orders/:id`, `POST /accept`, `POST /reject`, `POST /ready`, `POST /complete`.
- Kitchen state routes covered: `GET /v1/kitchen/orders`, `GET /v1/kitchen/orders/:id`, `POST /preparing`, `POST /ready`.
- Print routes covered: list print jobs, get print job status, retry print job, create manual print job, list print anomalies.
- Refund routes covered: payment/refund list/detail, refund returns, and `POST /v1/refunds`.
- App routes covered: same `/merchant/orders/**` paths through `OrderNotifier`; no separate App order endpoint was found.
- Fixed 2026-05-31 in `6a19a9c0`: Mini Program refund wrapper reaches `POST /v1/refunds` with the required `Idempotency-Key` header.

Backend state branches:

- Accept path branches by order type: takeout uses takeout-specific accept transaction and delivery-pool enqueue; non-takeout uses general order-status transaction.
- Ready path branches by order type: takeout uses takeout-specific ready transaction; non-takeout uses general order-status transaction.
- Reject path branches: order cancellation commits first; refund creation/provider submission runs after cancellation and does not make the API fail when refund submission fails; since 2026-06-01 the response returns `refund_submission` so clients can distinguish accepted, pending recovery, manual required, and not-needed states.
- Complete path branch: merchant logic guards ready-only non-takeout completion, while `CompleteOrderTx` itself is broader and should remain treated as a shared primitive, not a merchant-only invariant.
- Print scheduling branches: accepted/ready automatic print via display config; manual print requires manual trigger mode; worker filters printer type/order type/role and dedupes by task key.
- Refund branches: manual refund uses idempotency table and refund orchestrator; merchant-reject refund creates a refund order directly and uses Baofu refund-before-share command path.

Async branches:

- Merchant order websocket publish uses `order_update`; Mini Program order/kitchen subscribers currently expect notification-shaped messages, so cross-terminal refresh is not exhausted by implementation.
- Payment/refund terminal truth flows through Baofu callback/query facts, payment fact application, payment-domain outbox, and notification/alert workers.
- Refund recovery scans no-refund cancelled payments, pending normal-order refunds, pending reservation refunds, and stuck `processing` refunds; it does not cover retryable terminal `failed` rows after merchant reject.
- Print async branches include Redis print worker, Feieyun direct response, Feieyun callback by vendor order id, retry print jobs, manual print jobs, and timed print anomaly scheduler.
- Flutter local notification/foreground/polling branches can trigger local auto-accept and BLE print; backend observes only the ordinary accept call, not the local alert or local print side effect.

Failure and retry branches:

- Mini Program status actions have local duplicate-tap guards and rehydrate through HTTP reload; backend duplicate/replay behavior is state-conditional rather than idempotency-keyed.
- Flutter status actions have per-order single-flight futures; incoming alerts have message/order dedupe and pending-alert stores. Cross-channel duplicate proof still needs tests because push, websocket, and polling can race.
- Fixed 2026-05-31 in `6a19a9c0`: manual refund retry reuses the same idempotency key for the same unchanged refund draft.
- Fixed 2026-06-01: reject refund ambiguous/provider failures still leave `orders.status=cancelled`, but the API/UI now exposes pending-recovery or manual-required refund submission state.
- Fixed 2026-05-31 in `d3e84050`: provider-accepted or locally-created order refund rows left in `pending` can now be picked up by refund recovery for cancelled paid normal orders.
- BLE print failure is App-local and has no backend recovery, print log, or merchant-visible reconciliation path.

Reader and consumer branches:

- Merchant readers: order list/detail, kitchen/KDS, print anomaly page, Flutter order list/detail/alert surfaces, dashboard active-order snippets, and realtime consumers.
- Customer and delivery readers: customer order detail/notifications, delivery pool/rider assignment flows after takeout accept, and cancellation/refund visibility.
- Finance/payment readers: payment/refund detail, refund returns, settlement/profit-sharing side effects, payment-domain outbox, and operator alert surfaces.
- Config readers affecting this flow: `order_display_configs` and `cloud_printers` are consumed for cloud print and backend auto-accept but are owned by `merchant-device-display-config`.
- Local App readers affecting this flow: SharedPreferences notification settings and BLE saved device id affect App-side auto-accept/auto-print but are not backend truth.

Authorization and tenant branches:

- Merchant order actions allow owner/manager/cashier; kitchen actions allow owner/manager/chef; refund route is authenticated and checks merchant ownership inside refund service.
- App bound sessions use the same backend staff route authorization as Mini Program tokens; no separate App bypass path was found for order mutation.
- Print retry/status/manual print validate order and print-log association under merchant ownership.
- Provider callbacks are outside merchant auth and must remain signature/provider-truth guarded; detailed provider contract belongs to payment standards, not this slice.

Zombie and unreachable branches:

- `CompleteOrderTx` is broader than merchant completion logic and is a refactor risk if reused without caller-side guard.
- Mini Program websocket `order_update` handling is effectively unreachable in current enum/subscriber shape even though backend publishes it.
- Fixed 2026-05-31 in `6a19a9c0`: manual refund UI path now sends `Idempotency-Key`.
- Partially fixed 2026-05-31 in `d3e84050`: existing pending order refund rows after merchant reject are now picked up; terminal `failed` rows are still not retried automatically.
- Flutter local auto-accept and backend display-config auto-accept are distinct paths; neither is a zombie, but the dual-control contract is undocumented.

Test-proof gaps:

- Fixed 2026-05-31 in `6a19a9c0`: Mini Program contract test proves the wrapper sends `Idempotency-Key` and order detail generates/reuses the draft key.
- Fixed 2026-06-01: logic/API tests prove merchant reject reports pending-recovery and manual-required refund submission states while preserving durable order cancellation success.
- Fixed for `pending` on 2026-05-31 in `d3e84050`: worker and sqlc tests prove recovery includes existing normal-order refund rows stuck in `pending`. Eligible retryable `failed` rows remain open.
- Prove `order_update` refreshes Mini Program list/kitchen/detail across terminals, or deliberately replace it with the notification event shape those pages consume.
- Prove Flutter push/websocket/polling duplicate alerts cannot double-accept or double-print local BLE receipts.
- Prove backend display-config auto-accept and Flutter local alert auto-accept converge safely on one order status and one intended print policy.

## Test Coverage Signals

Observed tests:

- `locallife/logic/merchant_order_test.go` covers merchant order transition logic.
- `locallife/api/order_test.go` covers merchant order action and print APIs.
- `locallife/logic/order_service_print_test.go` covers print scheduling behavior.
- `locallife/worker/task_print_order_test.go` and `locallife/api/feieyun_callback_test.go` cover print worker/callback behavior.
- `locallife/logic/refund_service_test.go`, `locallife/api/payment_order_test.go`, and `locallife/db/sqlc/tx_refund_test.go` cover manual refund idempotency and refund transactions.
- `locallife/worker/refund_recovery_scheduler_test.go` covers refund recovery scans, including pending normal-order refund rows added on 2026-05-31.
- `weapp/scripts/check-merchant-manual-refund-idempotency-contract.test.js` covers Mini Program manual refund idempotency header and draft-key reuse.
- `locallife/integration/takeout_journey_integration_test.go` covers merchant reject in a takeout journey, but it drives legacy refund-result processing manually after reject and does not cover provider submission failure or the pending/failed recovery gap.

Missing high-value tests:

- Fixed 2026-06-01: API/logic tests prove merchant reject surfaces refund-submission state for not-needed, pending-recovery, and manual-required branches.
- Refund recovery tests for eligible retryable `failed` rows after merchant reject, once provider error classification defines safe retry behavior.
- Merchant websocket test or integration fixture proving `order_update` refreshes order list/kitchen state across terminals.
- Kitchen realtime degradation test for open-status refresh failure.
- Flutter App contract tests proving local auto-accept uses the same backend accept endpoint, dedupes push/poll/websocket duplicates, and does not double-print BLE receipts after retries.

## Gaps And Refactor Notes

- Fixed 2026-05-31 in `6a19a9c0`: manual refund wrapper/header is now present and covered by a Mini Program contract test.
- Fixed 2026-06-01: reject-order copy now comes from backend `refund_submission.message`, and the backend contract distinguishes accepted, pending recovery, manual required, and not needed.
- Partially fixed 2026-05-31 in `d3e84050`: refund recovery now covers order refund rows stuck in `pending` and uses original `out_refund_no`; `failed` retry remains open pending provider duplicate/error classification.
- Align websocket message types: either publish the notification type the Mini Program listens for, or add `order_update` handling in the Mini Program order/kitchen flows.
- Keep `CompleteOrderTx` broad only if all callers deliberately guard it before use. If refactoring, rename or split it to avoid accidental merchant-like use without ready-state checks.
- Decide product semantics for the two auto-accept controls: backend display-config `auto_accept_paid_orders` plus cloud-printer gating, and Flutter App local `autoAcceptEnabled` on new-order alerts. They currently coexist and can both accept the same order through conditional backend status logic.
