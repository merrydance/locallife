# Baofu Main-Business Payment Review Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the reviewed Baofoo/Baofu main-business payment gaps so takeout, dine-in, takeaway, reservation, reservation add-on, and reservation replacement flows use Baofoo consistently and settle local truth from Baofoo terminal facts without silent downgrade.

**Architecture:** Keep Baofoo provider contracts under `locallife/baofu/**`; keep business orchestration in `locallife/logic/**`; keep state transitions in `locallife/db/sqlc/**`; keep callbacks and public request errors at API boundaries. Baofoo callback/query facts are the terminal truth for aggregate payment/refund status, but local domain state must not be marked applied until the matching local payment/refund/order/reservation state has been durably updated.

**Tech Stack:** Go, gin, pgx/sqlc, gomock, Asynq workers, Baofoo BaoCaiTong aggregate payment contracts.

**Risk class:** G3. This work touches payment, refund, provider callbacks, async fact application, idempotency, state-machine truth, and customer-facing payment recovery.

---

## Execution Status 2026-05-23

Status: implemented and validated in the working tree.

Finding 1 implemented:

- Reservation and reservation add-on Baofoo payment success facts now call `UpdatePaymentOrderToPaid` before `ProcessPaymentSuccessTx`.
- Duplicate success facts remain idempotent only when the local payment is already `paid`.
- Fact applications are not marked `applied` if mark-paid or reservation processing fails.

Finding 2 implemented:

- `reservation_addon` local Baofoo payment creation now treats the request as reservation-owned: `OrderID=0`, `ReservationID=<reservation.ID>`, `business_type=reservation_addon`, channel `baofu_aggregate`.
- `CreatePartnerPaymentTx` locks the reservation, verifies user ownership, allowed reservation status, merchant/payment-mode drift, positive add-on amount, and pending add-on conflict.
- Request-facing partner-payment errors are mapped to stable Chinese guidance.

Finding 3 implemented:

- Reservation replacement refund now builds Baofoo `RefundBeforeShareRequest` with refund `OutTradeNo`, original payment reference (`OriginTradeNo` or `OriginOutTradeNo`), notify URL, UTC `TransactionTime`, refund reason, and pre-share amount semantics.
- Baofoo `FAIL`, nil response, empty refund trade number, contract/provider errors, and create failures no longer move the local refund into processing.
- Rejected replacement refund command input now records provider `baofu`; public API error remains `退款提交失败，请稍后重试或联系平台处理`, while logs/command fields keep provider context.

Finding 4 implemented:

- Reservation-owned Baofoo `closed` and `failed` terminal payment facts now close/fail the local payment without success outbox.
- Same-terminal duplicate facts are idempotent.
- Conflicting terminal truth, for example Baofoo `closed` while local status is already `paid`, fails the application and logs `operation=apply_baofu_payment_terminal_failure` with payment/fact/application/business IDs.

Finding 5 implemented:

- `ReplaceOrderInput` now carries `ClientIP`.
- `/v1/orders/{id}/replace` passes `ctx.ClientIP()` into logic.
- Positive-delta replacement Baofoo payment fails fast before local/upstream create if client IP is missing, with public message `支付环境信息缺失，请刷新页面后重试` and structured logs.

Static route confirmation:

- `/v1/payments` builds `NewDefaultPaymentFacadeWithBaofuAggregate` when `BAOFU_MAIN_BUSINESS_ENABLED=true`.
- Reservation add/modify dishes use `PaymentFacade.CreatePaymentOrder` with `business_type=reservation_addon`.
- Reservation replacement positive delta uses the same facade and passes real request client IP.
- No direct WeChat fallback or combined payment support was introduced for main-business order/reservation payment. Retained direct WeChat paths remain outside this scope: rider deposit, claim recovery, and Baofoo verify fee.

Validation run from `locallife/`:

