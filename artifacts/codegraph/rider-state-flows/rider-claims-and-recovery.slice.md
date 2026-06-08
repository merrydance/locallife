# Rider Claims And Recovery Slice

Status: rider-state flow slice created 2026-06-08
Risk class: G3 - rider claim visibility, persisted behavior decisions, recovery disputes, WeChat recovery payment, overdue sanctions, release actions, compensation
Scope: Mini Program rider claims list/detail -> rider claims/recoveries/recovery-disputes APIs -> claim recovery payment/dispute logic -> transactions/SQL -> payment facts/workers/schedulers/behavior actions -> dead/orphan paths

## Variant Coverage

This slice covers:

- Rider claims list/summary/detail page paths, including bucket filters, claim decision read, behavior summary, recovery detail, recovery payment, and dispute submission from the detail page.
- Backend rider claim routes `/v1/rider/claims/**`, rider recovery routes `/v1/rider/recoveries/**`, and rider dispute routes `/v1/rider/recovery-disputes/**`.
- Persisted behavior decision reads and behavior summary authorization for rider-owned delivery orders.
- Rider-target claim recovery read/pay, WeChat direct payment order creation/reuse, shared Mini Program payment workflow, generic payment query/detail recovery, terminal payment fact application, and release action.
- Rider recovery dispute creation, duplicate handling, auto-resolution, approved/rejected post-processing, compensation/release behavior actions, overdue scheduler, and rider suspension/release behavior effects.
- Resolved Mini Program appeal-wrapper drift and legacy rider damage risk worker.

This slice does not fully cover:

- Customer claim submission and the original behavior decision creation before rider-visible claim rows exist.
- Merchant/operator claim review UIs except where their shared recovery-dispute/result code affects rider post-processing.
- WeChat direct payment signature verification internals; this slice follows local payment fact application boundaries.

## Product Invariant

Rider claim/recovery state must be scoped to the current rider's deliveries and must never rerun adjudication from read APIs:

- Rider claim list/detail/decision reads must only return claims joined to deliveries assigned to the current rider.
- Decision and behavior summary endpoints are read-only over persisted behavior traces/decisions.
- Rider recovery get/pay requires recovery target `rider` and matching delivery rider id.
- Dispute creation marks the rider-target claim recovery `disputed`; approved disputes waive/release recovery and may compensate the rider; rejected disputes resume recovery.
- Overdue rider recoveries can suspend the rider through behavior actions, and paid/waived release actions must clear the sanction when no blocking recovery remains.

## Primary Forward Chain

1. Rider claims page loads summary, claim list, and recovery-dispute list concurrently; claim list uses `/v1/rider/claims`, while the appeal-compatible wrapper maps `/v1/rider/recovery-disputes` list response `disputes` back to the page's `appeals` view model.
   Evidence: `weapp/miniprogram/pages/rider/claims/index.ts:115`, `weapp/miniprogram/pages/rider/claims/index.ts:126`, `weapp/miniprogram/pages/rider/claims/index.ts:223`, `weapp/miniprogram/pages/rider/claims/index.ts:226`, `weapp/miniprogram/pages/rider/claims/index.ts:328`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:380`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:400`.

2. Rider claim detail loads claim detail first, then independently loads persisted decision, rider recovery, recovery-dispute detail, and behavior summary; partial failures are surfaced per panel.
   Evidence: `weapp/miniprogram/pages/rider/claims/detail/index.ts:96`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:120`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:124`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:125`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:126`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:127`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:128`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:137`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:142`.

3. Live rider claim wrappers call `/v1/rider/claims`, `/summary`, `/:id`, `/:id/decision`, `/behavior-summary`, `/recoveries/:id`, and `/recoveries/:id/pay`.
   Evidence: `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:481`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:491`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:574`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:599`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:613`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:627`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:638`.

4. Backend rider claim/recovery/dispute routes are registered under `/v1/rider`.
   Evidence: `locallife/api/server.go:1164`, `locallife/api/server.go:1165`, `locallife/api/server.go:1166`, `locallife/api/server.go:1167`, `locallife/api/server.go:1168`, `locallife/api/server.go:1169`, `locallife/api/server.go:1170`, `locallife/api/server.go:1171`, `locallife/api/server.go:1172`, `locallife/api/server.go:1173`, `locallife/api/server.go:1174`.

