# Payment Channel Removal Context

Last updated: 2026-05-22 Asia/Shanghai

## Goal

Hard-remove the legacy WeChat payment platform paths:

- `ordinaryserviceprovider`
- `ecommerce` / WeChat platform ecommerce / 平台收付通

Keep only:

- WeChat direct payment for rider deposits, recovery payments, merchant transfer compensation, and related direct refunds/callbacks.
- Baofu / BaoCaiTong as the main-business payment, refund, profit-sharing, account, withdrawal, and callback path.

Important user clarification:

- Do not delete platform advance compensation / 先行赔付 / 垫付业务 itself.
- The question was whether advance compensation depends on `ordinaryserviceprovider`. If it does not, remove `ordinaryserviceprovider` together with ecommerce.
- There is no historical data to preserve, so no cold-reserve compatibility or legacy-data migration is required.

## Working Rules For This Task

- After a context switch, read this file first instead of re-reading broad code.
- Treat this file as the source of truth for task background, scope decisions, completed batches, known blockers, and next commands.
- Subagents must also treat this file as their checkpoint target. If a subagent is blocked, waiting, or nearing context pressure, it must write a compact state update here before pausing.
- This file is authoritative only up to its latest update. If the thread advances after the last note and then compaction happens, any newer work that was not written here must be treated as tentative, not as preserved context.
- If a progress point is only in memory and not written here before compaction, treat it as lost and re-verify it from source after resume.
- If context pressure is rising and the current verified state is not yet written here, write a compact batch note before continuing. Do not assume unwritten progress will survive compaction.
- If the latest verified change is not written here before compaction, do not treat this file as post-compaction truth for that change. Re-read source and re-derive the state after resume.
- When the thread is nearing compaction, update this file immediately even if the batch is not fully finished yet. Partial but verified state is better than memory-only state.
- Do not leave subagents idle across a context reset. If they are waiting for instruction, either send the next bounded task, close them, or capture their current state in this file before the thread gets dense again.
- On every resume/compaction/handoff:
  - read this file before running any `rg`, tests, or file inspection;
  - read only the narrow routing files required by `AGENTS.md` if the session requires it;
  - continue from `Current Focus`, `Known Compile Errors`, and the latest `Batch Log` entry.
- Watch context pressure actively. When the session is getting close to compaction, or when you estimate the remaining usable context is roughly the last 10% of the current thread, write the current state into this file before continuing.
- If something only exists in chat memory and is not written here before compaction, treat it as unverified on the next resume and re-check the source before using it as progress.
- Prefer writing a batch update after any meaningful verification step, after a new blocker appears, before switching to another subsystem, and before the context gets crowded enough that the recent path might be lost.
- Do not wait for a perfect final summary if the thread is already getting dense; make a small durable update sooner so the next turn can resume cleanly. Treat this file as the recovery point, not the narrative.
- Do not rehydrate context by broadly scanning `locallife/api`, `locallife/logic`, `locallife/worker`, `locallife/scheduler`, or `locallife/wechat`.
- Use targeted `rg` only for exact symbols currently being removed or for compile errors.
- Update this file after each completed batch and after every important failed validation: what changed, remaining blockers, and next command.
- Do not revert unrelated dirty work already present in the tree.
- Do not reintroduce compatibility stubs for ecommerce or ordinary service provider.
- If a compile/test failure reveals a new work item, add it here before investigating deeper.

## Project Routing Already Checked

Read and applied:

- `AGENTS.md`
- `.github/copilot-instructions.md`
- `.github/README.md`
- `.github/instructions/backend-locallife.instructions.md`
- `.github/instructions/backend-api.instructions.md`
- `.github/instructions/backend-logic.instructions.md`
- `.github/instructions/backend-worker.instructions.md`
- `.github/instructions/backend-scheduler.instructions.md`
- `.github/instructions/backend-db-query.instructions.md`
- `.github/instructions/backend-db-sqlc.instructions.md`
- `.github/instructions/backend-wechat.instructions.md`
- `.github/instructions/backend-baofu.instructions.md`
- `.github/prompts/backend-payment-domain.prompt.md`
- `.github/standards/domains/wechat-payment/README.md`
- `.github/standards/domains/baofu-payment/README.md`

Risk class: `G3`, because this touches payment, refund, profit sharing, callbacks, workers, schedulers, config, and sqlc.

## Existing Dirty Work Not Ours

Do not revert these existing edits:

- `locallife/db/sqlc/tx_baofu_profit_sharing.go`
- `locallife/db/sqlc/tx_baofu_profit_sharing_test.go`
- `locallife/logic/payment_fact_application_service.go`
- `locallife/logic/payment_fact_application_service_test.go`
- `locallife/logic/rider_deposit_refund_service.go`
- `locallife/logic/rider_deposit_refund_service_test.go`
- `locallife/main.go`
- `locallife/main_test.go`
- `locallife/scheduler/data_cleanup.go`
- `locallife/worker/task_process_payment_mismatch_test.go`

Observed existing intent in those edits:

- `main.go` has already stopped constructing ecommerce and ordinary service provider clients at runtime.
- `runTaskProcessor` has already been changed to pass `nil` for ecommerce.
- production payment runtime validation has started moving toward Baofu-only.
- `data_cleanup.go` wording was broadened from WeChat-specific callback language to generic payment-channel callback language.

## Confirmed Current Problem Shape

Remaining references are broad, not just config:

- API routes and callbacks still register ecommerce and ordinary paths.
- `Server` still stores `ecommerceClient` and `ordinarySPClient`.
- `util.Config` still contains `WechatEcommerce*` and `WechatOrdinary*` fields, helpers, and validators.
- `wechat.EcommerceClientInterface` still exists and generated mocks still include it.
- `wechat/ordinaryserviceprovider/**` exists as a full bounded module.
- `internal/wechatruntime/ecommerce_client.go` and `ordinary_service_provider_client.go` still build old clients.
- `logic/payment_order_service.go` still has ecommerce and ordinary order creation, query, close, and signing paths.
- `logic/combined_payment_service.go` still has ecommerce and ordinary combined payment logic.
- `logic/refund_service.go` still has ecommerce and ordinary refund/profit-sharing-return/abnormal-refund logic.
- worker recovery/task code still has ordinary/ecommerce branches.
- `db/query/ecommerce_applyment.sql` and generated `db/sqlc/ecommerce_applyment.sql.go` still provide the legacy applyment state machine.
- `db/sqlc/constants.go` still exposes `PaymentChannelEcommerce` and `PaymentChannelOrdinaryServiceProvider`.

## Key Decisions

- Main-business payment channel should be Baofu aggregate only.
- Combined payment through old WeChat service-provider paths should be removed. Current Baofu combined payment behavior remains "not available / split payment" unless separately implemented.
- WeChat direct payment remains for rider deposit and compensation/recovery paths.
- Merchant settlement-account UI/API should point to Baofu settlement/account opening. Old WeChat ordinary settlement-account applyment paths should be removed.
- Old platform ecommerce fund management, withdrawal, subsidy, complaint, violation, cancel-withdraw, applyment, settlement-account, and receiver-lifecycle surfaces should be removed if they depend on ecommerce/ordinary.
- Advance compensation / payout to wallet should remain if it uses `TransferClientInterface` / direct merchant transfer, not ordinary service provider.
- `profitSharingReturn` is still part of the Baofu pre-share refund flow and must be kept; only its old WeChat channel naming is being removed.

## Suggested Removal Order