- `PATH=/usr/local/go/bin:$PATH go test ./logic -run 'BaofuReservationPaymentSuccess|BaofuReservationPaymentClosed|BaofuReservationPaymentFailed|ReservationPaymentSuccessCreatesOutbox|ReservationPaymentOutboxRetryAfterProcessed|BaofuOrderPaymentSuccessMarksPaidAndProcessesOrder|BaofuOrderPaymentClosedMarksPaymentClosedWithoutSuccessOutbox|BaofuOrderPaymentFailedMarksPaymentFailedWithoutSuccessOutbox|ReservationAddon|CreatePaymentOrder|ProcessReplaceOrderRefundWithBaofu|CreateReplaceOrderBaofuPayment|ReplaceReservationRefundCommandInputUsesBaofuProvider|ReplaceOrder' -count=1`
- `PATH=/usr/local/go/bin:$PATH go test ./db/sqlc -run 'CreatePartnerPaymentTx_ReservationAddon|CreatePartnerPaymentTx_ReservationPaymentModeMismatch|CreatePartnerPaymentTx_ReservationAmountChanged' -count=1`
- `PATH=/usr/local/go/bin:$PATH go test ./api -run 'ReplaceOrder|PaymentOrder|ReservationDishes' -count=1`
- `PATH=/usr/local/go/bin:$PATH make check-baofu-contract`
- `PATH=/usr/local/go/bin:$PATH make test-safety`
- `git diff --check`

Generated artifacts:

- `make sqlc`, `make mock`, `make swagger`, and `make check-generated` were not required because this change did not modify SQL query files, migrations, sqlc interfaces, mock interfaces, route definitions, or Swagger annotations. The touched `db/sqlc/tx_*.go` files are handwritten transaction glue.

Residual risk:

- Real Baofoo callback/query timing still depends on upstream samples, but local behavior now fails closed on malformed, missing, or contradictory terminal facts.
- Replace-order refund acceptance still follows Baofoo synchronous `SUCCESS` or unknown non-`FAIL` result as processing, matching existing pre-share refund behavior; final success/failure remains callback/query truth.

---

## Required Context

Read these first before implementing:

- `.github/copilot-instructions.md`
- `.github/README.md`
- `locallife/AGENTS.md`
- `.github/instructions/backend-locallife.instructions.md`
- `.github/instructions/review.instructions.md`
- `.github/prompts/backend-payment-domain.prompt.md`
- `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md`
- `.github/standards/domains/baofu-payment/README.md`
- `.github/standards/domains/baofu-payment/CONTRACT_IMPLEMENTATION_MAP.md`
- `.github/standards/domains/baofu-payment/BAOCAITONG_FIELD_CONTRACT_MATRIX.md`

Current reviewed state:

- Main `/v1/payments` order/reservation creation routes through `PaymentFacade` and `NewDefaultPaymentFacadeWithBaofuAggregate` when `BAOFU_MAIN_BUSINESS_ENABLED=true`.
- `payment_orders.payment_channel` for new main-business Baofoo orders should be `baofu_aggregate`.
- Combined payment is intentionally unsupported for Baofoo and must keep fail-closed split-checkout behavior.
- Retained direct WeChat paths exist for rider deposit, claim recovery, and Baofoo verify fee. Do not treat those as main-business order/reservation payment paths.

## Global Non-Negotiables

- Do not re-enable direct WeChat, WeChat platform ecommerce, ordinary service-provider, or combined payment for main-business order/reservation checkout.
- Do not mark `external_payment_fact_applications` as `applied` unless the local domain transition has completed or the local state is already in the same terminal truth idempotently.
- Do not swallow Baofoo contract errors, missing IDs, amount mismatches, unsupported terminal states, or state conflicts as success.
- Every unexpected provider, persistence, state-machine, enqueue, or idempotency error must have one structured log boundary with `payment_order_id`, `out_trade_no`, `business_type`, `reservation_id` or `order_id` when available, `fact_id` or `application_id` for fact paths, and a stable operation name.
- Public API errors must return stable Chinese business guidance. Do not expose raw SQL, pgx, Baofoo provider text, stack traces, or Go internal errors to frontend callers.
- Worker/fact errors should remain retryable only when retry can make progress. Permanent contract or invariant violations must log critical context and leave durable evidence for monitoring/manual repair; they must not be acknowledged as domain success.
- Use existing request error mapping patterns: `NewRequestError`, `NewRequestErrorWithCause`, `writeLogicRequestError`, `respondPaymentRequestError`, or local payment mapping helpers.
- Generated files under `locallife/db/sqlc/*.sql.go`, mocks, and Swagger must be regenerated only from source changes.