5. Rider claim list and summary resolve the current rider by authenticated user id and query claims by rider id plus bucket filters.
   Evidence: `locallife/api/recovery_dispute.go:944`, `locallife/api/recovery_dispute.go:951`, `locallife/api/recovery_dispute.go:952`, `locallife/api/recovery_dispute.go:959`, `locallife/api/recovery_dispute.go:973`, `locallife/api/recovery_dispute.go:1035`, `locallife/api/recovery_dispute.go:1037`, `locallife/api/recovery_dispute.go:1042`.

6. Rider claim detail and decision reload the claim through `GetRiderClaimDetailForRider`; not-found includes claims not joined to this rider's deliveries.
   Evidence: `locallife/api/recovery_dispute.go:603`, `locallife/api/recovery_dispute.go:610`, `locallife/api/recovery_dispute.go:616`, `locallife/api/recovery_dispute.go:621`, `locallife/api/recovery_dispute.go:1093`, `locallife/api/recovery_dispute.go:1100`, `locallife/api/recovery_dispute.go:1106`, `locallife/api/recovery_dispute.go:1111`.

7. Rider decision endpoint is a pure read: it lists persisted behavior decisions by order and returns nil when none exist; it does not rerun adjudication or create actions.
   Evidence: `locallife/api/recovery_dispute.go:629`, `locallife/api/recovery_dispute.go:632`, `locallife/api/recovery_dispute.go:638`, `locallife/api/recovery_dispute.go:643`.

8. Rider behavior summary verifies the order has a delivery assigned to the current rider before returning 30-day behavior summaries.
   Evidence: `locallife/api/behavior_summary.go:165`, `locallife/api/behavior_summary.go:167`, `locallife/api/behavior_summary.go:184`, `locallife/api/behavior_summary.go:194`, `locallife/api/behavior_summary.go:195`, `locallife/api/behavior_summary.go:200`.

9. Rider claim/recovery SQL scopes list/detail rows to rider-owned deliveries and carries recovery/dispute status into the list/detail response.
   Evidence: `locallife/db/query/recovery_dispute.sql:216`, `locallife/db/query/recovery_dispute.sql:251`, `locallife/db/sqlc/recovery_dispute.sql.go:751`, `locallife/db/sqlc/recovery_dispute.sql.go:823`, `locallife/db/sqlc/recovery_dispute.sql.go:1274`, `locallife/db/sqlc/recovery_dispute.sql.go:1415`.

10. Rider recovery detail resolves rider from user id, loads claim recovery by id, and logic verifies the claim belongs to the rider and target is rider.
    Evidence: `locallife/api/claim_recovery.go:137`, `locallife/api/claim_recovery.go:144`, `locallife/api/claim_recovery.go:150`, `locallife/api/claim_recovery.go:162`, `locallife/logic/claim_recovery_payment.go:66`, `locallife/logic/claim_recovery_payment.go:71`, `locallife/logic/claim_recovery_payment.go:75`.

11. Recovery payment requires payout already completed, recovery status `pending` or `overdue`, active WeChat openid, and direct payment client. It reuses an unexpired pending/paid payment order or closes an expired one before creating a new `payment_orders(business_type='claim_recovery')` row.
    Evidence: `locallife/api/claim_recovery.go:268`, `locallife/api/claim_recovery.go:275`, `locallife/api/claim_recovery.go:281`, `locallife/logic/claim_recovery_payment.go:82`, `locallife/logic/claim_recovery_payment.go:86`, `locallife/logic/claim_recovery_payment.go:89`, `locallife/logic/claim_recovery_payment.go:92`, `locallife/logic/claim_recovery_payment.go:101`, `locallife/logic/claim_recovery_payment.go:106`, `locallife/logic/claim_recovery_payment.go:115`, `locallife/logic/claim_recovery_payment.go:129`.

