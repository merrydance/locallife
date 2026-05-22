# BaoFu Profit Sharing Bill Decoupling Fix Plan

Date: 2026-05-22
Risk class: G3 - funds, profit sharing, delivery state, provider callbacks, workers, schedulers, and frontend-visible settlement amounts.
Target areas: `locallife/`, `merchant_app/`, and possibly `weapp/` rider display surfaces.

> **For future agents and engineers:** this file is intended to be enough context to continue after memory loss. Do not re-infer the business rule from isolated functions. Follow the background, invariants, and task boundaries below.

## 1. Goal

Decouple profit-sharing calculation from profit-sharing execution.

The system must calculate a durable merchant-facing settlement bill immediately after a customer order payment succeeds. The merchant new-order notification and merchant receipt must show the full bill. Rider delivery income must be decoupled from merchant income. After a rider accepts an order, the rider-side payment channel fee and net delivery income must be calculated and shown to the rider. The actual BaoFu profit-sharing action must only be triggered by the WeChat Mini Program official shipping settlement notification.

## 2. Non-Negotiable Business Rules

1. Calculation is not money movement.
   - Creating or updating `profit_sharing_orders` is a durable bill calculation.
   - Calling BaoFu `share after pay` is the actual external profit-sharing action.

2. Merchant bill timing:
   - The merchant bill must be available after the customer order payment is successfully applied.
   - The merchant new-order notification must carry this bill.
   - Merchant order details and receipt printing must read this same bill.

3. Rider bill timing:
   - Merchant revenue and rider delivery revenue must be separate calculations.
   - Before rider assignment, rider receiver and rider fee may be absent.
   - After rider accepts the order, update the same bill with rider ID, rider gross delivery fee, rider payment channel fee, and rider net income.
   - The rider must see the channel fee and net income after accepting.

4. Execution timing:
   - BaoFu profit-sharing execution must be triggered by `POST /v1/webhooks/wechat-miniprogram/settlement-notify`.
   - This WeChat notification corresponds to Mini Program shipping settlement/unfreeze.
   - User explicit confirmation releases immediately. If the user does not confirm receipt, WeChat auto-releases after delivery plus about 48 hours according to the Mini Program operating rule described in the product request.

5. Recovery timing:
   - Recovery schedulers may retry or query ambiguous processing states.
   - Recovery schedulers must not be the primary owner of first-time bill calculation.
   - Recovery schedulers must not silently execute profit sharing before the WeChat settlement/unfreeze trigger has been recorded.

6. Refund and cancellation:
   - Do not restore canceled orders to paid or completed just to let notifications or profit-sharing logic pass.
   - Existing incident context in `artifacts/baofu-cancel-order-refund-context.md` proves this trap already happened around order `202605191920420fbb3f`.
   - A paid order can be canceled after payment. Merchant notification and refund recovery must not depend on mutating the order back to a non-canceled state.

7. No downgrade handling:
   - No missing payment order, missing bill, missing receiver, unsupported status, malformed callback, unknown enum, failed signature, provider failure, or DB failure may be converted into success, empty response, skipped no-op, or vague "best effort".
   - Downgrade is forbidden for this change unless a later approved contract explicitly documents a safe optional branch with structured logs, metrics, frontend guidance, and tests. The current plan assumes no such downgrade exists.

8. Error visibility:
   - Every unexpected failure must land in structured logs exactly at the boundary that decides fail, retry, skip, or return.
   - Frontend-facing failures must receive stable Chinese business guidance. Do not expose raw provider text, SQL errors, Go driver errors, stack traces, signatures, certificates, credentials, full raw callbacks, bank cards, identity numbers, or internal payloads.

## 3. Current System Facts

### 3.1 Existing storage already has most required bill fields

`profit_sharing_orders` already includes:

- `payment_order_id`
- `merchant_id`
- `operator_id`
- `order_source`
- `total_amount`
- `delivery_fee`
- `rider_id`
- `rider_amount`
- `distributable_amount`
- `platform_rate`
- `operator_rate`
- `platform_commission`
- `operator_commission`
- `merchant_amount`
- `out_order_no`
- `sharing_order_id`
- `status`
- `payment_fee`
- `payment_fee_rate_bps`
- `provider`
- `channel`
- `merchant_sharing_mer_id`
- `rider_sharing_mer_id`
- `operator_sharing_mer_id`
- `platform_sharing_mer_id`
- `sharing_detail_snapshot`
- `calculation_version`
- `settlement_mode`
- `provider_payment_fee`
- `provider_payment_fee_rate_bps`
- `provider_payment_fee_base_amount`
- `provider_payment_fee_source`
- `merchant_payment_fee`
- `merchant_payment_fee_rate_bps`
- `merchant_payment_fee_base_amount`
- `rider_gross_amount`
- `rider_payment_fee`
- `rider_payment_fee_rate_bps`
- `rider_payment_fee_base_amount`
- `commission_base_amount`
- `platform_receiver_amount`

