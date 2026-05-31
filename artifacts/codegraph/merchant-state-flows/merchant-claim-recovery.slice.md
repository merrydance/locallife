# Merchant Claim Recovery Slice

Status: merchant-state flow slice created; frontend/backend route contract repaired 2026-05-31
Risk class: G3 - claim compensation, recovery payment, WeChat direct payment facts, merchant takeout suspension/release, dispute review, async workers, and tenant-sensitive merchant/rider boundaries
Scope: merchant Mini Program claim/recovery/appeal pages -> merchant claim and recovery APIs -> claim recovery creation, dispute, overdue, payment, release, and notification paths

## Variant Coverage

This slice covers:

- Merchant Mini Program claim list, claim detail, recovery payment, and appeal/dispute list/detail pages.
- Merchant claim read routes under `/v1/merchant/claims/**`.
- Merchant recovery routes under `/v1/merchant/recoveries/:id` and `/v1/merchant/recoveries/:id/pay`.
- Merchant recovery dispute routes under `/v1/merchant/recovery-disputes/**`.
- Claim recovery creation after claim payout, overdue scheduler, payment order creation, WeChat direct payment callback/query facts, fact application, paid release action, and dispute auto-resolution.

This slice does not fully cover:

- Customer claim submission and customer-side claim pages except where they create the claim and payout preconditions.
- Rider recovery UI and APIs except where shared SQL/logic proves status semantics or route drift symmetry.
- Operator recovery dispute list/detail beyond the result-processing path shared with merchant disputes.
- Full WeChat direct payment provider signature details; those stay under payment-domain standards and payment callback slices.

## Product Invariant

Merchant claim recovery must converge from claim payout to exactly one merchant-visible recovery truth:

- A merchant sees only claims for orders owned by the resolved merchant.
- If a merchant owes recovery, the claim detail must be able to fetch the recovery row and pay the recovery using the backend's recovery id contract.
- Recovery payment must not mark the recovery paid until WeChat direct payment terminal truth is applied.
- Overdue recovery must generate a durable block action and suspend merchant takeout until all blocking recovery rows are cleared.
- A recovery dispute must atomically create the dispute and move a payable recovery to `disputed`; approval waives and releases, rejection resumes `pending/overdue`.
- Frontend tabs, DTO names, and status enums must match backend truth: `disputed`, `recovery_dispute_id/status`, and `claim_recoveries.status`.

## Primary Forward Chain

1. Merchant claim list loads summary and page data from backend, then hydrates the first visible rows with persisted behavior-decision data.
   Evidence: `weapp/miniprogram/pages/merchant/claims/index.ts:116`, `weapp/miniprogram/pages/merchant/claims/index.ts:127`, `weapp/miniprogram/pages/merchant/claims/index.ts:221`, `weapp/miniprogram/pages/merchant/claims/index.ts:242`.

2. The claim list tab model previously used `appealed`, while backend binding and SQL use `disputed`; this was repaired on 2026-05-31 for Mini Program merchant/rider claim lists.
   Evidence: `weapp/miniprogram/pages/merchant/claims/index.ts:53`, `weapp/miniprogram/pages/merchant/claims/index.ts:112`, `locallife/api/recovery_dispute.go:106`, `locallife/db/query/recovery_dispute.sql:135`.

3. Claim summary response returns `disputed`; the Mini Program merchant/rider types and pages now consume `disputed` with old `appealed` only as a compatibility fallback.
   Evidence: `locallife/api/recovery_dispute.go:218`, `locallife/api/recovery_dispute.go:404`, `locallife/api/recovery_dispute.go:415`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:288`, `weapp/miniprogram/pages/merchant/claims/index.ts:119`.

4. Claim list now reads backend `recovery_dispute_id/recovery_dispute_status` first, while old `appeal_id/appeal_status` remain only as compatibility fallback.
   Evidence: `weapp/miniprogram/pages/merchant/claims/index.ts:73`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:110`, `locallife/api/recovery_dispute.go:129`, `locallife/db/query/recovery_dispute.sql:109`.