1. API/runtime entrypoints:
   - remove ecommerce and ordinary client fields/builders from `api/server.go`
   - remove old webhook routes:
     - `/v1/webhooks/wechat-ecommerce/*`
     - `/v1/webhooks/wechat-ordinary/*`
   - keep direct WeChat routes:
     - `/v1/webhooks/wechat-pay/notify`
     - `/v1/webhooks/wechat-pay/refund-notify`
     - `/v1/webhooks/wechat-pay/merchant-transfer-notify`
   - keep Baofu routes:
     - `/v1/webhooks/baofu/account/open`
     - `/v1/webhooks/baofu/withdraw`
     - `/v1/webhooks/baofu/payment`
     - `/v1/webhooks/baofu/share`
     - `/v1/webhooks/baofu/refund`

2. Logic facade and payment order:
   - remove ecommerce and ordinary constructors and client interfaces.
   - make main-business payment creation use `NewDefaultPaymentFacadeWithBaofuAggregate`.
   - remove old partner/ecommerce/ordinary creation/query/close helpers.
   - keep direct payment query/close/refund for direct-payment business types.

3. Refund/profit-sharing:
   - keep Baofu pre-share refund path.
   - keep direct refund path for rider deposits and direct payments.
   - remove ecommerce abnormal refund and old WeChat profit-sharing-return paths.

4. Worker/scheduler:
   - remove ordinary/ecommerce fields, setters, and branches.
   - remove applyment recovery and settlement verification schedulers.
   - remove old refund/profit-sharing recovery branches that query ecommerce/ordinary.
   - keep Baofu recovery and direct payment recovery branches.
   - keep compensation / claim payout if it uses direct transfer.

5. DB/sqlc:
   - remove `db/query/ecommerce_applyment.sql`.
   - remove generated `db/sqlc/ecommerce_applyment.sql.go` via `make sqlc`.
   - remove handwritten tx helpers only used by ecommerce/ordinary:
     - `tx_create_ecommerce_payment.go`
     - `tx_create_partner_payment.go` if no Baofu path uses it
     - applyment activation/notification helpers if no remaining caller.
   - remove old payment channel constants once callers are gone.
   - add a new migration to drop no-history legacy schema objects and enum values if required by current schema.

6. WeChat integration:
   - remove `wechat/ordinaryserviceprovider/**`.
   - remove `wechat/ecommerce.go` if no remaining caller.
   - remove ecommerce-only contract files under `wechat/contracts` after direct-payment imports are checked.
   - regenerate mocks without `EcommerceClientInterface`.

7. Config/docs/tests:
   - remove `WechatEcommerce*` and `WechatOrdinary*` config fields, helpers, validation, env example entries, and tests.
   - update `.github/standards/domains/wechat-payment/README.md` to state active WeChat scope is direct payment only.
   - update payment-domain prompt wording if it still presents ecommerce/ordinary as active implementation scope.
   - delete or rewrite tests whose only subject is removed legacy behavior.

## Files Most Likely To Touch Next

Start here; do not broad-scan unless compile errors point elsewhere:

- `locallife/api/server.go`
- `locallife/api/server_test_hooks.go`
- `locallife/logic/service_support.go`
- `locallife/logic/interfaces.go`
- `locallife/logic/payment_channel_boundary.go`
- `locallife/logic/payment_order_service.go`
- `locallife/logic/payment_order_query_wechat.go`
- `locallife/logic/combined_payment_service.go`
- `locallife/logic/refund_service.go`
- `locallife/worker/processor.go`
- `locallife/worker/refund_recovery_scheduler.go`
- `locallife/worker/profit_sharing_recovery_scheduler.go`
- `locallife/util/config.go`
- `locallife/Makefile`
- `locallife/wechat/interface.go`
- `locallife/db/sqlc/constants.go`
- `locallife/db/sqlc/store.go`
- `locallife/db/query/ecommerce_applyment.sql`
- `.github/standards/domains/wechat-payment/README.md`

## Current Focus

`api`, `logic`, and `worker` are green after the current cleanup batch. Continue outward to `scheduler`, then resume exact-symbol removal for remaining `ordinaryserviceprovider` / `ecommerce` references.

Live focus after the latest context shift:

- `locallife/integration/takeout_journey_integration_test.go` still contains old ecommerce-backed reservation and finance cases.
- `locallife/integration/finance_balance_integration_test.go` still uses `SetEcommerceClientForTest` and the removed ecommerce mock.
- `locallife/db/sqlc` compile blockers seen earlier may already be fixed by the restored direct-refund contract validation helpers, but they need a fresh test run before they can be treated as resolved.
- Fresh validation: `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./db/sqlc -count=1` passed.
- `locallife/integration/finance_balance_integration_test.go` was deleted and should stay deleted; its old ecommerce balance coverage has been replaced by the Baofu withdrawal-balance integration file.
- Keep direct WeChat paths for rider deposit / compensation / recovery.
- Keep Baofu as the main-business finance/payment provider.
- Latest exact-symbol scan on 2026-05-21 found remaining stale references concentrated in `locallife/integration/takeout_journey_integration_test.go`:
  - B1 integration and webhook cases still construct `MockEcommerceClientInterface`
  - B7 merchant-reject-refund still constructs `MockEcommerceClientInterface`
  - reservation cancel / deadline / refund-notify still construct `MockEcommerceClientInterface`
  - B3 still creates a direct `ecommerce` payment row for recovery coverage
  - the old combined-payment helper/webhook block is still present and should go
- After the latest edit pass, the targeted integration file no longer contains `ecommerce` / `ordinaryserviceprovider` symbols, but validation now fails earlier in `locallife/logic` due unrelated missing contract/errorcode symbols:
  - `wechatcontracts.CancelWithdrawQueryResponse`
  - `wechatcontracts.ProfitSharingQueryResponse`
  - `errorcodes.CanonicalCancelWithdrawCode`
  - `errorcodes.CancelWithdrawCodeParamError`
  - `errorcodes.CancelWithdrawCodeInvalidRequest`
  - `errorcodes.CancelWithdrawCodeNoAuth`
  - `errorcodes.CancelWithdrawCodeSignError`
  - `errorcodes.CancelWithdrawCodeAlreadyExists`
  - `errorcodes.CancelWithdrawCodeBizErrNeedRetry`
  - `errorcodes.CancelWithdrawCodeRateLimitExceeded`
  - This is now the next blocker for any integration test validation that imports `logic`.
- The latest integration edit converted `TestTakeoutJourneyB3Integration` from the deleted legacy `ProfitSharingRecoveryScheduler` to the active `BaofuPaymentRecoveryScheduler` and swapped the local capture helper to `worker.BaofuProfitSharingPayload`.

Last validation attempt:

```bash
cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api -count=1
```

Latest result before next batch:

- `go test ./logic -count=1` passes.
- `go test ./api -count=1` passes.

Current expected validation target:

```bash
cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic -count=1
```

Next exact command:

```bash
cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic -count=1
```

Expected next focus:

- If `api ./logic` remains green, run focused `go test ./worker` and `go test ./scheduler`.
- Fix worker/scheduler tests or compile blockers that still reference removed ecommerce/ordinary paths.
- Keep using exact-symbol `rg`; do not broadly rescan.

## Batch Log