Use this table as the first implementation target. Do not create a parallel `settlement_bills` table in the first fix unless implementation proves `profit_sharing_orders` cannot preserve the invariants.

### 3.2 Existing calculation logic

Relevant files:

- `locallife/logic/baofu_fee_calculator.go`
- `locallife/logic/baofu_profit_sharing_service.go`
- `locallife/db/sqlc/tx_baofu_profit_sharing.go`
- `locallife/db/query/profit_sharing_order.sql`

Current calculation version is `baofu_fee_v2`.

Existing formula shape:

- BaoFu provider fee estimate: default 30 bps over payment total.
- Merchant service fee: default 60 bps over merchant fee base.
- Rider service fee: default 60 bps over rider fee base.
- For takeout with rider receiver:
  - rider gross = min(delivery fee, total amount)
  - rider payment fee = ceil(rider gross * 60 bps)
  - rider net = rider gross - rider payment fee
  - merchant fee base = total amount - rider gross
  - merchant amount = merchant base - merchant payment fee - platform commission - operator commission
  - platform receiver amount = platform commission + merchant payment fee + rider payment fee - provider payment fee

### 3.3 Current problematic coupling

`locallife/db/sqlc/tx_baofu_profit_sharing.go` currently has `CreateBaofuProfitSharingOrderTx`.

That transaction currently both:

- creates the durable `profit_sharing_orders` row and fee ledgers, and
- enforces execution-style guards such as "there is already a refund" and "there is already a profit-sharing order".

This couples bill calculation to action eligibility. It is wrong for the new requirement because:

- merchant notification needs the bill at payment success,
- the order may later be canceled and refunded,
- profit-sharing action should wait for WeChat settlement/unfreeze,
- duplicate bill creation and duplicate provider action need different idempotency semantics.

### 3.4 Payment success and notification path

Relevant files:

- `locallife/logic/payment_fact_application_service.go`
- `locallife/db/sqlc/tx_payment_success.go`
- `locallife/worker/task_payment_domain_outbox.go`
- `locallife/worker/task_process_payment.go`

Current path:

1. BaoFu payment fact is applied.
2. `ProcessPaymentSuccessTx` updates the order.
3. `order_payment_succeeded` outbox is created.
4. Outbox dispatch eventually calls `notifyMerchantNewOrder`.
5. `notifyMerchantNewOrder` currently requires `order.Status == paid`.
6. It loads `profit_sharing_orders` through `loadMerchantOrderFeeBreakdown`.
7. If the bill is missing or order is already canceled, the outbox can repeatedly fail.

Known production fact from `artifacts/baofu-cancel-order-refund-context.md`:

- `payment_domain_outbox.id = 3` was retrying `order_payment_succeeded`.
- Failure was `notify merchant new order: merchant new order notification requires paid order: order_id=7 status=cancelled`.
- That outbox failure is a separate post-payment notification issue. Do not "fix" it by restoring order state.

### 3.5 Existing merchant bill response

Relevant files:

- `locallife/logic/merchant_order_fee_breakdown.go`
- `locallife/api/order.go`
- `locallife/worker/task_process_payment.go`

Existing `MerchantOrderFeeBreakdown` fields:

- `food_amount`
- `merchant_discount_amount`
- `voucher_discount_amount`
- `food_payable_amount`
- `delivery_fee_amount`
- `delivery_fee_discount_amount`
- `delivery_payable_amount`
- `customer_payable_amount`
- `platform_service_fee_amount`
- `payment_channel_fee_amount`
- `merchant_receivable_amount`

This is already close to the merchant-facing bill. Extend from this contract instead of inventing a second shape.

### 3.6 Existing rider surfaces

Relevant files:

- `locallife/api/delivery.go`
- `locallife/logic/delivery_grab.go`
- `locallife/db/sqlc/tx_delivery.go`
- `locallife/api/rider_income.go`
- `locallife/api/rider_workbench.go`
- `weapp/miniprogram/pages/rider/**`

Current delivery response includes:

- `delivery_fee`
- `rider_earnings`

It does not expose:

- `rider_gross_amount`
- `rider_payment_fee`
- `rider_net_earnings`
- `profit_sharing_order_id`
- `profit_sharing_status`

### 3.7 Existing WeChat settlement callback

Relevant files:

- `locallife/api/payment_callback.go`
- `locallife/wechat/contracts/shipping_settlement.go`
- `locallife/worker/task_upload_shipping_info.go`

Current route:

- `POST /v1/webhooks/wechat-miniprogram/settlement-notify`

Current behavior:

- Verifies WeChat signature through `directPaymentClient`.
- Parses `trade_manage_order_settlement`.
- Decrypts `ShippingSettlementNotificationResource`.
- Logs `transaction_id`, `merchant_trade_no`, `settlement_time`, `confirm_receive_method`.
- Marks notification processed.
- Does not enqueue BaoFu profit sharing.