5. Merchant claim detail loads claim detail, decision, recovery, appeal detail, behavior summary, and user-risk in parallel after the base claim read.
   Evidence: `weapp/miniprogram/pages/merchant/claims/detail/index.ts:126`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:127`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:128`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:129`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:130`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:131`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:132`.

6. Claim detail previously used claim id for recovery read and payment wrappers; Mini Program detail pages now use backend-provided `recovery_id`.
   Evidence: `weapp/miniprogram/pages/merchant/claims/detail/index.ts:129`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:332`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:475`.

7. The Mini Program recovery wrappers previously called claim-based paths that are not registered by the backend; merchant/rider shared wrappers now call `/v1/{role}/recoveries/:id` and `/pay`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:444`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:464`, `locallife/api/server.go:1040`, `locallife/api/server.go:1054`.

8. Backend merchant recovery read/pay handlers parse `ctx.Param("id")` as recovery id, not claim id.
   Evidence: `locallife/api/claim_recovery.go:94`, `locallife/api/claim_recovery.go:95`, `locallife/api/claim_recovery.go:107`, `locallife/api/claim_recovery.go:223`, `locallife/api/claim_recovery.go:224`, `locallife/api/claim_recovery.go:236`.

9. Merchant claim read routes allow owner/manager; recovery payment allows owner only.
   Evidence: `locallife/api/server.go:1027`, `locallife/api/server.go:1028`, `locallife/api/server.go:1031`, `locallife/api/server.go:1032`, `locallife/api/server.go:1047`, `locallife/api/server.go:1048`, `locallife/api/server.go:1051`.

10. Merchant claim list/detail SQL selects approved/auto-approved claims owned by the merchant and joins recovery dispute/recovery status.
    Evidence: `locallife/db/query/recovery_dispute.sql:101`, `locallife/db/query/recovery_dispute.sql:112`, `locallife/db/query/recovery_dispute.sql:123`, `locallife/db/query/recovery_dispute.sql:124`, `locallife/db/query/recovery_dispute.sql:180`, `locallife/db/query/recovery_dispute.sql:193`, `locallife/db/query/recovery_dispute.sql:197`, `locallife/db/query/recovery_dispute.sql:198`.

11. Merchant and rider claim detail responses now populate `recovery_id` and `recovery_status`, matching list semantics.
    Evidence: `locallife/api/recovery_dispute.go:464`, `locallife/api/recovery_dispute.go:498`, `locallife/db/query/recovery_dispute.sql:189`, `locallife/db/query/recovery_dispute.sql:192`.

12. Merchant claim decision is read-only and uses persisted behavior decisions; it does not re-run adjudication or create recovery side effects.
    Evidence: `locallife/api/recovery_dispute.go:501`, `locallife/api/recovery_dispute.go:542`, `locallife/api/recovery_dispute.go:545`, `locallife/api/recovery_dispute.go:551`, `locallife/api/recovery_dispute.go:562`.

13. Claim recovery rows are created after claim payout finalization when the persisted behavior decision says recovery is required.
    Evidence: `locallife/worker/task_claim_refund.go:457`, `locallife/worker/task_claim_refund.go:472`, `locallife/db/sqlc/tx_claim_behavior.go:571`, `locallife/db/sqlc/tx_claim_behavior.go:614`, `locallife/db/sqlc/tx_claim_behavior.go:749`, `locallife/db/sqlc/tx_claim_behavior.go:752`, `locallife/db/sqlc/tx_claim_behavior.go:759`, `locallife/db/sqlc/tx_claim_behavior.go:768`, `locallife/db/sqlc/tx_claim_behavior.go:775`.

14. Recovery creation also writes a behavior action with recovery id, target entity, amount, basis, and due date.
    Evidence: `locallife/db/sqlc/tx_claim_behavior.go:784`, `locallife/db/sqlc/tx_claim_behavior.go:792`, `locallife/db/sqlc/tx_claim_behavior.go:793`, `locallife/db/sqlc/tx_claim_behavior.go:795`, `locallife/db/sqlc/tx_claim_behavior.go:806`.