- 2026-05-22: Migration hygiene correction after review feedback.
  - User flagged two migration hygiene issues:
    - historical migrations `000161_create_subsidy_orders.up.sql` and `000218_create_profit_sharing_receiver_lifecycle.up.sql` had been edited;
    - new migrations `000235_drop_legacy_wechat_platform_payment_surfaces.up.sql` and `000236_update_subsidy_order_comment.up.sql` existed without matching down files.
  - Corrected the migration shape:
    - restored `000161_create_subsidy_orders.up.sql` and `000218_create_profit_sharing_receiver_lifecycle.up.sql` to their historical HEAD content;
    - added `000235_drop_legacy_wechat_platform_payment_surfaces.down.sql`;
    - added `000236_update_subsidy_order_comment.down.sql`.
  - Verification:
    - `git diff -- locallife/db/migration/000161_create_subsidy_orders.up.sql locallife/db/migration/000218_create_profit_sharing_receiver_lifecycle.up.sql ...` returned no diff for the two historical files;
    - full migration pair check with `comm -23 <up list> <down list>` returned empty output.
  - Note: `000235` down recreates legacy schema/constraints for rollback shape, but documents that data dropped or rewritten by the up migration is not recoverable from the down migration.
- 2026-05-22: Subagent status recheck after UI still appeared to show running agents.
  - Rechecked the four known subagent IDs with `wait_agent`.
  - All four returned `not_found`, meaning they are no longer active/manageable in the current agent registry:
    - `019e4ac1-1be4-7591-8efc-d49ac3cf7d00`
    - `019e4ac1-1c15-74f3-a9f9-e417e71e85ba`
    - `019e4afa-79de-7852-b131-04befcc6cf54`
    - `019e4afa-7c8d-7063-a8a8-729da356bebe`
  - Current tool surface has no "list all agents" API; only known IDs can be waited/resumed/closed.
  - Treat the visible UI state as stale unless a new subagent ID appears in chat/tool output.
- 2026-05-22: Subagent status cleanup checkpoint.
  - User saw several agents as still running; checked all four listed subagents with `wait_agent`.
  - All four returned `completed`, not actively running. One old subagent final message contained a "Confirm I should proceed" question, but the agent state itself was completed.
  - Closed all four completed subagent sessions:
    - `019e4ac1-1be4-7591-8efc-d49ac3cf7d00`
    - `019e4ac1-1c15-74f3-a9f9-e417e71e85ba`
    - `019e4afa-79de-7852-b131-04befcc6cf54`
    - `019e4afa-7c8d-7063-a8a8-729da356bebe`
  - Do not treat those agents as pending work after any future compaction.
- 2026-05-22: Fresh `api/logic` validation after subagent cleanup.
  - Ran `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic -count=1`.
  - `logic` passed.
  - `api` failed at two current blockers:
    - `TestUpdateMerchantOpenStatus_RequireBaofuWhenOpen_NoPaymentConfig`: test did not expect the now-required `GetBaofuAccountBindingByOwner` readiness check.
    - `TestClosePaymentOrderAPI_BaofuServiceNotConfiguredReturnsStableChineseMessage`: expected Baofu-specific unavailable message, actual response is generic close-service unavailable message.
  - Next focus is targeted API test/root-cause cleanup, not broad symbol scanning.
- 2026-05-22: API local blockers fixed and verified.
  - `api/merchant_status_test.go`: added the Baofu account-binding readiness expectation to the open-status no-config test.
  - `api/payment_order.go`: changed `isPaymentServiceNotConfigured` so it does not swallow `logic.ErrBaofuPaymentServiceNotConfigured`; Baofu close/query errors keep the Baofu-specific public message path.
  - Focused API tests passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api -run 'TestUpdateMerchantOpenStatus_RequireBaofuWhenOpen_NoPaymentConfig|TestClosePaymentOrderAPI_BaofuServiceNotConfiguredReturnsStableChineseMessage' -count=1`
  - Full API package passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api -count=1`
- 2026-05-22: Core package validation checkpoint.
  - Passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./scheduler -count=1`
  - Next focus: exact-symbol scan for remaining `ordinaryserviceprovider`, `ecommerce`, `WechatEcommerce`, `WechatOrdinary`, `PaymentChannelEcommerce`, and `PaymentChannelOrdinaryServiceProvider` references.
- 2026-05-22: Exact-symbol scan checkpoint.
  - Active `api`, `logic`, `worker`, and `scheduler` package tests are green, and exact scans no longer show live old WeChat client/interface symbols in those packages.
  - Remaining live-ish cleanup candidates found:
    - `locallife/wechat/contracts/complaint.go` has `EcommerceComplaint*` type names, despite complaint contracts no longer being an active platform-ecommerce payment provider path.
    - AI routing/prompt docs still describe `platform-ecommerce` as an active payment-domain routing target.
    - `merchant_payment_configs` still exists as a legacy-named table/query/model, but Baofu merchant report code now writes it as a merchant WeChat `sub_mch_id` cache. Do not blindly delete it before replacing these callers:
      - `logic/baofu_account_merchant_report_service.go`
      - `logic/payment_fact_application_service.go`
      - `worker/refund_recovery_scheduler.go`
      - `worker/order_profit_sharing_snapshot.go`
      - `db/sqlc/tx_create_partner_payment.go`
      - `db/query/cart.sql` and `db/query/merchant.sql`
  - Generated Swagger and historical migration/artifact references still contain old wording; treat them separately from live runtime references.
- 2026-05-22: Low-risk naming/prompt cleanup checkpoint.
  - Renamed `wechat/contracts/complaint.go` contract types from `EcommerceComplaint*` to neutral `Complaint*`.
  - Updated AI routing docs so platform-ecommerce / ordinary-service-provider are described as retired removal/audit scope, not active implementation scope:
    - `.github/prompts/backend-payment-domain.prompt.md`
    - `.github/prompts/README.md`
    - `.github/instructions/backend-wechat.instructions.md`
    - `.github/standards/backend/ERROR_HANDLING.md`
    - `locallife/AGENTS.md`
  - Validation passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./wechat/contracts -count=1`
  - Current live-code exact scan for old client/channel symbols only shows:
    - generated sqlc comment in `db/sqlc/models.go` for `wechat_merchant_violations`
    - two negative assertion strings in `api/merchant_status_test.go`
  - Next: check for old route path strings such as `/wechat-ecommerce`, then decide whether docs/swagger regeneration or route deletion is needed.

- 2026-05-21: Integration state rechecked before any new code edits.
  - Fresh subagent review confirmed the B1 Baofu webhook path is already correct: the callback only records the payment fact and enqueues `ProcessTaskPaymentFactApplication`, and the worker is what advances the order to `paid`.
  - The current code path for refund terminalization still requires the refund-result application step; a refund order can legitimately remain `processing` until that async step runs.
  - Next step is to rerun the focused integration failures against the current tree and only change tests or helper setup if the failure still reproduces.
- 2026-05-21: Context-pressure and recovery checkpoint.
  - Working rule has been tightened in this file: if a verified progress point is not written here before compaction, it must be re-derived from source after resume and must not be treated as post-compaction truth.
  - B1 integration failure is now understood as a test expectation mismatch: `ProcessPaymentSuccessTx` already creates delivery and delivery pool for takeout orders after payment is marked paid, so the failing assertion at `takeout_journey_integration_test.go:1048` should be updated rather than the implementation.
  - B3 recovery failure is now understood as a recovery-fixture mismatch: the Baofu profit-sharing unique key is `fee_type + business_object_type + business_object_id`, so the test must not rely on creating the same fee ledger twice while validating recovery enqueue behavior.
  - Next command after the upcoming edit batch should be a focused integration run for the B1/B3 takeout journey cases, plus the narrow worker/scheduler tests if any helper changes spill into those packages.
