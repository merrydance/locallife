# Baofoo Account Opening Submit Wait Review

Date: 2026-06-04
Risk class: G3 - Baofoo account opening touches external provider calls, account state transitions, callbacks, schedulers, worker recovery, sensitive identity/bank data, merchant report, APPLET authorization, and Mini Program payment-adjacent UX.
Target areas: `locallife/` backend, `locallife/worker/`, Baofoo account-opening logic, and `weapp/` settlement-account submit/status pages.

> For future agents and engineers: this file captures the root cause, review findings, and corrected implementation plan for the Baofoo account-opening submit wait problem. Do not re-infer the design from one page handler or one service function. The goal is backend-truth transparency, not fake frontend progress.

## Problem

Mini Program Baofoo settlement account submission shows a static waiting page. The user sees neither a useful ticking wait timer nor backend intermediate state updates while the submit request is blocked.

Observed user-facing failure:

- User taps submit.
- Page opens a wait UI, but the timer does not visibly advance during the long backend request.
- Backend intermediate states such as `opening_processing` are persisted too late to be visible to the user during the blocking request.
- The user can only wait without knowing whether data was accepted, provider opening started, merchant report is pending, or the request is still blocked on a provider call.

## Root Cause

### Frontend

The submit page starts wait UI state before the API call, then awaits `startBaofuAccountOnboarding(...)`.

Relevant files:

- `weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.ts`
- `weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.ts`
- `weapp/miniprogram/pages/rider/settlement-account/submit/index.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/services/baofu-account-onboarding.ts`
- `weapp/miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-submit.ts`
- `weapp/miniprogram/pages/merchant/_components/baofu-onboarding-wait/index.wxml`

The visible countdown is driven by polling progress in `pollBaofuSettlementAccountStatus()` and `delayWithPollProgress()`. That code only runs after `submitBaofuSettlementAccountProfile(...)` returns. During a long `POST /settlement-account`, polling has not started yet, so the existing progress events do not update the timer.

The behavior layer already has reusable wait session helpers such as `_beginBaofuLongWaitSession()` and `_handleBaofuOnboardingProgress()`. The missing piece is a local wall-clock tick while the submit request itself is pending.

### Backend

The POST handler synchronously starts provider work:

- `locallife/api/baofu_settlement_account.go`
  - `createBaofuSettlementAccount()` calls `service.StartOrRecoverOpening(...)`.
- `locallife/logic/baofu_account_onboarding_service.go`
  - `StartOrRecoverOpening()` creates or recovers the profile/flow.
  - For non-user-fee paths, it continues into `openFromProfile()`.
- `locallife/logic/baofu_account_onboarding_open.go`
  - `openFromProfile()` persists `opening_processing`, upserts processing binding, then calls `accountClient.OpenAccount(...)` inside the same request.
  - It may then apply active/failed/abnormal result and continue into merchant report.
- `locallife/logic/baofu_account_onboarding_apply.go`
  - `ApplyAccountOpenResult()` can continue merchant report synchronously when configured.

This means the frontend is blocked on a synchronous provider chain before it can poll backend state.

### GET Side Effects

The settlement-account read path currently performs provider recovery:

- `locallife/api/baofu_settlement_account_read.go`
  - `tryRecoverBaofuSettlementAccountFlow(...)`
  - `tryRecoverBaofuSettlementAccountMerchantReportFlow(...)`

This makes GET non-read-only and can hide long provider calls behind a status read. The target design should make GET pure read once worker/scheduler paths are responsible for progress.

## Review Findings On The Initial Plan

1. Do not create a parallel durable task/outbox system, and do not reuse `payment_domain_outbox` for the first Baofoo `OpenAccount` command.
   - `payment_domain_outbox` is an event dispatcher with `published` / `failed` lifecycle and payment-domain event semantics. It is appropriate for publishing domain events, not for owning the first execution of an external provider command.
   - The durable source of truth for Baofoo account opening is already `baofu_account_opening_flows`: it stores owner, profile, state, `open_trans_serial_no`, `login_no`, provider request snapshot, failure fields, and recovery eligibility.
   - Corrected direction: keep `baofu_account_opening_flows` as the only durable state anchor. Add a narrow Asynq task that carries `flow_id` only and reloads persisted state before doing provider work. Do not add a second outbox table and do not put first-open command semantics into `payment_domain_outbox`.