## Validation Baseline

Run from `locallife/` unless noted:

- Focused logic tests first, for example:
  - `go test ./logic -run 'PaymentFactServiceApplyExternalPaymentFactApplication_Reservation|CreatePaymentOrder_ReservationAddon|ReplaceOrder'`
  - `go test ./db/sqlc -run 'ProcessPaymentSuccessTx_Reservation|CreatePartnerPaymentTx_ReservationAddon'`
  - `go test ./api -run 'PaymentOrder|TableReservation|ReplaceOrder|BaofuCallback'`
- If SQL or sqlc interfaces change: `make sqlc` and `make check-generated`.
- If route/Swagger annotations change: `make swagger` and `make check-generated`.
- Before final handoff for this G3 package: `make test-safety` or, if unavailable/too broad locally, state exact skipped high-risk branches.
- For Baofoo contract-boundary changes: `make check-baofu-contract`.

---

## Finding 1: Reservation Baofoo Success Fact Is Applied Without Marking Local Payment Paid

### Background

`applyReservationPaymentFact` calls `ProcessPaymentSuccessTx` directly. `ProcessPaymentSuccessTx` returns without processing unless `payment_orders.status == 'paid'`. The order path first calls `markBaofuPaymentOrderPaid`; the reservation path does not. Because `ApplyExternalPaymentFactApplication` still terminalizes the fact and marks the application applied after a no-op, Baofoo success can be durably swallowed while reservation status, reservation payment ledger, and outbox are not updated.

### Boundary

- Fix only Baofoo main-business reservation payment facts: `business_type='reservation'` and `business_type='reservation_addon'`.
- Do not change retained direct WeChat rider deposit/claim recovery behavior.
- Do not make `ProcessPaymentSuccessTx` infer remote success on its own; the provider fact application layer owns translating Baofoo terminal success into local `paid`.
- Do not mark application applied if paid marking or reservation processing fails.

### Files

- Modify: `locallife/logic/payment_fact_application_service.go`
- Modify: `locallife/logic/baofu_payment_fact_application.go` if helper reuse is needed.
- Test: `locallife/logic/payment_fact_application_service_test.go`
- Optional integration test: `locallife/db/sqlc/tx_payment_success_test.go`

### Tasks

- [ ] Add a failing logic test named `TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentSuccessMarksPaidBeforeProcessing`.
  - Build a reservation application with `consumer=reservation_domain`, `business_object_type=payment_order`, `terminal_status=success`, `provider=baofu`, `channel=baofu_aggregate`, `capability=baofu_payment`.
  - Expect `UpdatePaymentOrderToPaid` before `ProcessPaymentSuccessTx`.
  - Expect `ProcessPaymentSuccessTx` returns `Processed: true`, `PaymentOrder.Status: paid`, and valid `ReservationID`.
  - Expect `CreatePaymentDomainOutboxOnce` with `PaymentDomainOutboxEventReservationPaymentSucceeded`.
  - Expected initial failure: missing `UpdatePaymentOrderToPaid` call.

- [ ] Add a failing idempotency test named `TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentSuccessAlreadyPaidStillProcesses`.
  - Make `UpdatePaymentOrderToPaid` return `db.ErrRecordNotFound`.
  - Expect fallback `GetPaymentOrder` returns status `paid`.
  - Expect `ProcessPaymentSuccessTx` still runs.
  - This preserves duplicate callback/query behavior without treating a non-paid state as success.

- [ ] Implement the reservation success path:
  - In `applyReservationPaymentFact`, after validation and before `ProcessPaymentSuccessTx`, when `isBaofuMainBusinessPaymentFact(fact)` and `fact.TerminalStatus == success`, call `svc.markBaofuPaymentOrderPaid(ctx, application, fact)`.
  - If mark-paid fails because the current row is not `paid`, return the error; let `ApplyExternalPaymentFactApplication` mark the application failed and retry or surface the invariant violation.
  - Thread the returned paid order only as context if needed; `ProcessPaymentSuccessTx` remains the authoritative domain processor.

- [ ] Add structured logging at the error boundary if current logs are not enough.
  - Preferred location: `markExternalPaymentFactApplicationFailed`, because it already logs application and fact context.
  - Ensure reservation failures include provider/channel/capability/external key/terminal status via existing fact fields.