- 2026-05-21: B1/B3 integration fixes landed and verified.
  - `TestTakeoutJourneyB1WebhookIntegration` now matches the actual Baofu webhook flow: after the worker applies the payment fact, the test only asserts the payment/order state progression and does not expect delivery creation inside the webhook-only slice.
  - `TestTakeoutJourneyB3Integration` now uses a clean recovery fixture again, and `resetIntegrationData` now truncates `baofu_fee_ledger`, preventing leftover ledger rows from tripping the unique index on rerun.
  - Focused validation passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./integration -run 'TestTakeoutJourneyB1WebhookIntegration|TestTakeoutJourneyB3Integration' -count=1`
- 2026-05-21: Wider integration sweep surfaced a separate B2 issue.
  - `TestTakeoutJourneyB2Integration` now fails at the rider location step with HTTP 400.
  - The failure is in `POST /v1/rider/location`, not in the Baofu removal work. The handler requires the rider to be online and the reported point to match the current active delivery; the current B2 fixture likely needs its delivery/active-order state refreshed before location reporting.
  - This is a new integration fixture issue to resolve next; it is not a regression from the payment-channel removal cleanup itself.

- 2026-05-21: Worker/scheduler legacy WeChat applyment cleanup batch.
  - Removed the dead ecommerce applyment outbox dispatch path from `locallife/worker/task_payment_domain_outbox.go`.
  - Removed the noop ecommerce applyment persistence hook from `locallife/worker/processor.go`.
  - Dropped applyment event types from `locallife/worker/payment_domain_outbox_scheduler.go` defaults.
  - Reworked `locallife/worker/task_payment_domain_outbox_test.go` to stop asserting retired applyment scheduler events and deleted the old applyment fixture builders.
  - Renamed the two claim-recovery direct-refund tests in `locallife/worker/task_process_payment_mismatch_test.go` to reflect the active direct-payment path.
  - Updated `locallife/worker/alert_payloads_test.go` to use the active Baofu aggregate payment channel instead of the retired ecommerce channel.
  - Renamed the leftover scheduler test that still mentioned an ecommerce client in `locallife/scheduler/operator_contract_expiry_scheduler_test.go`.
  - Self-review `rg` over `locallife/worker` and `locallife/scheduler` found no remaining `ecommerce`, `ordinaryserviceprovider`, or applyment-event references after the cleanup.
  - Validation attempted:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker ./scheduler -count=1`
  - Validation blocker:
    - Build fails in `github.com/merrydance/locallife/wechat/direct_payment.go` because `wechatcontracts.ValidateDirectRefundRequest`, `ValidateDirectRefundCreateResponse`, `ValidateDirectQueryRefundByOutRefundNoInput`, `ValidateDirectRefundQueryResponse`, and `ValidateDirectRefundNotificationResource` are undefined. This is outside the allowed `worker/` and `scheduler/` write scope, so I stopped there and reported it.
  - Next step:
    - Continue the broader payment-channel removal from the `wechat` package or wait for the upstream contract helpers to be restored, then rerun the worker/scheduler test command.

- 2026-05-21: Context-pressure rule tightened and cleanup is still in flight.
  - The working-memory file is now the canonical resume point for this task.
- 2026-05-21: Latest integration scan.
  - Confirmed the remaining ecommerce-era test code is now concentrated in `locallife/integration/takeout_journey_integration_test.go`.
  - Confirmed `locallife/integration/finance_balance_integration_test.go` is deleted and should not be resurrected.
  - Next step is to remove or rewrite the stale takeout/reservation cases against Baofu or direct-payment semantics.
- 2026-05-21: Latest validation pass.
  - `go test ./integration -run 'TestTakeoutJourneyB1Integration|TestTakeoutJourneyB1WebhookIntegration|TestTakeoutJourneyB7MerchantRejectRefundIntegration|TestReservationJourneyCCancelRefundIntegration|TestReservationJourneyCCancelAfterDeadlineIntegration|TestReservationJourneyCRefundNotifyIntegration|TestTakeoutJourneyB3Integration' -count=1` failed before running tests because `locallife/logic` now fails to compile on the missing cancel-withdraw / profit-sharing contract and errorcode symbols listed above.
- 2026-05-21: B3 integration retargeted.
  - Updated the B3 takeout integration test to use `BaofuPaymentRecoveryScheduler` and `BaofuProfitSharingPayload`.
  - Removed the legacy `ProfitSharingRecoveryScheduler` test helper from `locallife/integration/takeout_journey_integration_test.go`.
  - The active guidance is: write a compact batch note when the thread is approaching compaction or roughly the last 10% of usable context.
  - Subagents must checkpoint here before they pause on blockers, before they wait for instruction, and before they hand back a potentially dense state.
  - Current remaining cleanup is still centered on legacy `ecommerce` / `ordinaryserviceprovider` references in API, logic, worker, scheduler, db/sqlc, runtime clients, and domain docs.
  - Last known good validation before the newest deletion wave was:
    - `go test ./api -count=1`
    - `go test ./logic -count=1`
    - `go test ./worker -count=1`
  - Next step is to finish the remaining source cleanup, then rerun focused validation and record the exact result here.
- 2026-05-21: Cleanup still in progress, not finished.
  - The live tree still contains active `ecommerce` / `ordinaryserviceprovider` references in:
    - `locallife/wechat/ordinaryserviceprovider/**`
    - `locallife/wechat/contracts/**`
  - Current compaction rule: if the usable thread context is getting tight, write the newest concrete progress here before continuing. Do not rely on unrecorded recent reasoning surviving a later compression step.
  - Current immediate blocker: `locallife/wechat/direct_payment.go` still needs the direct-refund contract validators to be settled cleanly in `locallife/wechat/contracts/`, and `locallife/wechat/contracts/refund_validation_test.go` is still a scratch file that needs either consolidation or removal.
  - Latest validation notes:
    - `go test ./wechat/contracts -count=1` passes.
    - `go test ./db/sqlc -count=1` is currently blocked by migration state, not code compile errors.
  - Latest database blocker:
    - Test DB `schema_migrations` is dirty at version `235`.
    - The new `000235_drop_legacy_wechat_platform_payment_surfaces.up.sql` needs to be replayed cleanly after forcing the test DB back to version `234`.
    - Existing test data still contains legacy `ecommerce` / `ordinary_service_provider` payment channels, so the cleanup migration must normalize them before tightening constraints.
    - `db/sqlc` now gets past compilation, but migration-time tests fail on `profit_sharing_receiver_targets_channel_check` and `profit_sharing_order` recovery expectations, so the cleanup migration still needs to normalize that table and the related baofu-only assertions still need a pass.
    - The live `profit_sharing_receiver_targets` table is still constrained to `channel = 'ecommerce'`, so the cleanup migration needs a table-specific constraint update before the Baofu-only test data can be inserted.
    - The legacy profit-sharing receiver lifecycle SQL/table is now being removed as dead ecommerce-only surface, so the cleanup migration also needs to drop `profit_sharing_receiver_targets` and `profit_sharing_receiver_attempts` instead of trying to preserve a now-unused receiver lifecycle.
    - `locallife/internal/wechatdoc/**`
    - `locallife/db/sqlc/constants.go`
    - `locallife/db/sqlc/ecommerce_applyment.sql.go`
    - `locallife/db/query/refund_order.sql`
    - several `db/migration/*ecommerce*` files
  - This means the removal is not yet complete even though the core `api` / `logic` / `worker` cleanup batches have already landed.
  - Next step when resuming: delete or rewrite the remaining live contract/runtime/schema surfaces, then regenerate or validate the affected generated code.