2. Do not treat the existing Baofoo account-opening recovery scheduler as the first-open worker.
   - `locallife/worker/baofu_account_opening_recovery_scheduler.go` currently queries/recoveries flows already in provider progress.
   - `RecoverOpeningFlow()` uses `QueryAccount(...)`, not first-time `OpenAccount(...)`.
   - Corrected direction: introduce a first-open execution boundary, then continue using the existing recovery scheduler for later query/recovery.

3. Preserve rider/operator verify-fee behavior and its continuation after payment.
   - `StartOrRecoverOpening()` routes rider/operator through verify-fee payment first.
   - For unpaid verify fee, POST must continue returning the payment order and JSAPI pay params.
   - `ContinueAfterVerifyFeePaid()` currently performs `OpenAccount` synchronously from the payment fact application worker. The fix must change that continuation too; otherwise the original long provider call merely moves from POST to payment-fact processing and creates two first-open code paths.
   - Corrected direction: async first-open only after profile is complete and the flow is ready for account open. Do not bypass verify-fee pending/processing states. Payment success should mark or preserve `opening_processing` and enqueue the same first-open task.

4. Remove GET provider side effects only after async execution exists.
   - Current GET recovery is an unsafe but real production fallback.
   - Corrected direction: first wire a durable worker/scheduler path, then make GET pure read. Removing GET recovery first would strand processing flows.

5. Reuse frontend wait behavior instead of duplicating page timers.
   - Existing behavior files already centralize wait state and progress mapping.
   - Corrected direction: add submit-pending wall-clock ticking to the shared submit behavior and keep page files thin.

## Corrected Implementation Plan

### Final Audit Conclusion

The plan is cleared to implement only under these boundaries:

- Design goal: make Baofoo account-opening submission transparent and recoverable by persisting an intermediate backend state before provider work, returning HTTP quickly, and showing an honest elapsed timer while the frontend waits.
- Backend truth remains authoritative. The Mini Program may show local elapsed time during the submit request, but provider progress and terminal state must still come from GET polling.
- The implementation must not introduce a new durable command table, must not use `payment_domain_outbox` as a provider-command queue, and must not broaden the global `worker.TaskDistributor` interface for this single capability.
- The only new async boundary should be a narrow Baofoo account-opening task interface implemented by Redis and Noop distributors. API and payment-fact continuation should depend on that narrow interface.
- Existing `BaofuAccountOpeningRecoveryScheduler` and merchant-report recovery remain the recovery owners after provider progress begins. The new task starts the first `OpenAccount`; it must not duplicate query recovery or merchant-report recovery loops.
- GET may become pure read only after the async first-open path is wired. Removing GET provider recovery before that would strand flows.
- All owner roles are in scope: merchant, platform, rider, and operator backend flows; merchant, operator, and rider Mini Program submit behaviors. There is no platform Mini Program submit page to invent in this fix.
- Production safety gates: duplicate submits/tasks must reuse the same persisted `open_trans_serial_no` and `login_no`; enqueue failure must surface as a stable 503 instead of pretending progress was scheduled; provider errors must keep existing safe logging/error mapping and avoid leaking identity/bank/provider raw text.

### Backend Plan

1. Split Baofoo onboarding service responsibilities.
   - Keep profile validation, default merging, flow creation, active binding detection, and verify-fee payment creation in request-safe logic.
   - Move first-time `OpenAccount` and provider continuation out of the HTTP request path.

2. Add a prepare method or mode.
   - Suggested shape: a service method that persists profile/flow and returns the current state without calling Baofoo.
   - It should return:
     - `profile_pending` when required fields are missing.
     - `verify_fee_pending` or `verify_fee_processing` with pay params for rider/operator before payment.
     - `opening_processing` plus flow id when provider opening should begin asynchronously.
     - existing terminal/active states when already complete.

3. Reuse existing Asynq infrastructure without broad interface churn.
   - Add a narrow worker interface, for example `BaofuAccountOpeningTaskDistributor`, with one method to enqueue a `{flow_id}` payload.
   - Let `RedisTaskDistributor` and `NoopTaskDistributor` implement that narrow interface.
   - API and payment-fact continuation should type-assert to the narrow interface; do not add another method to the broad `TaskDistributor` interface or regenerate global worker mocks for unrelated OCR, recovery, risk, and order tests.
   - Use stable Asynq uniqueness keyed by `flow_id` so duplicate POST/payment-fact attempts converge on one queued first-open task.