This route is the intended trigger for actual BaoFu share execution.

### 3.8 Existing BaoFu share worker

Relevant files:

- `locallife/worker/task_baofu_profit_sharing.go`
- `locallife/worker/baofu_payment_recovery_scheduler.go`
- `locallife/logic/baofu_profit_sharing_service.go`
- `locallife/logic/payment_fact_application_service.go`

Current worker behavior:

- Loads `profit_sharing_orders` by ID.
- Requires provider `baofu` and channel `baofu_aggregate`.
- Allows status `pending` or `failed`.
- Builds request from `sharing_detail_snapshot`.
- Calls BaoFu `CreateProfitSharing`.
- Marks the local share order `processing`.
- Terminal callback or query facts later mark `finished` or `failed`.

This worker can be reused, but its enqueue trigger must move to WeChat settlement notification.

### 3.9 Existing receipt printing

Relevant files:

- Backend cloud print: `locallife/worker/task_print_order.go`
- Flutter Bluetooth print: `merchant_app/lib/core/print/esc_pos_utils.dart`
- Flutter push model: `merchant_app/lib/models/push_message.dart`
- Flutter order model: `merchant_app/lib/models/order.dart`

Backend Feieyun receipt currently prints:

- order number
- created time
- order type
- item lines
- subtotal
- discount
- voucher
- delivery fee
- customer paid total
- notes and address

It does not print:

- platform service fee
- merchant payment channel fee
- merchant receivable amount
- rider channel fee or net rider income

Flutter receipt currently prints:

- shop name
- order number
- items
- note
- order total

It does not parse or print `fee_breakdown`.

## 4. Target Architecture

### 4.1 Durable bill lifecycle

Use `profit_sharing_orders` as the durable settlement bill.

Recommended status semantics for first fix:

- `pending`: bill calculated and not yet submitted to BaoFu.
- `processing`: BaoFu share command submitted or accepted.
- `finished`: BaoFu share completed.
- `failed`: BaoFu share terminal failure.

Do not add a new status unless a migration proves necessary. `pending` can mean "calculated and awaiting settlement trigger".

### 4.2 Bill creation after payment success

When an order payment fact is applied successfully:

1. Payment success transaction updates order state as it does today.
2. Immediately after a successful payment application, ensure a `profit_sharing_orders` bill exists for `payment_order_id`.
3. This ensure operation must be idempotent.
4. If the bill already exists with same calculation version and same deterministic values, return it.
5. If a bill exists with conflicting values, log and return a hard error. Do not overwrite silently.
6. The merchant bill may have no rider receiver before rider assignment.

### 4.3 Rider assignment update

When a rider accepts a delivery:

1. `GrabOrderTx` remains the state transition owner for assigning delivery and freezing deposit.
2. After the transaction, or inside a dedicated transaction if persistence consistency requires it, update the existing bill with:
   - `rider_id`
   - `rider_sharing_mer_id`
   - `rider_gross_amount`
   - `rider_payment_fee`
   - `rider_payment_fee_rate_bps`
   - `rider_payment_fee_base_amount`
   - `rider_amount`
   - updated `sharing_detail_snapshot`
3. The update must require `profit_sharing_orders.status = pending`.
4. If status is already `processing`, `finished`, or `failed`, return a conflict because the bill can no longer be safely changed.
5. If rider receiver is missing or inactive, reject rider acceptance with a stable Chinese message. The existing readiness guard already blocks missing rider BaoFu account and should remain.

### 4.4 Execution after WeChat settlement/unfreeze

When `/v1/webhooks/wechat-miniprogram/settlement-notify` receives a valid `trade_manage_order_settlement` notification:

1. Verify signature. Failure returns WeChat `FAIL` and logs safe structured context.
2. Decrypt resource. Failure returns WeChat `FAIL` and logs safe structured context.
3. Parse resource. Failure returns WeChat `FAIL` and logs safe structured context.
4. Resolve payment order by:
   - `transaction_id`, if present, otherwise
   - `merchant_trade_no` / local out trade number.
5. Resolve `profit_sharing_orders` by `payment_order_id`.
6. Validate:
   - payment order is `paid`,
   - payment order is main business order,
   - payment channel is `baofu_aggregate`,
   - `requires_profit_sharing = true`,
   - bill provider/channel are `baofu` / `baofu_aggregate`,
   - bill status is `pending` or `failed` if retrying from a previous failed action,
   - no active refund exists that makes sharing unsafe,
   - for takeout, rider portion is complete if delivery fee requires rider share.