15. The recovery behavior-action worker validates the persisted recovery row and ensures recovery open events before marking the action successful.
    Evidence: `locallife/worker/task_claim_behavior_action.go:307`, `locallife/worker/task_claim_behavior_action.go:339`, `locallife/worker/task_claim_behavior_action.go:351`, `locallife/worker/task_claim_behavior_action.go:357`.

16. Claim recovery overdue scheduler runs every five minutes, scans due `pending` recoveries, marks them `overdue`, writes an event, creates a block action, and enqueues the behavior-action worker.
    Evidence: `locallife/worker/claim_recovery_scheduler.go:14`, `locallife/worker/claim_recovery_scheduler.go:88`, `locallife/db/query/claim_recovery.sql:131`, `locallife/db/sqlc/tx_claim_recovery_overdue.go:23`, `locallife/db/sqlc/tx_claim_recovery_overdue.go:27`, `locallife/db/sqlc/tx_claim_recovery_overdue.go:32`, `locallife/db/sqlc/tx_claim_recovery_overdue.go:51`, `locallife/worker/claim_recovery_scheduler.go:107`.

17. Merchant overdue block actions suspend merchant takeout using `merchant_profiles.is_takeout_suspended` fields.
    Evidence: `locallife/db/sqlc/tx_claim_recovery_overdue.go:89`, `locallife/db/sqlc/tx_claim_recovery_overdue.go:94`, `locallife/db/sqlc/tx_claim_recovery_overdue.go:99`, `locallife/worker/task_claim_behavior_action.go:243`, `locallife/worker/task_claim_behavior_action.go:277`, `locallife/worker/task_claim_behavior_action.go:278`, `locallife/db/query/trust_score.sql:77`.

18. Payment creation requires completed platform payout, merchant ownership, merchant recovery target, and payable recovery status.
    Evidence: `locallife/logic/claim_recovery_payment.go:50`, `locallife/logic/claim_recovery_payment.go:55`, `locallife/logic/claim_recovery_payment.go:59`, `locallife/logic/claim_recovery_payment.go:86`, `locallife/logic/claim_recovery_payment.go:89`, `locallife/logic/claim_recovery_payment.go:92`.

19. Claim recovery payment reuses an existing pending/paid payment order by business type and attach; expired pending orders are closed before creating a new order.
    Evidence: `locallife/logic/claim_recovery_payment.go:96`, `locallife/logic/claim_recovery_payment.go:101`, `locallife/logic/claim_recovery_payment.go:105`, `locallife/logic/claim_recovery_payment.go:106`, `locallife/logic/claim_recovery_payment.go:318`, `locallife/logic/claim_recovery_payment.go:346`, `locallife/logic/claim_recovery_payment.go:352`.

20. Payment creation writes a `payment_orders` row with `business_type='claim_recovery'`, WeChat direct pay params, external command audit, and a `payment_started` recovery event.
    Evidence: `locallife/logic/claim_recovery_payment.go:129`, `locallife/logic/claim_recovery_payment.go:135`, `locallife/logic/claim_recovery_payment.go:163`, `locallife/logic/claim_recovery_payment.go:184`, `locallife/logic/claim_recovery_payment.go:195`, `locallife/logic/claim_recovery_payment.go:196`, `locallife/logic/claim_recovery_payment.go:217`.

21. Mini Program recovery payment wraps the returned payment order into the shared payment workflow and reloads claim detail after sync.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/services/claim-recovery-payment.ts:36`, `weapp/miniprogram/pages/merchant/_main_shared/services/claim-recovery-payment.ts:52`, `weapp/miniprogram/pages/merchant/_main_shared/services/claim-recovery-payment.ts:58`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:490`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:480`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:481`.

22. WeChat payment callback/query facts for claim recovery are recorded with consumer `claim_recovery_domain` and business object `payment_order`.
    Evidence: `locallife/api/payment_callback.go:319`, `locallife/api/payment_callback.go:329`, `locallife/api/payment_callback.go:343`, `locallife/api/payment_callback.go:364`, `locallife/worker/direct_payment_fact.go:58`, `locallife/worker/direct_payment_fact.go:62`, `locallife/worker/payment_fact_application_scheduler.go:23`, `locallife/worker/payment_fact_application_scheduler.go:29`.