- [ ] Verify with:
  - `go test ./logic -run 'BaofuReservationPaymentSuccess|ReservationPaymentSuccessCreatesOutbox|ReservationPaymentOutboxRetryAfterProcessed'`
  - `go test ./logic -run 'BaofuOrderPaymentSuccessMarksPaidAndProcessesOrder|BaofuOrderPaymentClosedMarksPaymentClosedWithoutSuccessOutbox'`

### Frontend/Error Semantics

This path is callback/worker-driven, not a direct frontend call. Frontend-visible recovery comes from querying payment/reservation state. If application fails, do not return a false paid state. Logs must carry enough context for operator/manual recovery, and query APIs must continue to show the local payment as not settled until processing succeeds.

### Acceptance

- Baofoo success fact for `reservation` and `reservation_addon` marks local payment `paid` before reservation domain processing.
- Application is not marked applied on mark-paid failure or reservation processing failure.
- Existing order success/closed/failed tests still pass.

---

## Finding 2: `reservation_addon` Creation Writes Reservation ID As Order ID

### Background

`createReservationAddonPaymentOrder` calls `PaymentFacade.CreatePaymentOrder` with `OrderID: reservation.ID` and `BusinessType: reservation_addon`. `createLocalBaofuPaymentOrder` only rewrites `OrderID` to `ReservationID` when `BusinessType == reservation`; for `reservation_addon`, it leaves `OrderID=reservation.ID`. `CreatePartnerPaymentTx` then locks `orders.id = reservation.ID`, causing add-on creation failure or wrong-object linkage if IDs collide. Downstream settlement/refund allocation expects `reservation_id` to be valid for add-on payments.

### Boundary

- Fix `reservation_addon` as a reservation-owned payment, not an order-owned payment.
- Keep `/v1/payments` DTO as order/reservation only unless product explicitly exposes add-on through that endpoint later. Current add-on creation is through reservation dish APIs.
- Do not allow partial or arbitrary amount for initial reservation payment. Only `reservation_addon` may use caller-provided positive delta amount.
- Do not infer add-on ownership from attach strings. Persist `payment_orders.reservation_id`.

### Files

- Modify: `locallife/logic/baofu_payment_order_route.go`
- Modify: `locallife/db/sqlc/tx_create_partner_payment.go`
- Test: `locallife/logic/payment_order_service_test.go`
- Test: `locallife/db/sqlc/tx_create_partner_payment_test.go`
- Possible mock regeneration: `locallife/db/mock/store.go`

### Tasks

- [ ] Add a failing service test named `TestPaymentOrderServiceCreatePaymentOrder_ReservationAddonCreatesReservationLinkedBaofuPayment`.
  - Arrange a paid/confirmed/checked-in reservation owned by the user.
  - Call `CreatePaymentOrder` with `BusinessType: reservation_addon`, `OrderID: reservation.ID`, `Amount: positive delta`, and non-empty `ClientIP`.
  - Expect `CreatePartnerPaymentTx` receives `OrderID: 0`, `ReservationID: reservation.ID`, `BusinessType: reservation_addon`, `PaymentChannel: baofu_aggregate`, and `RequiresProfitSharing: true`.
  - Expected initial failure: transaction receives `OrderID: reservation.ID`.

- [ ] Add a failing transaction test named `TestCreatePartnerPaymentTx_ReservationAddonLocksReservationAndCreatesReservationLinkedPayment`.
  - Use a real pending/paid reservation fixture with status allowed for add-on.
  - Call `CreatePartnerPaymentTx` with `BusinessType: reservation_addon`, `ReservationID: reservation.ID`, `OrderID: 0`, and amount > 0.
  - Assert `payment_orders.reservation_id` is valid and `payment_orders.order_id` is null.
  - Assert duplicate pending add-on payment for the same reservation returns conflict.

- [ ] Update `createLocalBaofuPaymentOrder`.
  - Treat both `businessTypeReservation` and `reservationAddonBusiness` as reservation-owned:
    - Use prefix `BFR`.
    - Set `orderID = 0`.
    - Set `reservationID = createInput.OrderID`.
  - Keep order payments unchanged.