7. Create an auditable local record of settlement trigger before enqueueing action. Prefer `payment_domain_outbox` or an explicit external payment fact/command record. Do not rely only on logs.
8. Enqueue `TaskProcessBaofuProfitSharing` with `profit_sharing_order_id`.
9. If enqueue fails, release the notification claim and return WeChat `FAIL` so the official notification can retry.
10. If enqueue succeeds, mark notification processed and return success.

### 4.5 Recovery after implementation

Scheduler responsibilities after the fix:

- Recover missing calculated bills for old paid orders only when a safe business rule allows it.
- Retry `pending` bills only when a settlement trigger record exists.
- Query `processing` BaoFu share orders and apply terminal facts.
- Never silently execute sharing for a paid order that has no WeChat settlement trigger.

## 5. Error Handling Contract

### 5.1 Logging rules

Use structured logs. At minimum include:

- `payment_order_id`
- `order_id`, if known
- `profit_sharing_order_id`, if known
- `merchant_id`, if known
- `rider_id`, if known
- `out_trade_no` or masked provider object key
- `transaction_id`, masked or provider-safe
- `notification_id`, for WeChat callback
- `event_type`
- `capability`
- `status`
- error class or reason

Do not log:

- full raw decrypted WeChat resource,
- BaoFu full sensitive identifiers if domain rules mark them sensitive,
- bank cards,
- identity numbers,
- certificate material,
- private keys,
- API keys,
- unmasked callback payloads.

### 5.2 Frontend-facing semantic messages

Use stable Chinese messages. Proposed messages:

| Scenario | HTTP or delivery surface | Message |
| --- | --- | --- |
| Merchant bill not ready when merchant detail loads | `500` unless caused by known business conflict | `订单收款账单暂不可用，请稍后重试或联系平台处理` |
| Bill exists but values conflict with order/payment | `500`, alert operator | `订单收款账单异常，请联系平台处理` |
| Rider BaoFu account missing | `400` | `骑手结算账户未开通，暂不能接收配送费分账订单` |
| Rider bill cannot be calculated because merchant bill missing | `409` or `500` depending cause | `订单配送收益账单暂不可用，请稍后重试` |
| Rider bill cannot update because sharing already submitted | `409` | `订单结算已进入处理，不能重新接单` |
| WeChat settlement callback signature failure | WeChat notify `FAIL` | `signature verification failed` in callback response; log Chinese internal context |
| WeChat settlement callback cannot find payment order | WeChat notify `FAIL` | `payment order not found` in callback response; log internal context |
| WeChat settlement callback cannot find bill | WeChat notify `FAIL` | `profit sharing bill not found` in callback response; log internal context and alert |
| BaoFu share enqueue failure | WeChat notify `FAIL` | `enqueue failed, please retry` |
| BaoFu provider share call failure | Asynq retry | Log provider-classified failure and keep task retry semantics |

For HTTP handlers, use existing patterns:

- Business 4xx from logic: `logic.NewRequestError`.
- Unexpected 5xx: plain wrapped error to API layer, then `internalError(ctx, err)`.
- Upstream 502/503: `loggedServerError(ctx, err, publicMessage, logMessage)`.

For workers and schedulers:

- Log where the decision to retry, skip, or stop is made.
- Return retryable errors for infrastructure/provider temporary failures.
- Return `asynq.SkipRetry` only for malformed task payloads or permanently invalid local state that also gets logged and alerted.

## 6. Phase Plan

Each phase must be independently reviewable. Do not start later phases by refactoring earlier files broadly.

### Phase 0: Contract and baseline tests

Purpose: lock the expected behavior before implementation.

Files to inspect:

- `locallife/logic/baofu_fee_calculator.go`
- `locallife/logic/baofu_profit_sharing_service.go`
- `locallife/db/sqlc/tx_baofu_profit_sharing.go`
- `locallife/logic/payment_fact_application_service.go`
- `locallife/worker/task_process_payment.go`
- `locallife/api/payment_callback.go`
- `locallife/api/delivery.go`
- `locallife/worker/task_print_order.go`
- `merchant_app/lib/models/push_message.dart`
- `merchant_app/lib/core/print/esc_pos_utils.dart`

Tasks:

- [ ] Add tests proving payment-success bill calculation is required before merchant notification.
- [ ] Add tests proving merchant notification for a paid-then-canceled order does not require reverting order status.
- [ ] Add tests proving rider acceptance exposes rider payment fee and net income.
- [ ] Add tests proving WeChat settlement notify enqueues BaoFu sharing only after valid signature, valid decrypted resource, payment lookup, bill lookup, and eligibility checks.
- [ ] Add tests proving callback failures return WeChat `FAIL` and release the notification claim when retry is needed.
- [ ] Add tests proving unsupported/missing state is not downgraded to success.

Validation commands:

```bash
cd locallife
go test ./logic ./worker ./api -run 'ProfitSharing|MerchantNewOrder|SettlementNotify|Delivery' -count=1
```

Expected first result before implementation: focused tests fail for missing behavior.