12. Recovery payment creates a WeChat JSAPI order, records accepted/rejected command audit, writes `payment_started` claim recovery event, and returns pay params for the Mini Program workflow.
    Evidence: `locallife/logic/claim_recovery_payment.go:155`, `locallife/logic/claim_recovery_payment.go:163`, `locallife/logic/claim_recovery_payment.go:172`, `locallife/logic/claim_recovery_payment.go:176`, `locallife/logic/claim_recovery_payment.go:184`, `locallife/logic/claim_recovery_payment.go:195`, `locallife/logic/claim_recovery_payment.go:196`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:421`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:441`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:457`.

13. The Mini Program claim recovery payment helper calls `completePaymentWorkflow`, which polls generic payment status and fetches payment detail; direct `GET /v1/payments/:id/query` can also record/apply query facts for `claim_recovery` through the same payment fact consumer as callback facts.
    Evidence: `weapp/miniprogram/pages/rider/_main_shared/services/claim-recovery-payment.ts:52`, `weapp/miniprogram/pages/rider/_main_shared/services/claim-recovery-payment.ts:58`, `weapp/miniprogram/pages/rider/_main_shared/services/payment-workflow.ts:181`, `weapp/miniprogram/pages/rider/_main_shared/services/payment-workflow.ts:218`, `weapp/miniprogram/pages/rider/_main_shared/services/payment-workflow.ts:245`, `weapp/miniprogram/pages/rider/_main_shared/api/payment.ts:576`, `weapp/miniprogram/pages/rider/_main_shared/api/payment.ts:583`, `locallife/api/server.go:1107`, `locallife/api/server.go:1108`, `locallife/api/payment_order.go:389`, `locallife/logic/payment_order_query_wechat.go:304`, `locallife/logic/payment_order_query_wechat.go:392`.

14. Payment success applies through the shared payment fact path: `ProcessPaymentSuccessTx` parses claim recovery attach, marks pending/overdue recovery paid, writes `paid` event, creates a release behavior action, and marks the payment order processed.
    Evidence: `locallife/logic/payment_fact_application_service.go:348`, `locallife/db/sqlc/tx_payment_success.go:247`, `locallife/db/sqlc/tx_payment_success.go:259`, `locallife/db/sqlc/tx_payment_success.go:267`, `locallife/db/sqlc/tx_payment_success.go:270`, `locallife/db/sqlc/tx_payment_success.go:280`, `locallife/db/sqlc/tx_payment_success.go:284`, `locallife/db/sqlc/tx_payment_success.go:294`, `locallife/db/sqlc/tx_payment_success.go:304`.

15. Rider dispute submit now calls `/v1/rider/recovery-disputes` through the appeal-compatible frontend wrapper, and the backend route maps to `createRiderRecoveryDispute`.
    Evidence: `weapp/miniprogram/pages/rider/claims/detail/index.ts:395`, `weapp/miniprogram/pages/rider/claims/detail/index.ts:406`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:428`, `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts:430`, `locallife/api/server.go:1172`, `locallife/api/recovery_dispute.go:1184`, `weapp/scripts/check-rider-claims-recovery-disputes-contract.test.js:96`.

16. Backend rider dispute creation validates current rider ownership, dispute window, duplicate dispute, and creates or returns an existing rider dispute. Creation transaction marks the matching rider-target recovery `disputed` and writes a `disputed` event.
    Evidence: `locallife/api/recovery_dispute.go:1191`, `locallife/api/recovery_dispute.go:1197`, `locallife/api/recovery_dispute.go:1212`, `locallife/logic/recovery_dispute.go:92`, `locallife/logic/recovery_dispute.go:95`, `locallife/logic/recovery_dispute.go:103`, `locallife/logic/recovery_dispute.go:107`, `locallife/logic/recovery_dispute.go:113`, `locallife/logic/recovery_dispute.go:121`, `locallife/logic/recovery_dispute.go:137`, `locallife/db/sqlc/tx_create_recovery_dispute.go:31`, `locallife/db/sqlc/tx_create_recovery_dispute.go:43`, `locallife/db/sqlc/tx_create_recovery_dispute.go:54`, `locallife/db/sqlc/tx_create_recovery_dispute.go:58`.

17. If the new dispute remains `submitted`, the API attempts automatic resolution immediately; failure enqueues `automatic recovery dispute resolution` retry when the task distributor is available.
    Evidence: `locallife/api/recovery_dispute.go:1217`, `locallife/api/recovery_dispute.go:1218`, `locallife/api/recovery_dispute.go:1219`, `locallife/api/recovery_dispute.go:1707`, `locallife/api/recovery_dispute.go:1767`, `locallife/api/recovery_dispute.go:1780`, `locallife/api/recovery_dispute.go:1785`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:52`.

18. Automatic retry worker reloads the dispute, resolves only `submitted` rows, writes audit, builds post-process payload, and executes the same result effects path.
    Evidence: `locallife/worker/task_automatic_recovery_dispute_resolution.go:58`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:72`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:73`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:81`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:83`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:119`, `locallife/worker/task_automatic_recovery_dispute_resolution.go:123`.