- [ ] Update `CreatePartnerPaymentTx`.
  - Add a reservation branch for `arg.ReservationID > 0 && arg.BusinessType == "reservation_addon"`.
  - Lock reservation with `GetTableReservationForUpdate`.
  - Verify `reservation.UserID == arg.UserID`.
  - Verify allowed statuses: `paid`, `confirmed`, `checked_in` using existing constants where available.
  - Verify merchant and payment mode have not drifted when provided.
  - Verify `arg.Amount > 0`.
  - Check latest pending payment by reservation and `business_type='reservation_addon'` to enforce idempotent conflict.
  - Set `resolvedMerchantID = reservation.MerchantID`.

- [ ] Ensure `latestPaymentForBusiness` and `resolveConcurrentReservationPayment` continue using `GetLatestPaymentOrderByReservation` for add-on.

- [ ] Add or update request error mapping.
  - Add stable Chinese messages for add-on not payable, amount changed, merchant drift, and pending add-on conflict.
  - Avoid raw transaction error text in API responses.

- [ ] Regenerate and verify if store interface changes:
  - `make sqlc`
  - `make mock`
  - `make check-generated`

- [ ] Verify with:
  - `go test ./logic -run 'ReservationAddon|CreatePaymentOrder'`
  - `go test ./db/sqlc -run 'CreatePartnerPaymentTx_ReservationAddon|CreatePartnerPaymentTx_ReservationPaymentModeMismatch'`
  - `go test ./api -run 'ReservationDishes|PaymentOrder'`

### Frontend/Error Semantics

Reservation dish APIs must return actionable Chinese messages:

- Payment not ready because merchant Baofoo setup incomplete: `商户支付能力未完成配置，请联系平台处理后重试`
- Add-on amount invalid: `补差金额必须大于 0，请返回预订页面重新确认菜品`
- Reservation status no longer allows add-on: `当前预订状态不支持补差支付，请刷新后重试`
- Existing pending add-on payment: `已有待支付补差订单，请先刷新支付结果后再决定是否重试`

### Acceptance

- `reservation_addon` payment orders always persist `reservation_id`, never fake `order_id`.
- Add-on creation is idempotent around one pending add-on payment per reservation/business type.
- Refund allocation via `GetPaymentOrdersByReservation` can see paid add-on payments.

---

## Finding 3: Reservation Replacement Baofoo Refund Request Violates `order_refund` Contract

### Background

`processReplaceOrderRefundWithBaofu` sends `RefundBeforeShareRequest{OutTradeNo: outRefundNo}` but does not set `OriginTradeNo` or `OriginOutTradeNo`, does not set `TransactionTime`, and uses `TotalAmountFen: paymentOrder.Amount`. The Baofoo contract requires original payment reference and refund order number separately. Current normal refund paths correctly set refund `OutTradeNo` and original payment reference.

### Boundary

- Fix only reservation replacement refund orchestration.
- Do not change Baofoo contract DTO field meanings.
- Do not bypass `RefundBeforeShareRequest.Validate`.
- Do not mark local refund as processing if Baofoo request was not accepted.
- Do not use direct WeChat refund fallback.

### Files

- Modify: `locallife/logic/replace_order.go`
- Test: create `locallife/logic/replace_order_test.go` or extend existing order-service tests if replace-order helpers are already covered.
- Possibly modify: `locallife/logic/refund_error_mapping.go` if public error mapping lacks Baofoo replacement refund messages.

### Tasks

- [ ] Add a focused unit test named `TestProcessReplaceOrderRefundWithBaofuBuildsBaofooRefundWithOriginalPaymentReference`.
  - Use a fake `PaymentFacade` that records `RefundBeforeShareRequest`.
  - Use `paymentOrder.OutTradeNo = "BF202605230001"`, `paymentOrder.TransactionID = "BFPAY202605230001"`, `paymentOrder.Amount = 10000`, `refundAmount = 3500`, `outRefundNo = "R202605230001"`.
  - Expect request:
    - `OutTradeNo == outRefundNo`
    - `OriginTradeNo == paymentOrder.TransactionID`
    - `OriginOutTradeNo == ""`
    - `RefundAmountFen == refundAmount`
    - `TotalAmountFen == refundAmount` when no marketing refund info is present
    - `RefundReason` is the provided reason
    - `TransactionTime` is non-empty and in `yyyyMMddHHmmss`
  - Expected initial failure: original payment reference and transaction time missing, total amount mismatched.

