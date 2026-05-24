# WeChat Shipping Settlement Removal And Baofu Sharing Plan

> Status: supersedes the previous "upload WeChat shipping info and wait for settlement push" plan. The BaoFu technical confirmation is that LocalLife does not need to upload shipping information to WeChat for the current BaoFu aggregate-payment flow.

## Goal

Remove the WeChat Mini Program shipping-information upload and same-city delivery settlement-notify dependency from the active frontend/backend flow. Trigger BaoFu profit sharing from LocalLife-owned order completion semantics instead:

- User confirms receipt and the order enters `completed`.
- The auto-complete scheduler moves delivered takeout orders to `completed`.
- The BaoFu payment recovery scheduler backfills existing pending/failed BaoFu profit-sharing orders that are ready to be commanded.

## Decision

- Do not call WeChat `upload_shipping_info`.
- Do not expose or consume `/v1/webhooks/wechat-miniprogram/settlement-notify`.
- Do not rely on `trade_manage_order_settlement` / shipping-settlement pushes to release or trigger BaoFu profit sharing.
- Keep BaoFu merchant reporting and APPLET binding as the payment-channel compliance path; it does not imply LocalLife must also upload WeChat shipping information in this flow.
- Keep `wechat_transaction_id` only as a historical/payment-query display compatibility field. The Mini Program must not use it to launch WeChat's confirm-receipt component.

## Risk And Scope

- Risk: `G3`, because the change touches payment-adjacent completion semantics, profit-sharing trigger timing, async worker retries, scheduler recovery, and Mini Program order confirmation.
- In scope:
  - Remove active backend shipping-upload task, WeChat shipping client methods/contracts, and settlement-notify route/handler/tests.
  - Remove active Mini Program WeChat confirm-receipt component usage.
  - Trigger BaoFu profit-sharing command when a completed order has an existing ready BaoFu profit-sharing order and refund guard passes.
  - Let recovery enqueue existing ready `pending` / `failed` BaoFu profit-sharing orders without checking a WeChat settlement trigger.
  - Regenerate sqlc mocks and Swagger after query/interface/API-comment changes.
- Out of scope:
  - `weapp/demo-lab/**`, per product direction. It may retain historical demonstration references.
  - Historical artifact cleanup outside this plan.
  - BaoFu provider contract changes beyond command scheduling and recovery trigger timing.

## Implementation Plan

### 1. Backend Removal

- [x] Delete shipping-upload worker task and tests:
  - `locallife/worker/task_upload_shipping_info.go`
  - `locallife/worker/task_upload_shipping_info_test.go`
- [x] Delete WeChat shipping upload client/contracts/tests:
  - `locallife/wechat/shipping.go`
  - `locallife/wechat/shipping_test.go`
  - `locallife/wechat/contracts/order_shipping_message.go`
  - `locallife/wechat/contracts/shipping_settlement.go`
- [x] Remove worker distributor/processor/no-op/mock upload-shipping interfaces and registrations.
- [x] Remove rider-pickup `uploadShippingInfoAsync` enqueue path from `locallife/api/delivery.go`.
- [x] Remove Mini Program settlement-notify route and handler from `locallife/api/server.go` and `locallife/api/payment_callback.go`.
- [x] Remove `WECHAT_SHIPPING_SETTLE_NOTIFY_URL` config and examples.
- [x] Remove SQL query `CheckWechatSettlementTriggerForProfitSharingOrder`.

### 2. BaoFu Profit-Sharing Trigger

- [x] Change `TaskScheduler.ScheduleProfitSharing` to accept a BaoFu profit-sharing order ID directly.
- [x] Add `logic.ResolveCompletedOrderBaofuProfitSharingOrder` to find and validate a ready BaoFu profit-sharing order for a completed order.
- [x] Keep guardrails:
  - payment order exists and is paid
  - profit-sharing order provider is BaoFu
  - profit-sharing order status is `pending` or `failed`
  - `PaymentOrderRequiresProfitSharing` remains true
  - no `pending` / `processing` / `success` refund amount exists for the payment order