19. Dispute review transaction rejects by resuming recovery and writing `resumed`; approves by waiving disputed recovery, writing `waived`, creating a release action, and optionally creating a payout compensation action.
    Evidence: `locallife/db/sqlc/tx_recovery_dispute_review.go:31`, `locallife/db/sqlc/tx_recovery_dispute_review.go:51`, `locallife/db/sqlc/tx_recovery_dispute_review.go:57`, `locallife/db/sqlc/tx_recovery_dispute_review.go:62`, `locallife/db/sqlc/tx_recovery_dispute_review.go:77`, `locallife/db/sqlc/tx_recovery_dispute_review.go:84`, `locallife/db/sqlc/tx_recovery_dispute_review.go:89`, `locallife/db/sqlc/tx_recovery_dispute_review.go:98`, `locallife/db/sqlc/tx_recovery_dispute_review.go:110`, `locallife/db/sqlc/tx_recovery_dispute_review.go:151`.

20. Recovery-dispute result effects execute release action, penalize claimant, optionally execute compensation payout, or resume recovery after rejection; notifications are best-effort after effects.
    Evidence: `locallife/worker/task_process_recovery_dispute_result.go:75`, `locallife/worker/task_process_recovery_dispute_result.go:82`, `locallife/worker/task_process_recovery_dispute_result.go:104`, `locallife/worker/task_process_recovery_dispute_result.go:106`, `locallife/worker/task_process_recovery_dispute_result.go:123`, `locallife/worker/task_process_recovery_dispute_result.go:126`, `locallife/worker/task_process_recovery_dispute_result.go:129`, `locallife/worker/task_process_recovery_dispute_result.go:230`, `locallife/worker/task_process_recovery_dispute_result.go:258`.

21. Claim recovery overdue scheduler scans due pending recoveries, marks them overdue with a block behavior action, and enqueues the action. Rider-target block action suspends the rider; release action later clears recovery suspension if no blocking recovery remains.
    Evidence: `locallife/worker/claim_recovery_scheduler.go:78`, `locallife/worker/claim_recovery_scheduler.go:88`, `locallife/worker/claim_recovery_scheduler.go:98`, `locallife/worker/claim_recovery_scheduler.go:107`, `locallife/worker/task_claim_behavior_action.go:243`, `locallife/worker/task_claim_behavior_action.go:275`, `locallife/worker/task_claim_behavior_action.go:287`, `locallife/worker/task_claim_behavior_action.go:421`, `locallife/worker/task_claim_behavior_action.go:490`, `locallife/worker/task_claim_behavior_action.go:496`.

## SQL And Durable State Boundaries

- `claims`: rider-visible claim amount/status/description and reviewed fields.
- `orders` and `deliveries`: rider ownership join and order/rider behavior context.
- `behavior_decisions` and behavior trace tables: persisted decision/read-only adjudication evidence.
- `claim_recoveries`: recovery target, amount, due date, status `pending/overdue/disputed/paid/waived`, and closed state.
- `claim_recovery_events`: created/payable/payment_started/paid/overdue/disputed/waived/resumed/closed audit trail.
- `recovery_disputes`: rider dispute reason/status/review notes/compensation/reviewer timestamps.
- `payment_orders`, `external_payment_commands`, `external_payment_facts`, `external_payment_fact_applications`: claim recovery WeChat payment, callback/query facts, and fact application truth.
- `behavior_actions`: block/open/release/payout/notify actions used for suspension, release, and compensation.
- `riders`: suspension fields mutated by rider-target overdue block/release effects.

## Trust, Authorization, And Tenant Checks

- All rider claim/recovery/dispute reads start from authenticated user id and server-side `GetRiderByUserID`.
- Claim list/detail/decision SQL is rider-scoped through delivery rider id, so unrelated claims become 404/empty.
- Rider behavior summary rejects orders whose delivery rider id does not match the current rider.
- Rider recovery get/pay checks both delivery rider id and `recovery_target='rider'`.
- Rider dispute creation checks rider ownership, dispute window, duplicate appellant type, and claim recovery target.
- Payment fact application trusts terminal payment facts only after provider callback/query validation in the shared payment domain; generic payment query/detail routes also scope payment order ownership by authenticated user.

## Idempotency And Duplicate-Submit Checks

- Claim list/detail/decision/summary reads are idempotent.
- Recovery payment reuses an existing unexpired payment order for the same attach/payer and rotates expired pending orders.
- Payment callback/query success is idempotent once recovery is already paid or the payment order is processed.
- Rider dispute creation returns the existing rider dispute when duplicate submit races with the unique constraint and appellant id matches.
- Recovery dispute result effects tolerate already pending/overdue recovery on rejected resume and already terminal release action success.
- Overdue scheduler uses transactions and behavior action execution status to avoid repeatedly suspending through the same action.

## Recovery And Async Convergence Paths