23. Claim recovery payment fact application validates WeChat direct terminal success and calls `ProcessPaymentSuccessTx`.
    Evidence: `locallife/logic/payment_fact_application_service.go:274`, `locallife/logic/payment_fact_application_direct_payment.go:35`, `locallife/logic/payment_fact_application_direct_payment.go:41`, `locallife/logic/payment_fact_application_direct_payment.go:80`, `locallife/logic/payment_fact_application_direct_payment.go:99`.

24. `ProcessPaymentSuccessTx` parses claim recovery attach, validates recovery id, marks payable recovery `paid`, writes a paid event, and creates a release action.
    Evidence: `locallife/db/sqlc/tx_payment_success.go:242`, `locallife/db/sqlc/tx_payment_success.go:247`, `locallife/db/sqlc/tx_payment_success.go:259`, `locallife/db/sqlc/tx_payment_success.go:263`, `locallife/db/sqlc/tx_payment_success.go:267`, `locallife/db/sqlc/tx_payment_success.go:280`, `locallife/db/sqlc/tx_payment_success.go:284`, `locallife/db/sqlc/tx_payment_success.go:294`.

25. Release action execution clears merchant/rider suspension only if no other blocking recovery remains, then writes a closed event.
    Evidence: `locallife/worker/task_claim_behavior_action.go:151`, `locallife/worker/task_claim_behavior_action.go:425`, `locallife/worker/task_claim_behavior_action.go:475`, `locallife/worker/task_claim_behavior_action.go:489`, `locallife/worker/task_claim_behavior_action.go:503`.

26. Merchant detail appeal submission posts `claim_id` and reason to the merchant appeal wrapper.
    Evidence: `weapp/miniprogram/pages/merchant/claims/detail/index.ts:432`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:443`, `weapp/miniprogram/pages/merchant/claims/detail/index.ts:444`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:361`.

27. The appeal wrapper previously posted `/v1/merchant/appeals`; merchant wrappers now use `/v1/merchant/recovery-disputes`.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:361`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:363`, `locallife/api/server.go:1041`.