Boundary:

- No production code changes except test scaffolding in this phase.
- No schema changes in this phase.

### Phase 1: Split bill calculation transaction from action eligibility

Purpose: make a durable bill calculation possible at payment-success time.

Files:

- Modify `locallife/db/sqlc/tx_baofu_profit_sharing.go`
- Modify `locallife/db/sqlc/store.go`
- Modify `locallife/logic/baofu_profit_sharing_service.go`
- Modify `locallife/db/query/profit_sharing_order.sql`
- Regenerate `locallife/db/sqlc/*.sql.go`
- Regenerate mocks if store interface changes: `locallife/db/mock/store.go`

Implementation requirements:

- Introduce a clearly named transaction for bill calculation, for example `EnsureBaofuProfitSharingBillTx`.
- Keep `CreateBaofuProfitSharingOrderTx` or replace it with two clear transactions:
  - bill calculation/ensure,
  - action preparation/submission state transition.
- Bill calculation transaction must:
  - lock payment order,
  - permit existing paid order even before completed/delivered state,
  - not reject only because a refund row exists after bill creation,
  - reject conflicting existing bill,
  - create fee ledgers idempotently,
  - return the existing bill when values match.
- Action preparation must keep money-safety checks:
  - no active refund,
  - eligible payment order,
  - eligible bill status,
  - complete receiver snapshot.

No downgrade:

- If existing bill conflicts, return error.
- If payment order is not found, return error.
- If calculation values are negative, return error.
- If receiver resolution fails for merchant/platform, return error.
- Do not create an empty bill.

Validation:

```bash
cd locallife
make sqlc
make mock
go test ./db/sqlc -run 'BaofuProfitSharing|ProfitSharingOrder' -count=1
go test ./logic -run 'BaofuProfitSharingService|BaofuSettlement' -count=1
```

Boundary:

- This phase does not enqueue BaoFu sharing.
- This phase does not modify WeChat callback behavior.

### Phase 2: Calculate merchant bill when order payment succeeds

Purpose: guarantee merchant notification can read a complete merchant bill.

Files:

- Modify `locallife/logic/payment_fact_application_service.go`
- Modify or add focused logic tests in `locallife/logic/payment_fact_application_service_test.go`
- Modify supporting service interfaces if needed.

Implementation requirements:

- After `ApplyOrderPaymentFactResult` reports a processed order payment, ensure the BaoFu bill exists before creating or dispatching `order_payment_succeeded` outbox.
- Use existing payment order data and order data to derive:
  - merchant ID,
  - order source,
  - total amount,
  - delivery fee,
  - operator ID,
  - platform owner ID,
  - deterministic out order number.
- For takeout before rider assignment:
  - merchant-side bill must still be complete enough for merchant notification.
  - rider fields may be zero or absent until Phase 3.
- Use current active BaoFu receiver resolution for merchant/platform.

No downgrade:

- If bill creation fails, payment fact application must fail/retry instead of publishing merchant notification without bill.
- If merchant/platform receiver is missing, fail with structured logs and a stable operator-facing alert path.
- Do not publish `order_payment_succeeded` outbox when the bill is unavailable.

Validation:

```bash
cd locallife
go test ./logic -run 'Apply.*OrderPayment|PaymentFact.*Outbox|BaofuProfitSharing' -count=1
```

Boundary:

- Do not execute BaoFu sharing in this phase.
- Do not change rider acceptance in this phase.

### Phase 3: Update rider bill after rider accepts order

Purpose: decouple rider delivery income from merchant revenue and expose rider channel fee.

Files:

- Modify `locallife/logic/delivery_grab.go`
- Modify `locallife/db/sqlc/tx_delivery.go` only if the bill update must be in the same transaction.
- Modify `locallife/db/query/profit_sharing_order.sql`
- Modify `locallife/api/delivery.go`
- Modify tests:
  - `locallife/logic/delivery_grab_test.go`
  - `locallife/api/delivery_test.go`
  - `locallife/db/sqlc/tx_baofu_profit_sharing_test.go` or a new focused test file.

Implementation requirements:

- Add query/transaction to update rider settlement fields for `payment_order_id`.
- Update only when bill status is `pending`.
- Rebuild `sharing_detail_snapshot` with merchant, rider, operator, platform receivers and amounts.
- Ensure `rider_amount = rider_gross_amount - rider_payment_fee`.
- Ensure `rider_payment_fee` is visible in `deliveryResponse`.
- Add fields to `deliveryResponse`:
  - `rider_gross_amount`
  - `rider_payment_fee`
  - `rider_net_earnings`
  - `profit_sharing_order_id`
  - `profit_sharing_status`
- Keep existing `rider_earnings` for backward compatibility, but set or map it consistently with `rider_net_earnings` once the bill is available.

No downgrade:

- If bill is missing for a paid BaoFu profit-sharing order, reject rider acceptance with a stable message and log.
- If rider receiver is missing or inactive, reject rider acceptance with existing settlement account message.
- If bill status is not `pending`, reject with conflict.
- Do not accept rider assignment while silently failing to calculate rider fee.

Validation:

```bash
cd locallife
make sqlc
make mock
go test ./logic -run 'GrabDeliveryOrder|RiderBaofu|ProfitSharing' -count=1
go test ./api -run 'Delivery|GrabOrder' -count=1
```

Boundary:

- This phase updates local bill only.
- This phase does not submit BaoFu sharing.

### Phase 4: Make merchant notification read the durable bill and tolerate post-payment cancellation

Purpose: send merchant new-order notification with a complete bill without relying on mutable order status.

Files:

- Modify `locallife/worker/task_process_payment.go`
- Modify `locallife/logic/merchant_order_fee_breakdown.go` only if response fields must expand.
- Modify tests:
  - `locallife/worker/task_process_payment_notify_rider_test.go`
  - `locallife/logic/merchant_order_fee_breakdown_test.go`, if present or add one.

Implementation requirements:

- Replace `order.Status == paid` hard requirement with a payment/bill based eligibility check:
  - latest payment order exists,
  - payment order is `paid`,
  - bill exists,
  - order belongs to same merchant/payment order.
- Include full `fee_breakdown` in async notification `ExtraData`.
- Include full `fee_breakdown` in websocket new-order payload.
- If the order is already canceled after payment, notification may still be sent as a paid-order event if the outbox was created by payment success.

No downgrade:

- If payment order is missing, fail and log.
- If bill is missing, fail and log.
- If bill values are inconsistent with order total, fail and log.
- Do not send notification without bill.

Validation:

```bash
cd locallife
go test ./worker -run 'NotifyMerchantNewOrder|PaymentDomainOutbox' -count=1
```

Boundary:

- Do not change cancellation/refund state transitions in this phase.
- Do not mark old failed outbox rows published manually.

### Phase 5: Trigger BaoFu share from WeChat Mini Program settlement notification

Purpose: move the actual profit-sharing action trigger to the official WeChat settlement/unfreeze callback.

Files:

- Modify `locallife/api/payment_callback.go`
- Modify `locallife/wechat/contracts/shipping_settlement.go` only if parsing fields need strict validation.
- Modify `locallife/worker/task_baofu_profit_sharing.go` if action eligibility needs a trigger reference.
- Modify `locallife/db/query/payment_order.sql` if payment lookup by transaction/out trade number is insufficient.
- Modify `locallife/db/query/external_payment_fact.sql` or add a small durable trigger record if needed.
- Modify tests:
  - `locallife/api/payment_callback_test.go`
  - `locallife/worker/task_baofu_profit_sharing_test.go`

Implementation requirements:

- On settlement callback, after signature/decrypt/parse:
  - resolve payment order,
  - resolve bill,
  - validate eligibility,
  - record settlement trigger,
  - enqueue `TaskProcessBaofuProfitSharing`.
- Callback idempotency must use existing `TryClaimWechatNotification` and a durable enqueue/trigger check.
- Duplicate callback must not enqueue duplicate external actions.
- If task enqueue fails, release notification claim and return WeChat `FAIL`.
- If validation fails due to local missing state, log and return WeChat `FAIL` so retry or operator intervention remains visible.

No downgrade:

- Do not acknowledge success when bill is missing.
- Do not acknowledge success when payment cannot be resolved.
- Do not acknowledge success when rider portion is required but incomplete.
- Do not acknowledge success when a refund is active.
- Do not swallow malformed resource fields.

Validation:

```bash
cd locallife
go test ./api -run 'HandleOrderSettlementNotify' -count=1
go test ./worker -run 'BaofuProfitSharing' -count=1
```

Boundary:

- This phase may enqueue BaoFu sharing.
- It must not call BaoFu directly inside the HTTP callback.

### Phase 6: Restrict scheduler to recovery, not primary action trigger

Purpose: prevent the old scheduler from executing sharing just because an order became completed.

Files:

- Modify `locallife/worker/baofu_payment_recovery_scheduler.go`
- Modify `locallife/db/query/profit_sharing_order.sql`
- Modify tests:
  - `locallife/worker/baofu_payment_recovery_scheduler_test.go`

Implementation requirements:

- Scheduler may create missing bills for historical paid orders only if explicitly safe and tested.
- Scheduler may enqueue `pending` share orders only if a settlement trigger record exists.
- Scheduler continues querying `processing` share orders.
- Scheduler continues applying terminal facts via payment fact application tasks.

No downgrade:

- If a `pending` bill has no settlement trigger, log at info or warning level with object IDs and skip without enqueuing action.
- If a trigger exists but enqueue fails, log error and keep retryable state.
- Do not silently mark bill failed or finished from scheduler-only assumptions.