- Recovery payment converges through WeChat direct payment callback/query facts and payment fact application retries.
- If the Mini Program loses the payment result, generic payment polling/detail/query recovery can still converge the same claim recovery fact path.
- Automatic dispute resolution happens inline first; on failure it can retry through `automatic recovery dispute resolution` task.
- Result effects can run through async task distributor or inline fallback.
- Overdue recovery block actions are queued and retried; release actions run on paid/waived recovery and write closed events.
- Frontend detail page rehydrates after appeal/payment flows and preserves partial panel data when a subquery fails.

## Frontend Draft And Backend Rehydration

- Claims list derives bucket counts and rows from backend summary/list responses; recovery-dispute list now uses the registered rider dispute route, so the old missing-route failure is removed.
- Claim detail treats claim detail as primary truth and loads decision/recovery/appeal/behavior panels independently with retry affordances.
- Recovery payment UI is optimistic only for action notice; final paid/closed state must come from backend recovery/payment fact convergence.
- Generic payment workflow status is a UI recovery hint; final claim recovery closed/paid state is rehydrated from `/v1/rider/recoveries/:id`.

## Test Coverage Signals

Observed tests:

- `locallife/api/recovery_dispute_test.go` covers rider claim list/detail/decision and rider recovery dispute routes.
- `locallife/api/claim_recovery_test.go` and `locallife/logic/claim_recovery_payment_test.go` cover rider recovery access/payment branches where present.
- `locallife/db/sqlc/recovery_dispute_tx_test.go` covers dispute create/review transactions and recovery status side effects.
- `locallife/worker/task_automatic_recovery_dispute_resolution_test.go`, `locallife/worker/task_process_recovery_dispute_result_test.go`, and claim behavior action tests cover async result and action effects where present.
- `locallife/worker/task_risk_management.go` is covered by legacy no-op behavior in adjacent worker tests.

Missing high-value tests:

- Broader Mini Program page-level test that claims summary/list/dispute list partial failures are rendered with the desired UX rather than relying only on service contract coverage.
- End-to-end recovery payment success -> payment fact -> `claim_recoveries.status='paid'` -> release rider suspension.
- Contract coverage for claim recovery payment query fallback through `/v1/payments/:id/query` and `/v1/payments/:id`.
- End-to-end overdue rider recovery -> suspend rider -> dispute approved/paid -> release rider.
- Read-only decision endpoint should never create new behavior decisions/actions.

## Gaps And Refactor Notes

- Consider splitting claims page initial load so recovery-dispute list failures do not hide live claim list and summary.
- Consider naming cleanup from "appeal" to "recovery dispute" in rider frontend services to match backend route and domain terms.

## Dead And Orphan Paths

- Resolved: `weapp/miniprogram/pages/rider/_main_shared/api/appeals-customer-service.ts` rider list/detail/create wrappers now call `/v1/rider/recovery-disputes`.
- Residual cleanup: frontend service names still say appeal for compatibility with existing rider claims pages, although the backend domain term is recovery dispute.
- `locallife/worker/task_risk_management.go:64` through `locallife/worker/task_risk_management.go:72` keep the legacy `risk:check_rider_damage` consumer but intentionally ignore it because claim behavior trace is now the adjudication path.

## Branch Exhaustion

- Entry branches checked: claims page load, bucket switch, pagination, detail load, decision retry, recovery retry, behavior retry, dispute submit, recovery payment, and appeal-compatible recovery-dispute list/detail/create wrappers.
- Request branches checked: `/v1/rider/claims`, `/claims/summary`, `/claims/:id`, `/claims/:id/decision`, `/claims/behavior-summary`, `/recoveries/:id`, `/recoveries/:id/pay`, generic `/v1/payments/:id` and `/query` recovery, `/recovery-disputes` POST/GET, and `/recovery-disputes/:id`.
- Backend state branches checked: no rider, claim not assigned to rider, no persisted decision, recovery not found, recovery target mismatch, payout incomplete, already paid, non-payable status, payment order reuse/expiry/close, generic payment query unsupported/provider unavailable, missing WeChat openid, duplicate dispute, dispute window expired, submitted/approved/rejected dispute, pending/overdue/disputed/paid/waived recovery.
- Async branches checked: payment callback/query fact application, release behavior action, overdue scheduler/block action, automatic dispute resolution inline/retry, result effects inline/task, compensation payout action, claimant penalty, notifications.
- Dead/orphan branches checked: previously stale rider appeals API wrappers are now routed through recovery-disputes; legacy no-op rider damage risk worker remains intentionally dead.