- [x] Trigger BaoFu profit-sharing command after user confirmation completes the order.
- [x] Trigger BaoFu profit-sharing command after takeout auto-complete completes the order.
- [x] Restore recovery scheduler fallback:
  - Create missing pending share orders through the existing recovery path.
  - Enqueue existing ready `pending` / `failed` BaoFu share orders through `ListBaofuProfitSharingOrdersReadyForCommand`.

### 3. Mini Program Removal

- [x] Replace active Mini Program confirm-receipt component flow with local confirmation modal plus backend `confirmOrder`.
- [x] Remove active `openBusinessView`, `weappOrderConfirm`, `pendingConfirmOrderId`, and `WECHAT_ORDER_CONFIRM_APPID` usage from `weapp/miniprogram/**`.
- [x] Stop passing `transactionId` from active order detail/tracking pages into confirm receipt.
- [x] Remove the active static check script for WeChat confirm-receipt component usage.
- [x] Keep the exported `confirmReceiptWithRecovery` name as a compatibility wrapper around the new local/backend flow.

### 4. Generated Artifacts And Docs

- [x] Run `make sqlc` after SQL/interface changes.
- [x] Run `make swagger` after route and API comment changes.
- [x] Run `make check-generated` before handoff.

### 5. Validation Plan

- [x] Focused backend tests already run during implementation:
  - `go test ./logic -run TestOrderServiceConfirmOrder_SchedulesBaofuProfitSharing -count=1`
  - `go test ./scheduler -run TestTakeoutAutoCompleteScheduler_AutoCompletesWithoutClaim -count=1`
  - `go test ./worker -run 'TestBaofuPaymentRecoverySchedulerRunOnceCreatesPendingShareAndEnqueuesCommand|TestBaofuPaymentRecoverySchedulerRunOnceCreatesReservationShareAndEnqueuesCommand|TestBaofuPaymentRecoverySchedulerRunOnceEnqueuesExistingPendingShare' -count=1`
- [x] Run focused package validation for changed backend packages.
- [x] Run `make check-generated`.
- [x] Run `make test-safety` because this is a `G3` funds-adjacent change.
- [x] Run active Mini Program validation from `weapp/`:
  - `npm run lint`
  - `npm run compile`

## Validation Results

- `go test ./logic ./scheduler ./worker ./api ./util -count=1`: passed.
- `go test ./worker -run 'TestProcessTaskBaofuProfitSharing|TestBaofuPaymentRecoverySchedulerRunOnce' -count=1`: passed, including the regression that a BaoFu share command is not created when a refund amount is already occupied.
- `make check-generated`: passed; generated sqlc, mocks, and Swagger are in sync.
- `make test-safety`: passed.
- `npm run lint` from `weapp/`: passed.
- `npm run compile` from `weapp/`: passed.
- `go test ./wechat -run 'TestCreateRefund_AcceptsProcessingResponseWithoutOptionalFields' -count=1`: failed with the pre-existing direct WeChat refund validation error `amount.total must be positive`; this is outside the removed shipping/settlement path and should be triaged separately before treating `./wechat` package-wide tests as a clean gate.
- Active frontend/backend residual scan excluding `weapp/demo-lab/**`: no old shipping-upload, settlement-notify, `openBusinessView`, or `pendingConfirmOrderId` references remain.

## Residual Risk To Watch

- BaoFu profit-sharing command execution itself remains provider-async and retry-driven; this plan changes when LocalLife enqueues the command, not BaoFu's final share result semantics.
- Duplicate enqueue is controlled by the worker task uniqueness window and provider/order status guards, but delayed duplicate tasks may still reach the worker and must remain idempotent there.
- Refund/share race protection is split across existing persistence guards: BaoFu share-order creation rejects existing occupied refunds, and BaoFu refund creation rejects existing `pending` / `processing` / `finished` share orders. The completion-time enqueue helper also checks occupied refund amount before scheduling.
- The direct WeChat refund unit-test failure noted above is unrelated to shipping/settlement removal but remains a package-level `./wechat` verification gap.
- Historical references under `weapp/demo-lab/**` and older `artifacts/**` are intentionally not part of the active-code cleanup.