- 2026-05-21: Logic/worker cleanup batch verified.
  - `locallife/logic/payment_fact_application_service_test.go` now includes `UpdatePaymentOrderToPaid` expectations for the Baofu order-payment success paths that were failing after the channel shrink.
  - `locallife/worker/task_process_payment.go` keeps merchant lookup only inside the `ABNORMAL` refund branch, so success callbacks no longer hit `resolveMerchantIDByPaymentOrder`.
  - `locallife/worker/task_process_payment_reservation_refund_test.go` now expects success-path refund updates instead of the retired skip behavior.
  - Verification passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker -count=1`

- 2026-05-21: Current batch narrowed to compile/test cleanup.
  - Renamed the duplicate Baofu refund fact test in `locallife/logic/payment_fact_application_service_test.go` to `TestPaymentFactServiceApplyExternalPaymentFactApplication_OrderBaofuRefundSuccessUsesBaofuChannel`.
  - Moved refund-result merchant lookup in `locallife/worker/task_process_payment.go` inside the `ABNORMAL` branch so success callbacks no longer trigger `GetOrder`.
  - Next validation:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker -count=1`

- 2026-05-21: Switched the remaining cleanup to subagent split to avoid context drift.
  - `locallife/logic/payment_fact_application_service_test.go` still has legacy ecommerce / ordinary fixtures mixed into otherwise current Baofu and direct-WeChat coverage.
  - `locallife/worker/payment_recovery_scheduler_test.go` and `locallife/worker/payment_channel_boundary_test.go` need current recovery / Baofu-boundary semantics.
  - `locallife/worker/task_upload_shipping_info_test.go` looks like stale combined-shipping coverage and may need to be deleted or rewritten to the active direct flow.
  - `locallife/worker/task_process_payment_reservation_refund_test.go` mainly needs fixture channel cleanup.
  - Next step: let focused subagents patch their owned files, then run package validation.

- 2026-05-21: Profit-sharing return boundary clarified.
  - `profitSharingReturn` is still part of the Baofu pre-share refund flow and should be kept.
  - Removed the duplicate `isBaofuMainBusinessProfitSharingFact` helper from `locallife/logic/payment_fact_application_service.go`; validation now reuses `isSupportedMainBusinessProfitSharingFact`.
  - Next command:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication_(ProfitSharing|Baofu)' -count=1`

- 2026-05-21: Worker cleanup batch in progress.
  - Deleted `locallife/worker/reservation_payment_fact.go`; it had no remaining callers after the Baofu order/refund path rewrite.
  - `locallife/worker/profit_sharing_fact.go` now records profit-sharing facts on `db.PaymentChannelBaofuAggregate` instead of the removed ecommerce/ordinary channels.
  - `locallife/logic/payment_command_service.go` no longer accepts ecommerce/ordinary as valid external payment channels.
  - `locallife/logic/payment_fact_application_service.go` now treats direct refunds as the only active WeChat refund fact and Baofu aggregate as the only active profit-sharing fact.
  - Next command:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker ./logic -count=1`

- 2026-05-21: Logic test cleanup is in progress.
  - Removed the obsolete `payment_fact_application_service_test.go` applyment / settlement / merchant-withdraw / merchant-cancel-withdraw test blocks and their helper builders.
  - The file now keeps only the active profit-sharing, rider-deposit, Baofu verify fee, order payment, and reservation payment coverage.
  - Next command:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic -count=1`

- 2026-05-21: Worker and scheduler validation passed after import cleanup.
  - `logic/payment_fact_application_service.go`: removed an unused `strings` import left behind by the prior refactor.
  - Validation passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker ./scheduler -count=1`
  - Next focus: continue removing the remaining ordinary/ecommerce references from API, logic, db/sqlc, and WeChat integration files.

- 2026-05-21: Continued worker/scheduler cleanup with fresh validation.
  - Validation failed immediately on `go test ./worker ./scheduler -count=1` because `logic/payment_fact_application_service.go` still had an unused `strings` import after the prior refactor.
  - Fixed the import residue; next command is the same targeted validation:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker ./scheduler -count=1`