- [ ] Add a second unit test named `TestProcessReplaceOrderRefundWithBaofuFallsBackToOriginOutTradeNoWhenTransactionIDMissing`.
  - Use no `TransactionID`.
  - Expect `OriginOutTradeNo == paymentOrder.OutTradeNo`.
  - Expect no direct WeChat fallback.

- [ ] Implement request building by matching `RefundService.processBaofuPreShareRefund`.
  - Set `OutTradeNo` to `refundOrder.OutRefundNo` or `outRefundNo`.
  - Set `NotifyURL` from `paymentFacade.BaofuRefundNotifyURL()`.
  - Set `RefundAmountFen` to requested refund amount.
  - Set `TotalAmountFen` to requested refund amount for pre-share non-marketing refund.
  - Set `TransactionTime` with current UTC time in `20060102150405`.
  - Set `RefundReason`.
  - Set `OriginTradeNo` from `paymentOrder.TransactionID` when present, else `OriginOutTradeNo` from `paymentOrder.OutTradeNo`.

- [ ] Handle returned Baofoo refund ID.
  - If `CreateBaofuRefund` succeeds, update local refund order to `processing` with returned `TradeNo` or refund ID field if available.
  - Record an accepted external payment command using existing helper.
  - If Baofoo returns nil or missing required response fields, mark refund failed, record rejected command, log error, and return stable request error.

- [ ] Add structured logs at failure points.
  - Include `payment_order_id`, `refund_order_id`, `out_refund_no`, `origin_out_trade_no`, `origin_trade_no_present`, `refund_amount`, `operation=replace_order_baofu_refund`.

- [ ] Verify with:
  - `go test ./logic -run 'ProcessReplaceOrderRefundWithBaofu|ReplaceOrder'`
  - `go test ./logic -run 'RefundServiceBaofu|MerchantRejectRefund'`
  - `make check-baofu-contract`

### Frontend/Error Semantics

When replacement refund cannot be initiated, API should return a stable Chinese message through the existing replace-order handler:

- Baofoo config missing: `宝付退款通道未配置，请联系平台处理`
- Baofoo rejects or contract validation fails: `退款提交失败，请稍后重试或联系平台处理`
- Funding chain changed: `预订退款资金链路已变化，请刷新后重试`

Raw Baofoo/provider validation details must be logged, not returned.

### Acceptance

- Replacement refund requests pass `RefundBeforeShareRequest.Validate`.
- Original payment reference and refund order number are never conflated.
- Local refund state changes only after Baofoo request acceptance.

---

## Finding 4: Reservation Baofoo Closed/Failed Terminal Facts Retry Forever Instead Of Closing Local Payment

### Background

`BaofuPaymentService.RecordPaymentFact` creates an application for any terminal Baofoo payment fact. Order-domain application accepts `success`, `closed`, and `failed`, and maps closed/failed to local terminal statuses. Reservation-domain validation only accepts `success`, so closed/failed reservation facts become failed applications and keep retrying. Local payment remains `pending`, which breaks terminal truth and recovery.

### Boundary

- Only handle Baofoo terminal `closed` and `failed` for reservation-owned payment orders.
- Do not create reservation payment success outbox for closed/failed.
- Do not update reservation status to paid for closed/failed.
- Do not turn unknown Baofoo terminal statuses into local success or local closed. Unknown remains an error and must be logged.

### Files

- Modify: `locallife/logic/payment_fact_application_service.go`
- Modify: `locallife/logic/baofu_payment_fact_application.go` if extracting shared terminal helper.
- Test: `locallife/logic/payment_fact_application_service_test.go`

### Tasks

- [ ] Add a failing test named `TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentClosedMarksPaymentClosedWithoutSuccessOutbox`.
  - Fact: `terminal_status=closed`.
  - Expect `UpdatePaymentOrderToClosed(application.BusinessObjectID)`.
  - Expect no `ProcessPaymentSuccessTx`.
  - Expect no reservation payment outbox.
  - Expect fact terminalized and application applied.