4. Add a first-open worker execution path.
   - Payload should be identifier-only, preferably `flow_id`.
   - Worker should load the flow/profile and call an idempotent service method that performs `OpenAccount`.
   - Worker must be safe under duplicate delivery and retry.
   - Worker must no-op terminal states such as `ready`, `voided`, non-recoverable `failed`, and states that still need user payment.
   - Provider business failures should persist safe failure state and safe user-facing guidance. Provider transient failures should remain retryable or recoverable according to existing Baofoo error classification.
   - Worker retry must never generate a different `open_trans_serial_no` for the same flow. The prepare step must persist identifiers before enqueue; the worker must reuse those identifiers.

5. Keep existing recovery schedulers.
   - `BaofuAccountOpeningRecoveryScheduler` remains responsible for `opening_processing` query recovery and failed duplicate/not-found reconciliation.
   - Merchant-report recovery remains responsible for `merchant_report_processing` and `applet_auth_pending`.
   - Do not duplicate those recovery loops in the new first-open worker.

6. Make GET pure read after the async path exists.
   - Remove or gate `tryRecoverBaofuSettlementAccountFlow(...)` and `tryRecoverBaofuSettlementAccountMerchantReportFlow(...)` from GET.
   - GET should report persisted profile, binding, flow, payment, merchant report, and APPLET auth state only.
   - The response should continue exposing state labels/status descriptions from backend truth.

7. Keep POST contract compatible.
   - Continue returning `202 Accepted`.
   - For complete profile and no user-fee wait, return quickly with `opening_processing`.
   - For rider/operator needing verify fee, continue returning payment information as today.
   - For `profile_pending`, continue returning missing-field guidance.
   - If the flow needs provider opening but no Baofoo account-opening task distributor is available or enqueue fails, return `503` with stable user-facing text and structured logs. Do not return a fake `202` that leaves the flow without a worker owner.

8. Update verify-fee payment continuation.
- `PaymentFactService` should still own payment fact application and payment-order processing.
- After a successful rider/operator verify-fee payment, continuation should prepare/mark the opening flow and enqueue the same first-open task instead of synchronously calling Baofoo.
- If enqueue fails from the payment-fact worker, return an error so the payment fact application remains retryable by its existing scheduler.
- The HTTP server composition root must use the same async continuation. Do not inject `BaofuAccountOnboardingService` directly as a payment-fact continuation, or manual/callback-adjacent application paths can still call `OpenAccount` synchronously.
- The first-open worker must not treat a local `processing` binding as proof that Baofoo received `OpenAccount`. A worker can crash after writing a processing binding but before the provider call. Only terminal/active binding or a terminal flow may no-op; otherwise retry must reuse the persisted `open_trans_serial_no` and call the provider again.
- Asynq `Unique` duplicate enqueue for the same `flow_id` must be treated as idempotent success. A duplicate queued task means first-open execution is already scheduled; surfacing it as HTTP 503 or payment-fact failure would harm duplicate-submit and callback retry paths without improving safety.

### Frontend Plan

1. Add submit-pending wall-clock ticking in the shared submit behavior.
   - Use existing wait session id and cancellation semantics.
   - Timer starts immediately after the page commits `waitVisible=true`.
   - Timer must stop on hide/unload, terminal result, submit error, or manual session cancellation.

2. Keep polling backend truth after POST returns.
   - Do not fake provider progress.
   - Use existing `pollBaofuSettlementAccountStatus()` to update title/description/status from GET responses.
   - The local timer only expresses elapsed waiting time; backend status text remains server-driven.

3. Apply to merchant, operator, and rider submit pages.
   - Platform currently has a status page but no submit page under `weapp/miniprogram/pages/platform/finance/settlement-account/submit/`.
   - Do not invent a platform submit surface in this fix.

4. Strengthen the existing guard.
   - Current `weapp/scripts/check-baofu-onboarding-long-wait.js` is mostly static string checks.
   - Add a runtime-like check or helper test that simulates a never-resolving submit promise and asserts elapsed seconds can advance before polling starts.

## Production Impact Review

Expected safe behavior after the corrected plan:

- Merchant/platform-style account opening no longer blocks HTTP while waiting for Baofoo open/merchant report/provider continuation.
- Rider/operator verify-fee payment params remain available before payment.
- Existing Baofoo callbacks still apply provider facts and merchant-report continuation.
- Existing Baofoo account-opening recovery scheduler still catches stale `opening_processing`.
- GET no longer hides provider work and becomes predictable for Mini Program polling.
- Mini Program timer becomes honest elapsed time while backend status remains real server state.

Main production risks to test:

- Duplicate submit or duplicate queued task must not call `OpenAccount` with different `open_trans_serial_no`.
- Duplicate task retry after a local processing binding was written must still be able to call `OpenAccount` with the same persisted `open_trans_serial_no`; local processing binding alone is not a provider-delivery receipt.
- Worker retry after provider timeout must not create a second account-opening command with a new identity.
- Query recovery must still reconcile Baofoo duplicate/opening-not-found edge cases.
- Merchant report and APPLET binding must still proceed after active account opening.
- Rider/operator unpaid verify fee must not be pushed into `OpenAccount`.
- Existing readiness surfaces for merchant/operator/platform/rider must still read the same persisted state.

Broader audit result:

- Other production applications: the change is scoped to Baofoo settlement-account onboarding, one new narrow worker task, and three Mini Program submit behaviors. It should not affect normal order payment, refund, profit sharing, withdrawal, OCR, delivery, risk, onboarding review, or cloud print flows because the broad task interface and shared payment-domain outbox are not changed.
- Redundancy and maintainability: the plan reuses existing flow rows, state constants, provider request builder, binding upsert, Baofoo error mapping, recovery scheduler, merchant-report continuation, and Mini Program wait view helpers. New code should be limited to a prepare/execute boundary plus a small task wrapper.
- Maintainability guard: keep the existing onboarding service bounded. The prepare/execute async boundary belongs in a small same-package file instead of pushing `baofu_account_onboarding_service.go` past the backend file-size guardrail.
- Engineering standards: business orchestration stays in `logic/`, transport stays in `api/`, async execution stays in `worker/`, provider DTOs stay under `baofu/**`, and frontend state remains backend-contract-driven.
- Robustness: the design handles weak networks, duplicate submits, duplicate tasks, missing Redis/Noop distributor, provider timeout/retry, payment-fact retry, and read-path predictability.
- Security: sensitive identity/bank values remain encrypted in profile storage and only decrypted inside provider request construction; logs must keep existing sanitized provider error fields and must not include full provider request snapshots or raw decrypted data.

## Implementation Review Notes

Final implemented boundaries after self-review:

- `POST /settlement-account` now calls `PrepareOpening`, persists `opening_processing`, and enqueues a first-open worker task by `flow_id`.
- `GET /settlement-account` is pure read; provider recovery is owned by workers/schedulers.
- Rider/operator verify-fee continuation prepares the same flow and enqueues the same first-open task instead of synchronously calling Baofoo.
- The first-open task payload is identifier-only: `{ "flow_id": ... }`.
- Duplicate task enqueue is idempotent success under Asynq `Unique`; real enqueue infrastructure errors still surface as failures.
- `opening_processing` local binding is not treated as provider receipt. The worker retries `OpenAccount` with the persisted `open_trans_serial_no` and `login_no` unless an active binding or terminal flow exists.
- New logic is split into `baofu_account_onboarding_prepare.go` so the core service file remains under the 500-line backend guardrail.
- Mini Program submit pages start a local wall-clock tick before awaiting POST, then hand timer ownership to backend polling progress after POST returns.

## Validation Plan

Backend targeted tests:

```bash
cd /home/sam/locallife/locallife
go test ./api -run 'TestBaofuSettlementAccount' -count=1
go test ./logic -run 'TestBaofuAccountOnboardingService|TestBaofuAccountMerchantReportService|TestBaofuMerchantReportService' -count=1
go test ./worker -run 'Test.*BaofuAccountOpening|TestProcessTaskPaymentDomainOutbox|TestPaymentDomainOutboxScheduler' -count=1
```

Baofoo contract guard:

```bash
cd /home/sam/locallife/locallife
make check-baofu-contract
```

Generation expectations:

- Run `make sqlc` if SQL or sqlc query files change.
- Run `make mock` if store or worker interfaces used by mocks change.
- Run `make swagger` if public API annotations or request/response contracts change.
- Run `make check-generated` after SQL/API contract source changes.

Mini Program validation:

```bash
cd /home/sam/locallife/weapp
npm run check:baofu-onboarding-long-wait
npm run check:baofu-account-status-page
npm run compile
```

Broader safety checks to consider before merge:

```bash
cd /home/sam/locallife/locallife
make test-safety
```

## Current Working Tree Note

At implementation closeout, the only known unrelated untracked file is:

- `artifacts/merchant-application-ocr-correction-task-2026-06-04.md`

Do not stage, revert, or overwrite that file while submitting the Baofoo account-opening wait fix unless the user explicitly asks.