28. Merchant appeal list/detail pages now rely on service calls backed by `/v1/merchant/recovery-disputes/**`.
    Evidence: `weapp/miniprogram/pages/merchant/appeals/index.ts:98`, `weapp/miniprogram/pages/merchant/appeals/index.ts:111`, `weapp/miniprogram/pages/merchant/appeals/detail/index.ts:185`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:321`, `weapp/miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts:350`, `locallife/api/server.go:1042`, `locallife/api/server.go:1043`, `locallife/api/server.go:1044`.

29. Backend recovery dispute create resolves the merchant, verifies claim/recovery ownership, checks the dispute window, rejects duplicates, creates the dispute, marks the recovery `disputed`, and auto-resolves best effort.
    Evidence: `locallife/api/recovery_dispute.go:670`, `locallife/api/recovery_dispute.go:683`, `locallife/logic/recovery_dispute.go:44`, `locallife/logic/recovery_dispute.go:53`, `locallife/logic/recovery_dispute.go:57`, `locallife/logic/recovery_dispute.go:63`, `locallife/logic/recovery_dispute.go:74`, `locallife/db/sqlc/tx_create_recovery_dispute.go:20`, `locallife/db/sqlc/tx_create_recovery_dispute.go:44`, `locallife/api/recovery_dispute.go:698`.

30. Auto-resolution derives approved/rejected from persisted behavior decisions. Approved disputes waive recovery and create release actions; rejected disputes resume `pending/overdue`.
    Evidence: `locallife/logic/recovery_dispute_auto_resolution.go:35`, `locallife/logic/recovery_dispute_auto_resolution.go:59`, `locallife/db/sqlc/tx_recovery_dispute_review.go:49`, `locallife/db/sqlc/tx_recovery_dispute_review.go:52`, `locallife/db/sqlc/tx_recovery_dispute_review.go:72`, `locallife/db/sqlc/tx_recovery_dispute_review.go:76`, `locallife/db/sqlc/tx_recovery_dispute_review.go:90`.

31. Recovery dispute result effects execute release/compensation or resume, then send appellant and claimant notifications; if inline dispatch fails, a retry task can process later.
    Evidence: `locallife/api/recovery_dispute.go:1719`, `locallife/api/recovery_dispute.go:1738`, `locallife/api/recovery_dispute.go:1763`, `locallife/api/recovery_dispute.go:1792`, `locallife/worker/task_process_recovery_dispute_result.go:103`, `locallife/worker/task_process_recovery_dispute_result.go:106`, `locallife/worker/task_process_recovery_dispute_result.go:125`, `locallife/worker/task_process_recovery_dispute_result.go:258`, `locallife/worker/task_process_recovery_dispute_result.go:313`.

## Reverse-Reference Findings

- Fixed 2026-05-31: Mini Program recovery read/pay wrappers now use recovery-id `/recoveries/:id` routes for merchant and rider paths.
- Fixed 2026-05-31: merchant and rider claim list/detail DTOs expose `recovery_id`, so detail pages do not need a claim-id compatibility recovery route.
- Fixed 2026-05-31: merchant/rider claim list bucket and summary mapping now use backend `disputed`.
- Fixed 2026-05-31: merchant/rider claim pages now read `recovery_dispute_id/status/reason` before old `appeal_*` compatibility fields.
- Fixed 2026-05-31: merchant/rider claim detail responses include `recovery_status` and `recovery_id`.
- Fixed 2026-05-31: merchant recovery-dispute wrappers now use `/v1/merchant/recovery-disputes/**`.
- Remaining: product should still decide whether a claim-id recovery compatibility route is desirable for external clients; no such route is required by the repaired Mini Program.
- Recovery dispute create route allows owner/manager because it sits in the read group. Product should confirm whether manager-submitted disputes are acceptable; payment is owner-only.
- Auto-resolution uses persisted behavior decisions and does not re-run adjudication in read paths, which is a healthy no-side-effect boundary.
- Release action is created in the same transaction as `paid`/`waived`, but executing the release side effect is async/retryable. A paid recovery can remain suspended until the behavior-action worker succeeds.

## SQL And Durable State Boundaries

- `claims`: claim lifecycle and payout precondition; approved/auto-approved claims are merchant-visible.
- `behavior_decisions`: persisted adjudication source for responsible party, compensation source, recovery target, and automatic dispute resolution.
- `behavior_actions`: durable work queue for payout, recovery open, block/suspend, release, and notify actions.
- `claim_recoveries`: recovery truth with `status` values including `pending`, `overdue`, `disputed`, `paid`, and `waived`.
- `claim_recovery_events`: audit/event trail for created, payable, payment_started, disputed, overdue, paid, waived, resumed, and closed events.
- `recovery_disputes`: merchant/rider dispute truth with `submitted`, `approved`, `rejected`, optional compensation fields, and region scope.
- `payment_orders`: claim recovery payment truth with `business_type='claim_recovery'` and attach containing claim/recovery ids.
- `external_payment_commands`: WeChat direct payment create command audit for claim recovery.
- `external_payment_facts` and `external_payment_fact_applications`: provider callback/query/recovery terminal truth and domain application queue.
- `merchant_profiles.is_takeout_suspended/takeout_suspend_reason/takeout_suspend_until`: merchant service-blocking state written by overdue recovery actions and released when clear.

## Trust, Authorization, And Tenant Checks

- Merchant claim read/recovery dispute routes require merchant staff owner or manager; payment route is owner-only.
- Handlers resolve the merchant server-side from the authenticated user and recheck merchant id against order ownership.
- Recovery read/pay logic checks `recoveryCtx.MerchantID == MerchantID`; pay also checks `recovery_target='merchant'`.
- Recovery dispute create checks claim ownership, claim recovery existence, dispute window, and duplicate dispute by claim/appellant type.
- Payment callback/query facts validate direct payment fact owner, object type, object id, terminal success, and recovery id inside the payment attach before marking recovery paid.
- Release actions do not trust frontend state; they reload claim recovery by claim id and compare expected recovery id before writing closed events.

## Idempotency And Duplicate-Submit Checks

- Claim and dispute list/detail GETs are idempotent reads.
- Mini Program pages have local loading/submitting/recoveryPaying guards, and the repaired wrappers can now reach backend recovery-id read/pay routes.
- Recovery payment creation reuses an existing active payment order by `business_type + attach` for the same payer; expired pending orders are closed before a new one is created.
- `ProcessPaymentSuccessTx` is idempotent for already-paid recovery rows and refuses status regression for non-payable states.
- Recovery overdue marking is conditional from `pending` to `overdue`; duplicate scheduler runs skip rows that are no longer pending.
- Recovery dispute create checks existing dispute before insert, but without a visible unique constraint in the traced SQL snippet, concurrent duplicate submissions should be verified at DB level.
- Recovery dispute result execution is partially idempotent through action status and conditional recovery status updates.

## Recovery And Async Convergence Paths

- Claim payout worker finalizes claim compensation and creates recovery rows/actions after platform payout succeeds.
- Behavior action worker ensures recovery open events and later block/release side effects.
- Claim recovery scheduler scans `pending` rows every five minutes and generates overdue block actions.
- WeChat direct payment callback records claim recovery payment facts and creates fact-application rows; payment fact application scheduler retries domain application every minute.
- Payment success marks the recovery paid and creates a release action; behavior-action worker clears merchant/rider suspension only if no blocking recovery remains.
- Recovery dispute create auto-resolves inline best effort; on failure it enqueues `recovery_dispute:automatic_resolution`.
- Recovery dispute result worker executes release/compensation/resume effects and sends appellant/claimant notifications.

## Frontend Draft And Backend Rehydration

- Claim list has no draft; it rebuilds rows from backend list and secondary decision hydration.
- Claim detail keeps local appeal reason draft, submits it through recovery-dispute routes, then reloads detail.
- Recovery payment now calls backend by `recovery_id`, runs shared payment workflow, locally reflects pending/success, then reloads detail.
- Appeal list/detail pages are read-only and preserve last trusted data on silent refresh failure; merchant service calls now use recovery-dispute routes.
- Backend detail returns recovery dispute fields, and Mini Program pages now read them before old appeal compatibility fields.

## Test Coverage Signals

Observed tests:

- `locallife/api/recovery_dispute_test.go` covers merchant/rider recovery dispute create/list/detail/summary, auto-resolution, removed operator review routes, and notifications.
- `locallife/logic/recovery_dispute_logic_test.go` covers claim/recovery ownership, duplicate disputes, window checks, and recovery mark-disputed failure propagation.
- `locallife/db/sqlc/recovery_dispute_tx_test.go` covers review transaction effects including waive/release and resume paths.
- `locallife/logic/claim_recovery_test.go` covers claim recovery read/pay owner checks, payable status gates, payment order reuse, expired rotation, and external command audit.
- `locallife/db/sqlc/tx_claim_behavior_test.go` covers recovery creation after payout and behavior effects.
- `locallife/db/sqlc/tx_payment_success_test.go` covers claim recovery payment success marking paid and release-action creation.
- `locallife/worker/task_claim_behavior_action_test.go`, `task_process_recovery_dispute_result_test.go`, and `task_automatic_recovery_dispute_resolution_test.go` cover block/release/dispute result workers.
- `locallife/api/payment_callback_test.go`, `worker/payment_recovery_scheduler_test.go`, and `logic/payment_fact_application_service_test.go` include claim recovery payment fact/application coverage.

Missing high-value tests:

- Fixed 2026-05-31: Mini Program API contract test now verifies merchant recovery routes use recovery id, not claim id.
- Fixed 2026-05-31: Mini Program contract test covers `disputed` bucket/status and recovery-dispute field usage in merchant pages.
- Fixed 2026-05-31: API tests prove merchant and rider claim list/detail expose recovery id/status.
- Remaining: backend compatibility or contract test deciding whether `/v1/merchant/claims/:id/recovery` should exist for external clients.
- Concurrent recovery-dispute duplicate submit test at SQL/transaction level.
- End-to-end claim recovery payment test from pay request -> WeChat fact -> paid recovery -> release action execution -> merchant takeout suspension cleared.

## Gaps And Refactor Notes

- Fixed 2026-05-31: Mini Program claim recovery wrappers use backend-supported recovery-id routes for merchant and rider.
- Fixed 2026-05-31: merchant/rider claim list/detail DTOs expose recovery id/status.
- Fixed 2026-05-31: merchant "appeal" UI remains product copy, but service routes now target backend recovery-dispute APIs.
- Fixed 2026-05-31: frontend tabs, summary DTOs, and recovery status display now use `disputed`.
- Fixed 2026-05-31: `recovery_status` and dispute fields are consumed consistently in claim detail response paths.
- Confirm whether managers may submit recovery disputes. If owner-only is required, split dispute creation from read group middleware.
- Add recovery-dispute DB uniqueness or a transaction-level duplicate guard if concurrent duplicate submission is possible.
- Make paid/waived release action retry status visible to merchant if suspension release is delayed.

## Branch Exhaustion

- Entry branches checked: Mini Program claim list, claim detail, recovery payment action, recovery appeal/dispute submit, merchant appeal list/detail pages, shared claim recovery payment workflow, and behavior-decision hydration. Flutter App has no claim/recovery/appeal feature entry in `merchant_app/lib/features/**`. Web/operator/rider paths are intentionally out of current merchant Mini Program/App scope except where shared backend state affects merchant truth.
- Request branches checked: merchant claim list/detail/summary/decision reads, recovery read/pay routes, recovery-dispute create/list/detail/summary routes, repaired former `/v1/merchant/appeals/**` Mini Program wrappers, repaired former claim-id recovery wrappers, WeChat direct payment creation, payment callback/query fact routes, and worker task inputs.
- Backend state branches checked: approved/auto-approved claim visibility, behavior-decision source, recovery row creation after payout, recovery open action, `pending/overdue/disputed/paid/waived` transitions, overdue merchant suspension, payment order reuse/expiry, WeChat terminal success application, dispute create/auto-resolution/review result, release/resume/compensation actions, and notification side effects.
- Async branches checked: claim payout worker, behavior-action worker, overdue scheduler, WeChat direct payment callback/query facts, payment fact application scheduler, automatic dispute-resolution task, dispute-result worker, and release-action retry behavior. Merchant UI recovery is reload/re-entry; no claim recovery realtime subscription was found.
- Failure/retry branches checked: broken frontend route/id mismatch before recovery payment reaches backend, existing pending payment order reuse, expired payment order close-and-recreate, duplicate scheduler runs, repeated payment facts, dispute auto-resolution failure enqueue, and release action failure delaying suspension clear.
- Reader/consumer branches checked: claim list tabs/summary, claim detail fan-out, recovery card/action state, appeal/dispute list/detail, merchant takeout suspension readers, payment order readers, and notification consumers.
- Authorization/tenant branches checked: owner/manager claim reads and dispute reads/creates, owner-only recovery payment, merchant ownership rechecks by order/recovery target, dispute window and duplicate checks, callback fact validation by business object, and release-action expected recovery id checks.
- Zombie/unreachable branches checked: frontend `appealed` bucket, Mini Program `/v1/merchant/appeals/**` API paths, and claim-id recovery read/pay wrappers were repaired on 2026-05-31; backend helper for claim-id context still exists but is not exposed as merchant compatibility API.
- Test-proof gaps checked: backend coverage exists for recovery dispute logic, claim recovery payment, payout/release workers, and payment facts. 2026-05-31 added Mini Program route/field contract coverage and backend list/detail recovery-id/status coverage. Missing proof remains for backend contract choice for claim-id compatibility, SQL-level concurrent duplicate disputes, and full pay -> fact -> release -> suspension-clear e2e.