- [ ] Add a failing test named `TestPaymentFactServiceApplyExternalPaymentFactApplication_BaofuReservationPaymentFailedMarksPaymentFailedWithoutSuccessOutbox`.
  - Same as above, using `terminal_status=failed`.

- [ ] Add idempotent conflict tests.
  - When `UpdatePaymentOrderToClosed/Failed` returns `ErrRecordNotFound`, fallback to `GetPaymentOrder`.
  - If current status is already `closed`, `failed`, `paid`, or `refunded`, treat as idempotent only when it does not contradict a successful paid state.
  - If current status is still `pending`, return error and keep application failed/retryable.

- [ ] Update `validateReservationPaymentFactApplication`.
  - Accept `success`, and for Baofoo main-business payment facts also accept `closed` and `failed`.
  - Keep provider/channel/capability/object-type validation strict.

- [ ] Update `applyReservationPaymentFact`.
  - For Baofoo closed/failed, call a reservation-safe terminal failure helper.
  - Return `ApplyReservationPaymentFactResult{PaymentOrder: terminalPaymentOrder, Processed: false}`.
  - Do not call `ProcessPaymentSuccessTx`.

- [ ] Add logs for unexpected terminal conflicts.
  - Include `payment_order_id`, `fact_id`, `terminal_status`, `current_payment_status`, `business_type`, `operation=apply_baofu_reservation_payment_terminal_failure`.

- [ ] Verify with:
  - `go test ./logic -run 'BaofuReservationPaymentClosed|BaofuReservationPaymentFailed|BaofuOrderPaymentClosed|BaofuOrderPaymentFailed'`
  - `go test ./worker -run 'BaofuPaymentRecovery|PaymentFactApplicationScheduler'`

### Frontend/Error Semantics

Payment query/recovery should show local terminal `closed` or `failed`; frontend can prompt retry instead of seeing endless `pending`. Public text should remain business-level:

- `支付已关闭，请重新发起支付`
- `支付失败，请重新发起支付或更换支付方式`

Do not expose Baofoo raw `txnState`, `errCode`, or parser internals to frontend.

### Acceptance

- Reservation closed/failed Baofoo facts terminalize local payment and application exactly once.
- Unknown or malformed states still fail closed with logs.
- No success outbox is emitted for non-success terminal facts.

---

## Finding 5: Reservation Replacement Positive Delta Payment Sends Empty Baofoo Client IP

### Background

`createReplaceOrderBaofuPayment` calls `PaymentFacade.CreatePaymentOrder` with `ClientIP: ""`. Baofoo JSAPI unified order validation requires a non-empty risk-info client IP. Normal `/v1/payments` and reservation dish add/modify APIs pass `ctx.ClientIP()`.

### Boundary

- Fix reservation replacement positive-delta payment only.
- Do not weaken Baofoo `ClientIP` validation.
- Do not replace missing client IP with `127.0.0.1`, server IP, or another synthetic value.
- If the API cannot provide client IP, fail fast with a Chinese actionable message and structured logs.

### Files

- Modify: `locallife/logic/replace_order.go`
- Modify: `locallife/logic/order_service.go`
- Modify: `locallife/logic/interfaces.go` if `ReplaceOrderInput` changes.
- Modify: `locallife/api/order.go`
- Test: `locallife/api/order_test.go`
- Test: `locallife/logic/replace_order_test.go` or `locallife/logic/order_service_*_test.go`

### Tasks

- [ ] Add `ClientIP string` to `ReplaceOrderInput`.
  - Preserve existing fields.
  - Thread it from `api/order.go` using `ctx.ClientIP()`.
  - Thread it through `OrderService.ReplaceOrder` into `ReplaceOrder`.

- [ ] Add a failing API test named `TestReplaceOrderPassesClientIPToLogic`.
  - Build a request with remote address or forwarded header consistent with existing API test helpers.
  - Use a fake command service that captures `ReplaceOrderInput`.
  - Assert `input.ClientIP` is not empty.

- [ ] Add a failing logic test named `TestCreateReplaceOrderBaofuPaymentRequiresClientIP`.
  - Call positive delta replacement with empty client IP.
  - Expect a `NewRequestError`-style bad request or mapped service error with Chinese message.
  - Ensure no Baofoo create call is attempted.