- 2026-05-21: API stale expectation cleanup completed.
  - Renamed `TestApproveOperatorApplicationAdmin_WritesProfitSharingReceiverTargetIntent` to `TestApproveOperatorApplicationAdmin_CreatesOperatorAndRole` and removed the obsolete `GetUser` expectation tied to old receiver-target behavior.
  - Renamed `TestCreatePaymentOrderAPI_ServiceUnavailableWhenMainBusinessPaymentUnavailable` to `TestCreatePaymentOrderAPI_ServiceUnavailableWhenBaofuPaymentUnavailable`, added the current Baofu order route `GetMerchant` expectation, and asserted the active Baofu-service-unconfigured message.
  - Validation passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api -run 'TestApproveOperatorApplicationAdmin_CreatesOperatorAndRole|TestCreatePaymentOrderAPI_ServiceUnavailableWhenBaofuPaymentUnavailable' -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api -count=1`
  - Next command: `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic -count=1`

- 2026-05-21: Root package stale ecommerce/ordinary wiring cleanup completed.
  - `main.go`: removed unused `internal/wechatruntime` import and removed old nil ecommerce/ordinary arguments from `NewRefundRecoveryScheduler`, `NewDataCleanupScheduler`, and `NewRedisTaskProcessor`.
  - `main_test.go`: deleted removed ecommerce client builder tests; changed production ordinary-service-provider runtime test to assert rejection because Baofu main-business config is now required.
  - Validation passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker ./scheduler -count=1`
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test . ./util -count=1`
  - Next focus: remove config/env legacy ecommerce/ordinary fields and tests, then continue to db/sqlc and worker exact residuals.

- 2026-05-21: Config/env legacy payment surface cleanup completed.
  - `util/config.go`: removed all `WechatEcommerce*` and `WechatOrdinary*` config fields plus their effective URL helpers, runtime-config detectors, and validators.
  - `util/config_test.go`: removed ecommerce/ordinary config loading and validation tests; retained direct WeChat payment and merchant-transfer notify coverage.
  - `main_test.go`: replaced the old production ordinary-service-provider acceptance/rejection fixture with a direct-WeChat-only production fixture that still proves Baofu main-business config is required.
  - `app.env.example`: removed the old `WECHAT_ECOMMERCE_*` and `WECHAT_ORDINARY_*` env blocks.
  - Validation passed:
    - `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./util . -count=1`
  - Next focus: remove DB/sqlc legacy channel helpers and worker exact residuals.

- 2026-05-21: Fixed stale `OrderService.ReplaceOrder` call after `ReplaceReservationOrder` became the current Baofu-only wrapper. Next command remains `go test ./api ./logic`.

- 2026-05-21: Cleaned two now-empty legacy assertion files from `logic/`:
  - removed `payment_channel_boundary_test.go`
  - removed `payment_command_error_fields_test.go`
  The next `go test ./api ./logic` pass will show the next remaining stale test files.

- 2026-05-21: Continued Baofu-only cleanup after API/logic compilation drift.
  - Removed the obsolete `logic/combined_payment_service_test.go` legacy ecommerce/ordinary test suite and the dead worker ecommerce refund helper; restored the direct-refund helper needed by direct WeChat paths.
  - Validation failed on the next pass of `go test ./api ./logic`. Remaining compile blockers are now concentrated in stale logic tests:
    - `logic/merchant_reject_refund_test.go`: ecommerce/ordinary refund tests and constructors
    - `logic/operator_status_service_test.go`: outdated `NewOperatorStatusService` call signature
    - `logic/order_service_replace_test.go`: old ordinary-service-provider replace-order test
  - Next command: `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic`

- 2026-05-21: Started Baofu-only reservation add-on and replace-order cleanup.
  - Changed `PaymentOrderService` to accept `reservation_addon` with explicit `Amount`.
  - Changed `reservation_dishes.go` add-on payment creation to call `PaymentFacade.CreatePaymentOrder` instead of hand-building legacy ecommerce/ordinary combine payment records.
  - Added `AllowPartialOrderPay` to `CreatePartnerPaymentTxParams` so replacement-order delta payments can be represented without old service-provider code.
  - Validation failed:
    - `go test ./api ./logic`
    - Remaining compile blockers:
      - `logic/merchant_reject_refund.go`: stale `wechat.EcommerceClientInterface`, `paymentOrderUsesEcommerceChannel`, `createEcommerceRefundContract`, ordinary/ecommerce refund branches.
      - `logic/payment_fact_service.go`: missing `wechatcontracts` import.
      - `logic/rider_deposit_refund_service.go`: stale `wechat.EcommerceClientInterface`.
      - `logic/service_support.go`: missing/stale `ospcontracts` helper.
      - `logic/ecommerce_payment_order_errors.go`: stale removed helper `containsAny`.
  - Next command after fixing blockers: `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic`

- 2026-05-21: Re-ran routing and confirmed the remaining blockers from `go test ./api ./logic` are still the old reservation add-on / merchant reject / replace-order / payment-fact refund-creator entrypoints.
  - Current compile blockers before this batch:
    - `logic/reservation_dishes.go`: `wechat.EcommerceClientInterface`, `ordinaryServiceProviderCombineClient`
    - `logic/merchant_reject_refund.go`: `wechat.EcommerceClientInterface`
    - `logic/replace_order.go`: `wechat.EcommerceClientInterface`
    - `logic/payment_fact_service.go`: missing `wechatcontracts` import
    - `logic/payment_fact_application_service.go`: stale `refundCreator` / ecommerce / ordinary refund-creator branches
  - Next command after this batch: `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api ./logic`

### 2026-05-20 API runtime entrypoint batch

Touched:

- `locallife/api/server.go`
- `locallife/api/server_test_hooks.go`

Changed:

- Removed `Server` fields for `ecommerceClient` and `ordinarySPClient`.
- Removed `internal/wechatruntime` and `wechat/ordinaryserviceprovider` imports from `server.go`.
- Removed runtime construction of ecommerce and ordinary service-provider clients from `NewServer`.
- Removed old webhook routes:
  - `/v1/webhooks/wechat-ecommerce/payment-notify`
  - `/v1/webhooks/wechat-ecommerce/combine-notify`
  - `/v1/webhooks/wechat-ecommerce/refund-notify`
  - `/v1/webhooks/wechat-ecommerce/withdraw-notify`
  - `/v1/webhooks/wechat-ecommerce/applyment-notify`
  - `/v1/webhooks/wechat-ecommerce/profit-sharing-notify`
  - `/v1/webhooks/wechat-ecommerce/complaint-notify`
  - `/v1/webhooks/wechat-ecommerce/violation-notify`
  - `/v1/webhooks/wechat-ordinary/payment-notify`
  - `/v1/webhooks/wechat-ordinary/combine-notify`
  - `/v1/webhooks/wechat-ordinary/refund-notify`
  - `/v1/webhooks/wechat-ordinary/profit-sharing-notify`
  - `/v1/webhooks/wechat-ordinary/violation-notify`
- Kept direct WeChat routes:
  - `/v1/webhooks/wechat-pay/notify`
  - `/v1/webhooks/wechat-pay/refund-notify`
  - `/v1/webhooks/wechat-pay/merchant-transfer-notify`
- Kept Baofu routes:
  - `/v1/webhooks/baofu/account/open`
  - `/v1/webhooks/baofu/withdraw`
  - `/v1/webhooks/baofu/payment`
  - `/v1/webhooks/baofu/share`
  - `/v1/webhooks/baofu/refund`
- Removed route registration for old ecommerce/ordinary fund management, subsidy, receiver lifecycle repair, violation-management, merchant limitation, identity verification, and abnormal refund surfaces.
- Removed ecommerce and ordinary test hook setters from `server_test_hooks.go`.

### 2026-05-20 worker cleanup batch

Touched:

- `locallife/worker/refund_recovery_scheduler.go`
- `locallife/worker/refund_recovery_scheduler_test.go`
- `locallife/worker/task_payment_domain_outbox.go`
- `locallife/worker/task_payment_domain_outbox_test.go`
- `locallife/worker/task_payment_domain_outbox_rider_deposit_test.go`
- `locallife/worker/task_process_payment_mismatch_test.go`
- `locallife/worker/test_baofu_helpers_test.go`
- `locallife/worker/test_json_helpers_test.go`

Changed:

- Kept worker logic on direct WeChat + Baofu only.
- Removed stale legacy test expectations that still assumed ecommerce/ordinary branches.
- Preserved direct refund recovery, Baofu refund recovery, and rider-deposit fact-recording behavior.

Validation:

- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker`

### 2026-05-20 worker refund-recovery cleanup batch

Touched:

- `locallife/worker/refund_recovery_scheduler.go`
- `locallife/worker/refund_recovery_scheduler_test.go`

Changed:

- Removed legacy ordinary-service-provider and ecommerce refund recovery branches from the refund recovery scheduler.
- Kept direct refund recovery for rider deposits and Baofu refund recovery for main-business payments.
- Rebuilt the refund recovery scheduler tests to keep only:
  - pending reservation refund recovery
  - direct refund recovery alert/query-fact coverage
  - Baofu refund recovery query-fact coverage
- Updated `NewRefundRecoveryScheduler` call sites in the test file to the current 3-argument signature.

Validation status:

- `gofmt` was not available on PATH, so the files were not reformatted yet in this pass.
- Next command: run `/usr/local/go/bin/gofmt -w locallife/worker/refund_recovery_scheduler.go locallife/worker/refund_recovery_scheduler_test.go`, then rerun `/usr/local/go/bin/go test ./worker`.

Checks:

- Targeted `rg` for `ecommerce|ordinary|Ordinary|Ecommerce|wechat-ecommerce|wechat-ordinary|gateEcommerce` in `server.go` and `server_test_hooks.go` returned no matches.
- `gofmt -w locallife/api/server.go locallife/api/server_test_hooks.go` could not run because `gofmt` was not found in PATH.

Notes:

- Old handler implementations and tests still exist outside `server.go`; they are now unreachable from the route table and should be removed in later API cleanup batches once logic/client dependencies are cut.
- Do not re-add test hook setters; affected old tests should be deleted or rewritten to Baofu/direct-payment tests.

### 2026-05-20 active facade wiring batch

Touched:

- `locallife/api/logic_adapters.go`

Changed:

- Removed `wechat/ordinaryserviceprovider` import.
- `buildOrderCommandService` and `buildOrderQueryService` now construct `logic.NewOrderService` with only the direct WeChat client; ecommerce and ordinary-service-provider client slots are nil.
- `buildPaymentFacade` now returns:
  - Baofu aggregate facade when `BaofuMainBusinessEnabled` is true.
  - direct-payment-only default facade otherwise.
- `buildRefundOrchestrator` now builds a direct-payment-only default facade when Baofu is not enabled and a direct client exists.
- Removed `usesOrdinaryServiceProviderMainBusinessPayments` and `mainBusinessOrdinaryServiceProviderClient`.
- `buildProfitSharingReceiverLifecycleService`, `buildOperatorStatusService`, and `buildRiderDepositRefundService` no longer receive ecommerce clients.

Checks:

- `/usr/local/go/bin/gofmt -w locallife/api/server.go locallife/api/server_test_hooks.go locallife/api/logic_adapters.go`
- Targeted `rg` for removed server fields and old test hooks in `server.go`, `server_test_hooks.go`, and `logic_adapters.go` returned no matches.