Validation:

```bash
cd locallife
go test ./worker -run 'BaofuPaymentRecovery|ProfitSharingRecovery' -count=1
```

Boundary:

- This phase must not alter provider DTOs.
- This phase must not change refund recovery semantics.

### Phase 7: Expand backend cloud receipt bill details

Purpose: make merchant printed receipts show all bill details from the durable bill.

Files:

- Modify `locallife/worker/task_print_order.go`
- Modify tests:
  - `locallife/worker/task_print_order_test.go`, if present, or add focused tests.

Implementation requirements:

- Load payment order and bill before building full receipt.
- Print at least:
  - `用户实付`
  - `平台服务费`
  - `支付通道费`
  - `商户实收`
- For takeout with rider:
  - `配送费`
  - `骑手通道费`
  - `骑手实收`
- Kitchen split slip may omit financial details if product confirms it is kitchen-only. Full/front slip must include the bill.

No downgrade:

- Manual or automatic full receipt printing must fail with logged error when bill is required but missing.
- Do not print a receipt with incomplete financial detail for paid BaoFu profit-sharing orders.

Validation:

```bash
cd locallife
go test ./worker -run 'PrintOrder|Receipt|FeeBreakdown' -count=1
```

Boundary:

- This phase changes backend cloud print only.
- Flutter Bluetooth receipt is Phase 8.

### Phase 8: Expand Flutter merchant app receipt model and Bluetooth receipt

Purpose: make merchant app receipts print the same bill detail from notifications and order snapshots.

Files:

- Modify `merchant_app/lib/models/order.dart`
- Modify `merchant_app/lib/models/push_message.dart`
- Modify `merchant_app/lib/core/print/esc_pos_utils.dart`
- Modify or add tests under `merchant_app/test/`.

Implementation requirements:

- Add a typed `FeeBreakdown` model matching backend `fee_breakdown`.
- Parse `fee_breakdown` from:
  - websocket notification payload,
  - local notification payload,
  - polled order snapshots.
- Preserve message deduplication by `message_id`.
- Print:
  - `用户实付`
  - `平台服务费`
  - `支付通道费`
  - `商户实收`
  - rider fee rows when present.
- If `items_load_failed` or required fee breakdown is missing for a paid BaoFu order, do not print. Show Chinese error:
  - `订单收款账单仍在同步，暂不打印小票`

No downgrade:

- Do not silently print without fee details for a paid BaoFu profit-sharing order.
- Do not fabricate zero fees when backend omitted the bill.

Validation:

```bash
cd merchant_app
flutter test
flutter analyze
```

Boundary:

- This phase does not change backend APIs.
- It consumes fields added by earlier backend phases.

### Phase 9: Weapp rider display alignment

Purpose: show rider channel fee and net delivery income after acceptance.

Files to inspect first:

- `weapp/AGENTS.md`
- `.github/instructions/weapp-mini-program.instructions.md`
- `weapp/miniprogram/api/delivery.ts`
- `weapp/miniprogram/pages/rider/**`

Implementation requirements:

- Update delivery API types to include:
  - `rider_gross_amount`
  - `rider_payment_fee`
  - `rider_net_earnings`
  - `profit_sharing_status`
- On rider accepted task detail, show gross delivery fee, channel fee, and net expected income.
- Keep copy in Chinese product language. Avoid provider/internal terms where possible; use `通道费` and `预计实收`.

No downgrade:

- If backend says bill is unavailable, show a clear blocking state instead of hiding the row.
- Do not infer fee locally from delivery fee unless backend contract explicitly provides rate and source.

Validation:

```bash
cd weapp
npm run lint
npm run compile
```

If `node`, `npm`, or `npx` is not found, rerun with:

```bash
PATH="$HOME/.local/bin:$PATH" npm run lint
PATH="$HOME/.local/bin:$PATH" npm run compile
```

Boundary:

- This phase is frontend display only.
- It must not define new business rules.

### Phase 10: Swagger, generated outputs, safety, and operational handoff

Purpose: close the implementation with generated artifacts, focused safety checks, and runbook notes.

Files:

- `locallife/docs/swagger.*`, if API annotations or response structs changed.
- `.github/standards/domains/baofu-payment/README.md`, only if domain guidance needs durable update.
- `.github/standards/domains/wechat-payment/README.md`, only if retained Mini Program shipping settlement guidance needs durable update.
- `artifacts/baofu-cancel-order-refund-context.md`, only if incident follow-up status changes.

Validation:

```bash
cd locallife
make sqlc
make mock
make swagger
make check-generated
make check-baofu-contract
make test-safety
go test ./api ./logic ./worker ./db/sqlc -count=1
```

Frontend validation if corresponding phases changed:

```bash
cd merchant_app
flutter test
flutter analyze
```

```bash
cd weapp
npm run lint
npm run compile
```