- [ ] Update `createReplaceOrderBaofuPayment`.
  - Accept `clientIP string`.
  - Trim and validate it before calling `CreatePaymentOrder`.
  - Pass `ClientIP: clientIP`.

- [ ] Add structured logging at API or service boundary on missing client IP.
  - Include `user_id`, `order_id`, `reservation_id`, `operation=replace_order_baofu_payment`, and `reason=missing_client_ip`.

- [ ] Verify with:
  - `go test ./api -run 'ReplaceOrder'`
  - `go test ./logic -run 'ReplaceOrder|CreateReplaceOrderBaofuPayment'`
  - `go test ./logic -run 'BaofuPaymentService'`

### Frontend/Error Semantics

If client IP is missing, return:

- `支付环境信息缺失，请刷新页面后重试`

Log the technical reason. Do not tell frontend "client_ip required" or expose Baofoo risk-info internals.

### Acceptance

- Reservation replacement positive-delta payment sends real request client IP to Baofoo.
- Missing client IP fails before creating any local payment or upstream Baofoo order.
- Baofoo validation remains strict.

---

## Cross-Finding Safety Tasks

### Task A: Error Mapping And Logging Review

**Files:**

- Inspect/modify: `locallife/logic/payment_order_service.go`
- Inspect/modify: `locallife/logic/refund_error_mapping.go`
- Inspect/modify: `locallife/api/order.go`
- Inspect/modify: `locallife/api/table_reservation.go`
- Inspect/modify: `locallife/logic/payment_fact_application_failure.go`

- [ ] Ensure every new request-facing failure maps to a stable Chinese message.
- [ ] Ensure every worker/callback failure logs structured context once.
- [ ] Add tests for API semantic messages where the path is user-triggered:
  - reservation add-on payment config missing
  - replace-order payment client IP missing
  - replace-order Baofoo refund rejected
- [ ] Verify no raw Baofoo, SQL, pgx, or Go error text appears in user-facing JSON.

### Task B: End-To-End Static Route Confirmation

**Files:**

- Inspect: `locallife/api/payment_order.go`
- Inspect: `locallife/api/table_reservation.go`
- Inspect: `locallife/api/order.go`
- Inspect: `locallife/api/logic_adapters.go`
- Inspect: `locallife/main.go`

- [ ] Confirm `/v1/payments` order/reservation still uses `NewDefaultPaymentFacadeWithBaofuAggregate` under Baofoo main-business config.
- [ ] Confirm reservation add/modify dishes use `PaymentFacade.CreatePaymentOrder` with `business_type=reservation_addon`.
- [ ] Confirm replacement positive delta uses the same facade and passes client IP.
- [ ] Confirm no direct WeChat main-business fallback was introduced.
- [ ] Confirm combined payment stays unsupported and frontend capability response remains split-checkout.

### Task C: Final Validation And Handoff

- [ ] Run focused tests listed in each finding.
- [ ] Run `make check-baofu-contract`.
- [ ] Run `make test-safety` or state exact skipped G3 validations.
- [ ] Run `make check-generated` if any SQL/sqlc/mock/Swagger source changed.
- [ ] Summarize:
  - What changed.
  - What was validated.
  - What was not validated.
  - Residual risk by concrete branch: callback duplicate, recovery query, failed/closed terminal, add-on refund, replace-order refund, replace-order positive payment.

## Completion Criteria

This fix package is not complete until all are true:

- Initial reservation and reservation add-on Baofoo success facts mark local payment paid and process reservation domain state before application is applied.
- Reservation add-on payment orders persist `reservation_id` and never fake `order_id`.
- Reservation replacement refunds use Baofoo `order_refund` with correct original payment reference, refund order number, transaction time, and amount semantics.
- Reservation closed/failed Baofoo terminal facts close/fail local payment without endless retry.
- Reservation replacement positive-delta payments pass real client IP to Baofoo.
- All user-triggered errors have Chinese semantic frontend guidance.
- All worker/callback/provider errors have structured logs with IDs and operation names.
- No fallback to direct WeChat, combined payment, silent success, or manual-only downgrade was introduced for main-business payment flows.