Notes:

- `logic.NewOrderService` and `logic.NewDefaultPaymentFacade` still accept ecommerce/ordinary parameters internally; those will be removed in the logic batch after old handler/test compile references are cut.
- Old API handler files still reference `server.ecommerceClient` / `server.ordinarySPClient` and must be deleted or rewritten. Since their routes were removed, prefer deleting legacy-only handler files rather than adding compatibility fields.

### 2026-05-20 API legacy surface deletion batch

Touched:

- `locallife/api/merchant_finance.go`
- `locallife/api/platform_finance.go`

Deleted legacy-only API files/tests:

- `locallife/api/subsidy.go`
- `locallife/api/subsidy_test.go`
- `locallife/api/profit_sharing_capability.go`
- `locallife/api/profit_sharing_capability_test.go`
- `locallife/api/complaint.go`
- `locallife/api/complaint_test.go`
- `locallife/api/violation.go`
- `locallife/api/violation_test.go`
- `locallife/api/ecommerce_applyment.go`
- `locallife/api/ecommerce_applyment_test.go`
- `locallife/api/applyment_bank_catalog.go`
- `locallife/api/applyment_bank_catalog_test.go`
- `locallife/api/settlement_account.go`
- `locallife/api/settlement_account_test.go`
- `locallife/api/settlement_account_state.go`
- `locallife/api/merchant_cancel_withdraw.go`
- `locallife/api/merchant_cancel_withdraw_test.go`
- `locallife/api/merchant_cancel_withdraw_fact.go`

Changed:

- Removed old WeChat ecommerce merchant account balance and withdraw API surface from `merchant_finance.go`.
- Removed old WeChat ecommerce platform account balance query surface from `platform_finance.go`.
- Kept Baofu settlement status helpers in `platform_finance.go`.

Known remaining API cleanup from this point:

- `locallife/api/table_reservation.go` still references old ecommerce/ordinary clients through reservation add/modify/cancel paths.
- `locallife/api/payment_callback.go` and related callback fact/test files still contain legacy ecommerce/ordinary callback handling.
- Several legacy-only API tests still reference removed ecommerce/ordinary hooks and should be deleted or rewritten after handlers are removed.
- `locallife/api/settlement_account_error_handling.go` appears legacy-only and should be deleted after confirming no live caller remains.

## Validation Plan

After source edits:

1. `cd locallife && make sqlc`
2. `cd locallife && make mock` if interfaces changed and `make sqlc` did not cover all mocks
3. focused compile/test first:
   - `go test ./util ./wechat ./logic ./worker ./api`
4. then broader:
   - `make test-unit`
5. `make swagger` if routes or Swagger annotations changed.

## Current Next Step

Stop broad context gathering. Continue with a narrow logic facade batch:

- inspect only exact remaining `server.ecommerceClient`, `server.ordinarySPClient`, `SetEcommerceClientForTest`, and `SetOrdinaryServiceProviderClientForTest` references.
- delete or rewrite legacy-only API files/tests whose routes are already gone:
  - `subsidy.go` / `subsidy_test.go`
  - old ecommerce/ordinary sections of `payment_callback.go` and `payment_callback_ordinary_profit_sharing.go`
  - `complaint.go` / `complaint_test.go`
  - `violation.go` / `violation_test.go`
  - `ecommerce_applyment.go` / `ecommerce_applyment_test.go`
  - old ecommerce finance/cancel-withdraw/platform-finance/profit-sharing capability tests
- keep or rewrite settlement-account and applyment bank catalog routes to Baofu equivalents if they are still active; do not reattach ordinary service-provider clients.

### 2026-05-20 Baofu callback and reservation cancel cleanup batch

Touched:

- `locallife/api/baofu_callback.go`
- `locallife/api/merchant_finance.go`
- `locallife/logic/reservation.go`
- `locallife/logic/reservation_cancel_test.go`
- `locallife/worker/task_process_payment.go`

Changed:

- Added neutral Baofu callback helper `enqueueOrderPaymentFactApplication` in `api/baofu_callback.go`.
  - Nil fact returns nil.
  - Missing `taskDistributor` returns an error.
  - Distributor is cast to `worker.PaymentFactApplicationTaskDistributor`.
  - Enqueues `DistributeTaskProcessPaymentFactApplication` with `QueueCritical`, `MaxRetry(5)`, and `Unique(paymentFactApplicationTaskUnique)`.
- Restored `errors` import in `api/merchant_finance.go`.
- Removed ecommerce/ordinary service-provider parameters from `logic.CancelReservation`.
- Deleted `CancelReservationWithOrdinaryServiceProvider` and old direct calls to WeChat ecommerce/ordinary refund clients from reservation cancellation.
- Paid reservation cancellation now requires the payment order channel to be `db.PaymentChannelBaofuAggregate`, creates a pending refund order via `CreateRefundOrderTx`, and relies on async recovery/worker to submit the Baofu refund.
- Rewrote reservation cancel coverage to `TestCancelReservation_BaofuRefundCreatesPendingRefundOrder`.
- Updated `worker.processReservationRefund` to reject non-Baofu reservation refund orders and call `processBaofuAggregateRefund`.

Checks:

- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic -run TestCancelReservation_BaofuRefundCreatesPendingRefundOrder` passed.
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api` still fails on legacy API tests:
  - `api/ecommerce_applyment_status_contract_test.go`: removed ecommerce applyment capability helper.
  - `api/merchant_finance_test.go`: removed WeChat ecommerce withdraw/fund symbols.

Notes:

- `api/merchant_finance.go` no longer contains the old platform-ecommerce withdraw/account-balance surface; its stale tests should be deleted or rewritten against active Baofu finance/account paths.
- Do not reintroduce ecommerce/ordinary compatibility helpers to satisfy stale tests.

### 2026-05-20 API test cleanup batch

Touched:

- `locallife/api/payment_callback_test.go`
- `locallife/api/payment_order_test.go`
- `locallife/api/merchant_finance.go`

Changed:

- Removed stale payment callback tests for removed WeChat ecommerce applyment callback paths:
  - `TestHandleApplymentStateNotifyIdempotency`
  - `TestHandleApplymentStateNotify_AccountNeedVerifyRoutesToPendingFactApplication`
  - `TestHandleApplymentStateNotify_WithoutTaskDistributorReturnsFail`
  - `TestHandleApplymentStateNotify_EnqueueFailureReturnsFail`
  - `TestHandleApplymentStateNotify_FinishRoutesToFactApplication`
  - `TestHandleApplymentStateNotify_RejectedRoutesToFactApplication`
  - `TestHandleApplymentStateNotify_IgnoresOperatorApplymentAfterRemoval`
  - `TestResolveApplymentCallbackStatus`
- Removed stale payment callback tests for removed WeChat ecommerce profit-sharing callback paths:
  - `TestHandleProfitSharingNotify_WithoutTaskDistributorReturnsSuccess`
  - `TestHandleProfitSharingNotify_UnsupportedReceiverResultFallsBackToProcessing`
  - `TestHandleProfitSharingNotifyIdempotency`
- Removed unused old callback test helpers for ecommerce/partner/combined callback requests and ordinary applyment/profit-sharing contracts.
- Updated `payment_order_test.go` so the Baofu main-business create-payment test uses a normal test server plus `SetBaofuAggregateClientForTest`, with no ordinary-service-provider mock.
- Removed stale ordinary-service-provider and ecommerce payment order API tests:
  - ordinary main-business payment creation and idempotency paths;
  - ecommerce/ordinary payment query paths;
  - old service-provider combined payment query/close paths;
  - platform abnormal refund API coverage that depended on removed ecommerce abnormal refund surface.