Operational checks:

- Confirm payment success creates bill.
- Confirm merchant notification includes `fee_breakdown`.
- Confirm paid-then-canceled order does not block notification because of current order status.
- Confirm rider acceptance updates rider fee fields.
- Confirm WeChat settlement notify enqueues BaoFu share.
- Confirm duplicate WeChat settlement notify does not enqueue duplicate share action.
- Confirm scheduler does not enqueue share without settlement trigger.
- Confirm BaoFu share callback/query still marks `finished` or `failed`.
- Confirm refund flow still blocks sharing when active refund exists.

## 7. Provider Contract Checks Before Code Changes

BaoFu capability group:

- Main provider: BaoFu/BaoCaiTong.
- Active capability groups:
  - Aggregate WeChat JSAPI payment.
  - Share after pay.
  - Pre-share refund.
  - BaoFu account opening/binding for merchant, rider, operator, platform receivers.

Must read before provider DTO/request/parser changes:

- `.github/standards/domains/baofu-payment/README.md`
- `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md`
- `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md`
- `.github/standards/domains/baofu-payment/BAOCAITONG_FIELD_CONTRACT_MATRIX.md`
- `.github/standards/domains/baofu-payment/CONTRACT_IMPLEMENTATION_MAP.md`

WeChat capability group:

- Retained Mini Program shipping settlement helper, not WeChat payment acquisition.
- Active route: `POST /v1/webhooks/wechat-miniprogram/settlement-notify`.
- Do not reintroduce retired WeChat platform payment/profit-sharing APIs.

Must read before callback contract changes:

- `.github/standards/domains/wechat-payment/README.md`
- `locallife/wechat/contracts/shipping_settlement.go`
- official Mini Program shipping settlement docs if the local contract fields change.

## 8. Idempotency Requirements

Bill calculation:

- Natural key: `payment_order_id`.
- Repeated calculation with identical deterministic values returns existing bill.
- Repeated calculation with conflicting deterministic values fails.

Rider bill update:

- Natural key: `payment_order_id`.
- Allowed only before external action is submitted.
- Repeated same rider update returns existing updated bill or no-op only after verifying identical values.
- Different rider update after assignment conflict fails.

WeChat settlement callback:

- Natural key: WeChat notification ID through `TryClaimWechatNotification`.
- Secondary action guard: local settlement trigger record or external payment command record tied to `profit_sharing_order_id`.
- Duplicate callback must not enqueue duplicate BaoFu share action.

BaoFu share worker:

- Natural key: `profit_sharing_order_id` and `out_order_no`.
- External command record must be created before provider call.
- If local command exists and bill already `processing`, repeated task must not call provider again.

Outbox:

- Keep using `CreatePaymentDomainOutboxOnce` for idempotent domain events.
- Do not alter existing unique index semantics without a separate SQL review.

## 9. Explicit Non-Goals

- Do not redesign BaoFu provider DTOs unless a field matrix change requires it.
- Do not create a new settlement-bill table in the first fix unless `profit_sharing_orders` cannot uphold the invariants.
- Do not change customer order cancellation semantics.
- Do not "repair" refund incidents by changing order status.
- Do not reintroduce retired WeChat platform ecommerce or ordinary service-provider payment surfaces.
- Do not move business logic into HTTP handlers.
- Do not make frontend compute channel fees locally.
- Do not silently skip printing fee details for paid BaoFu profit-sharing orders.

## 10. Review Checklist For Each Phase

Before marking a phase complete, answer:

- Did this phase keep calculation separate from action?
- Did it preserve a single durable bill source?
- Did every unexpected failure either retry or fail visibly?
- Did every frontend-facing failure have stable Chinese guidance?
- Did logs include enough object IDs for diagnosis without sensitive leakage?
- Did this phase avoid provider DTO guessing?
- Did this phase avoid silent downgrade?
- Did it update tests before or with implementation?
- Did it run the smallest relevant validation command?
- Did it avoid reverting unrelated user work?

## 11. Completion Criteria

The full fix is complete only when all of these are true:

- Paid BaoFu main-business order creates a durable bill before merchant notification.
- Merchant new-order notification includes `fee_breakdown`.
- Paid-then-canceled order does not block merchant payment-success notification because the current order status is canceled.
- Rider acceptance updates and returns rider channel fee and net expected income.
- WeChat Mini Program settlement notification is the first valid trigger for BaoFu share execution.
- Duplicate WeChat settlement notifications are idempotent.
- Scheduler no longer performs first-time sharing action without a settlement trigger.
- Backend cloud print and Flutter Bluetooth receipt include settlement bill detail.
- Missing/invalid state fails visibly, logs structured context, and gives frontend stable Chinese guidance.
- `make check-baofu-contract`, generated checks, focused Go tests, and changed frontend validations have been run or explicitly documented as not run with concrete residual risk.