- Kept active coverage for:
  - Baofu main-business payment creation;
  - Baofu provider-error mapping;
  - direct-payment query rejection;
  - Baofu split-checkout/combined-payment disabled capability.
- Removed unused `strings` import from `merchant_finance.go`.

Checks:

- `/usr/local/go/bin/gofmt -w locallife/api/payment_callback_test.go locallife/api/payment_order_test.go locallife/api/merchant_finance.go`
- Targeted `rg` found no ecommerce/ordinary client/test-hook references in:
  - `locallife/api/payment_callback_test.go`
  - `locallife/api/payment_order_test.go`
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./api` passed.
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic ./worker ./api` passed after rewriting the reservation addon refund worker test to Baofu aggregate.

Notes:

- `stubRefundOrchestrator.ApplyAbnormalRefund` remains only as a refund orchestrator interface stub method in API tests; it is not a WeChat ecommerce client dependency.

### 2026-05-20 worker reservation refund test batch

Touched:

- `locallife/worker/task_process_payment_reservation_refund_test.go`

Changed:

- Rewrote `TestProcessTaskInitiateRefund_ReservationAddonRefund_UsesProvidedOutRefundNo` from the old ecommerce refund path to the active Baofu aggregate refund path.
- Payment order test data now uses `db.PaymentChannelBaofuAggregate`.
- Removed `CreateEcommerceRefund` and merchant `SubMchID` expectations.
- Injected the existing fake Baofu aggregate refund client and asserted:
  - caller-provided `OutRefundNo` is reused;
  - Baofu request uses configured collect merchant/terminal and refund notify URL;
  - `TransactionID` is sent as `OriginTradeNo`;
  - refund amount and total amount both use the payload refund amount;
  - external payment command is recorded as Baofu refund with reservation business owner.

Checks:

- `/usr/local/go/bin/gofmt -w locallife/worker/task_process_payment_reservation_refund_test.go`
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker -run TestProcessTaskInitiateRefund_ReservationAddonRefund_UsesProvidedOutRefundNo` passed.
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic ./worker ./api` passed.

### 2026-05-20 worker scheduler legacy target cleanup batch

Touched:

- `locallife/worker/payment_fact_application_scheduler.go`
- `locallife/worker/payment_fact_application_scheduler_test.go`
- `locallife/worker/baofu_payment_recovery_scheduler.go`

Changed:

- Removed legacy `ordinaryserviceprovider` / ecommerce scheduler targets from `paymentFactApplicationSchedulerTargets`:
  - `applyment_domain` / `ordinary_service_provider_applyment`
  - `settlement_domain` / `ordinary_service_provider_applyment`
  - `settlement_domain` / `merchant_payment_config`
  - `merchant_funds_domain` / `withdrawal_record`
  - `merchant_funds_domain` / `merchant_cancel_withdraw_application`
- Kept active fact application scheduler targets for:
  - Baofu/direct profit sharing facts still modeled by `profit_sharing_domain`
  - direct claim recovery payments
  - direct rider deposit payments/refunds
  - Baofu account verify-fee payments
  - Baofu main-business order/reservation payments and refunds
- Added Baofu-owned `baofuShareRecoveryMinAge` and removed the Baofu recovery scheduler dependency on the deleted old WeChat profit-sharing scheduler constant.
- Updated the scheduler test to assert only the retained active targets.

Checks:

- `/usr/local/go/bin/gofmt -w locallife/worker/payment_fact_application_scheduler.go locallife/worker/baofu_payment_recovery_scheduler.go locallife/worker/payment_fact_application_scheduler_test.go`
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker -run TestPaymentFactApplicationSchedulerRunOnceEnqueuesConfiguredTargets` passed.
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./worker` passed.
- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./logic ./worker ./api` passed.

Next focus:

- Continue exact-symbol cleanup rather than broad rescans.
- Likely stale worker files still to remove after checking references:
  - `worker/task_merchant_withdraw_result.go`
  - `worker/task_merchant_cancel_withdraw_result.go`
  - `worker/task_profit_sharing_receiver_lifecycle.go`
  - `worker/applyment_support.go`
  - `worker/applyment_fact.go`
- Also inspect old outbox/distributor references to `DistributeTaskProcessProfitSharing`; main-business profit sharing should be Baofu path only.

### 2026-05-20 resume refund-runtime cleanup checkpoint

Touched:

- `locallife/internal/wechatruntime/refund_contracts.go`

Changed:

- Removed the ecommerce refund runtime bridge:
  - `CreateEcommerceRefundContract`
  - `ApplyEcommerceAbnormalRefundContract`
  - ecommerce refund response/request converter helpers.
- Kept only `CreateDirectRefundContract` in the runtime refund bridge.

Failed check:

- `cd /home/sam/locallife/locallife && /usr/local/go/bin/go test ./scheduler ./wechat ./internal/wechatruntime ./logic ./api`

Compiler blockers now:

- `internal/wechatruntime/refund_contracts_test.go` still tests removed ecommerce refund runtime helpers and the removed ecommerce mock.
- `logic` still has old `wechat.EcommerceClientInterface` dependencies in:
  - `combined_payment_service.go`
  - `order_service.go`
  - `payment_fact_service.go`
  - `payment_order_service.go`
  - `profit_sharing_receiver_lifecycle_service.go`
  - `profit_sharing_receiver_sync_service.go`
  - `reservation_dishes.go`
  - `service_support.go`

Next exact focus:

- Remove the stale ecommerce refund runtime test.
- Remove old ecommerce/ordinary service-provider constructor fields and interfaces from logic; keep direct WeChat and Baofu aggregate only.

### 2026-05-20 logic interface shrink checkpoint

Touched:

- `locallife/logic/interfaces.go`
- `locallife/logic/service_support.go`
- `locallife/logic/refund_helpers.go`
- `locallife/logic/payment_fact_service.go`

Changed:

- Removed `PaymentFacade` methods for:
  - ecommerce refund
  - ordinary-service-provider refund
  - ordinary-service-provider refund notify URL
  - ordinary-service-provider profit-sharing return
  - ecommerce abnormal refund
  - ordinary-service-provider MchID access
- Simplified `DefaultPaymentFacade` to keep only:
  - direct refund
  - Baofu refund
  - Baofu refund notify URL
  - direct/baofu payment services
  - stubbed `CreateProfitSharingReturn` returning unsupported
  - empty `SpMchID`
- Removed ecommerce refund helper wrappers from `refund_helpers.go`.
- Removed ecommerce refund creator injection and ordinary refund creator types from `payment_fact_service.go`.

Known follow-up:

- `logic` still contains many old ecommerce/ordinary implementation files and call sites, but the interfaces are now shrinking under them.
- Next validation should be the focused `go test ./logic` run to surface the remaining compile errors after the interface shrink.

### 2026-05-21 B1-B3 integration checkpoint

Verified:

- `locallife/integration/transfer_integration_test.go` now truncates `external_payment_commands`, `external_payment_facts`, `external_payment_fact_applications`, and `payment_domain_outbox`.
- `createIntegrationMerchant` now uses a unique address suffix instead of fixed `测试地址`.
- `TestTakeoutJourneyB3Integration` now includes Baofu readiness wiring for merchant, rider, and platform receivers.
- The B3 fixture is now aligned with the Baofu-only recovery path instead of the removed ecommerce path.

Validation status:

- Shared helper change passed its focused integration check in the subagent workspace.
- B3 passed its focused single-test check in the subagent workspace after the platform receiver helper was added.

Next command:

- Run the focused integration pair from the main workspace after formatting, then expand outward if both stay green.
