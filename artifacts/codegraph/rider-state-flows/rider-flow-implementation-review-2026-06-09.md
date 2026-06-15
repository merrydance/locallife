# Rider Flow Implementation Review - 2026-06-09

Scope: review-only pass over every rider-side flow documented in `artifacts/codegraph/rider-state-flows/`.
Rule for this pass: mark implementation risks and branch gaps only; do not patch business code.

## Findings

### F1 [High] Rider approval can commit before review-run persistence and credential ledger activation

Flow: application and identity onboarding.

Evidence:
- `locallife/logic/rider_onboarding_review_service.go:101` through `:114` approves the application and creates the rider first.
- `locallife/db/sqlc/tx_rider_application.go:35` through `:64` shows the approval transaction only updates the application, creates `riders`, and creates `user_roles`.
- `locallife/logic/rider_onboarding_review_service.go:127` through `:142` persists/completes the onboarding review run after approval.
- `locallife/logic/rider_onboarding_review_service.go:145` through `:160` activates rider credential ledgers after approval and review-run handling.
- `locallife/logic/rider_onboarding_review_service.go:86` through `:87` refuses to process non-`submitted` applications; the worker path calls this service at `locallife/worker/task_onboarding_review.go:81`.

Impact: if review summary persistence or credential activation fails after `ApproveRiderApplicationTx` commits, the app is already `approved` and the rider/user role exist. A retry of the review worker will hit the non-`submitted` guard, so the system can strand an active rider without credential ledger/governance evidence.

Discussion direction: make post-approval credential activation recoverable for already-approved rider applications, or move the approval/review-summary/credential-activation boundary into one durable state machine with a retryable continuation state.

### F2 [Medium] Registration document completeness gate depends on preview URL instead of canonical asset id

Flow: application and identity onboarding.

Evidence:
- Existing document preview resolution returns `''` on private media URL failure at `weapp/miniprogram/pages/register/rider/index.ts:214` through `:220`.
- Preview refresh only writes the URL when a non-empty URL is resolved at `weapp/miniprogram/pages/register/rider/index.ts:241` through `:249`.
- The step gate checks `!idFront.url || !idBack.url || !healthCert.url` at `weapp/miniprogram/pages/register/rider/index.ts:410` through `:412`.

Impact: backend truth may have valid `assetId` bindings while a private preview URL is empty because of access URL latency/failure. The rider is then blocked client-side even though the canonical upload/document state exists.

Discussion direction: use `assetId` for completeness, and treat preview URL as display-only.

### F3 [Medium] Workbench order-pool count is global, not rider-visible

Flow: workbench, status, and location.

Evidence:
- Workbench summary calls `service.loadOrderPool(ctx, rider.ID, &result)` at `locallife/logic/rider_workbench.go:191`.
- `loadOrderPool` ignores rider location/eligibility and calls `CountDeliveryPool(ctx)` at `locallife/logic/rider_workbench.go:229` through `:236`.
- `CountDeliveryPool` is `SELECT COUNT(*) FROM delivery_pool` at `locallife/db/query/delivery_pool.sql:75` through `:76`.
- The rider-visible recommendation path has a nearby filter at `locallife/db/query/delivery_pool.sql:56` through `:73`.

Impact: dashboard "available orders" can show orders outside the rider's visible radius or current eligibility. This is not a money-loss bug, but it weakens the operator/rider closed loop because the dashboard count can disagree with the actual order hall.

Discussion direction: either label it as global platform pool count, or compute the same rider-visible count used by recommendation.

### F4 [Low] Non-eligible rider status returns a deposit-style online block reason

Flow: workbench, status, and location.

Evidence:
- Suspended status gets a status-specific message at `locallife/api/rider.go:827` through `:829`.
- The next branch for any other non-online-eligible rider status sets `OnlineBlockReason = "押金不足..."` at `locallife/api/rider.go:830` through `:832`.
- The actual insufficient-deposit branch is separate at `locallife/api/rider.go:833` through `:835`.

Impact: approved/pending/inactive or operationally paused riders can receive the wrong instruction. This makes frontline support and rider self-recovery less clear.

Discussion direction: map non-eligible status to status-specific operational copy instead of deposit copy.

### F5 [High] Grab transaction does not revalidate latest rider eligibility after row lock

Flow: delivery lifecycle.

Evidence:
- `GrabDeliveryOrder` pre-checks `IsOnline`, `Status`, suspension, and Baofu readiness before entering the transaction at `locallife/logic/delivery_grab.go:54` through `:80`.
- `GrabOrderTx` locks the order and rider row at `locallife/db/sqlc/tx_delivery.go:201` through `:209`, then only rechecks deposit at `locallife/db/sqlc/tx_delivery.go:221` through `:225`.
- The same rider state can be changed concurrently by offline at `locallife/api/rider.go:977` through `:980`, platform rider status update at `locallife/api/platform_rider_management.go:200` through `:203`, and suspension writes through `locallife/db/query/trust_score.sql:192` through `:204`.

Impact: a rider can pass the pre-check, then be offlined/suspended/deactivated before the transaction commits; the transaction can still assign delivery, remove the pool row, and freeze deposit. This breaks the "only online eligible riders can grab" invariant under races.

Discussion direction: revalidate online status, active status, rider suspension, and Baofu readiness inside the locked transaction or make the pre-check inputs part of conditional SQL updates.

### F6 [High] Manual confirm-delivery can complete delivery/deposit/stats even when order status update failed or no-oped

Flow: delivery lifecycle.

Evidence:
- `ConfirmDelivery` loads the order but does not call `IsOrderStatusAllowedForDeliveryAction` before `CompleteDeliveryTx` at `locallife/logic/delivery_status.go:345` through `:361`.
- It always sets the response order status to `rider_delivered` at `locallife/logic/delivery_status.go:366` through `:377`.
- `CompleteDeliveryTx` updates `deliveries` first at `locallife/db/sqlc/tx_delivery.go:335` through `:342`.
- It then calls `UpdateOrderToRiderDelivered` and ignores `ErrRecordNotFound` at `locallife/db/sqlc/tx_delivery.go:344` through `:350`.
- `UpdateOrderToRiderDelivered` only updates orders currently in `delivering` or `rider_delivered` at `locallife/db/query/order.sql:273` through `:280`.
- The geofence auto path has the missing guard at `locallife/logic/delivery_geofence.go:127` through `:135`, but manual confirm does not.

Impact: DB can end with `deliveries.status='delivered'`, deposit unfrozen, and rider stats incremented while `orders.status` remains a non-delivered status. The API response can falsely report `rider_delivered`.

Discussion direction: manual confirm should use the same order-action guard as geofence, and `CompleteDeliveryTx` should fail if order status cannot be advanced.

### F7 [Medium] Start-delivery lacks the same explicit order-action pre-check used elsewhere

Flow: delivery lifecycle.

Evidence:
- `StartDelivery` checks assigned rider and delivery status at `locallife/logic/delivery_status.go:272` through `:278`, then calls `UpdateDeliveryToDeliveringTx` at `locallife/logic/delivery_status.go:289` through `:293`.
- It does not explicitly call `IsOrderStatusAllowedForDeliveryAction` before the transaction.
- The SQL order update only allows `picked` or `delivering` at `locallife/db/query/order.sql:265` through `:271`.

Impact: lower than F6 because the transaction returns a conflict rather than swallowing the failed order update, but the logic-layer state-machine checks are inconsistent across delivery actions.

Discussion direction: add the same explicit order-action guard for consistency and clearer regression tests.

### F8 [Low/Medium] Recommendation read can show orders to riders who cannot grab because Baofu readiness is missing

Flow: delivery lifecycle / income readiness boundary.

Evidence:
- Recommendation by user checks rider exists, online, active, and not suspended at `locallife/logic/delivery_recommendation_user.go:33` through `:52`.
- It does not check Baofu settlement readiness before returning recommendations.
- Grab checks Baofu readiness at `locallife/logic/delivery_grab.go:74` through `:80`.

Impact: a rider can see recommended orders that are guaranteed to fail at grab time if settlement readiness has drifted after going online. This can be intended as soft visibility, but it conflicts with the current "online/grab readiness" model.

Discussion direction: decide whether order visibility should share the grab-readiness gate or display a settlement-readiness blocker before showing actionable cards.

### F9 [Medium, historical/fixed] Rider deposit withdrawal formerly lacked durable request idempotency

Current-code status, 2026-06-15 sync: fixed/stale finding. Current code requires `Idempotency-Key` on `POST /v1/rider/withdraw`, persists `rider_deposit_withdrawal_requests` with `(user_id, idempotency_key)` uniqueness and request hash, replays same user/key/hash refund plans, rejects conflicting reuse, and the Mini Program stores the draft key with the pending withdrawal context. Keep this item as historical context only; future work should run the focused idempotency tests before changing the flow.

Flow: rider deposit.

Historical finding basis:
- The original review found a request-level retry gap: a lost 202 response could leave the client without the refund order ids needed for status recovery, and the review snapshot did not show a durable operation key bound to the withdrawal request.
- That finding is retained here only to explain why the fix was made; the evidence below supersedes the older source anchors.

Current code evidence:
- `POST /v1/rider/withdraw` now documents and requires `Idempotency-Key`, rejects missing or too-long keys, and passes the key to `SubmitWithdrawal` at `locallife/api/rider.go:540` through `:575`.
- `SubmitWithdrawal` trims/requires the key, derives a request hash, and passes both into `PrepareRiderDepositRefundTx` at `locallife/logic/rider_deposit_refund_service.go:76` through `:126`.
- `PrepareRiderDepositRefundTx` locks the rider, looks up `rider_deposit_withdrawal_requests` by `(user_id, idempotency_key)`, replays the same hash by loading the original refund plan, rejects same-key/different-hash reuse, and only creates refund orders for a first-time request at `locallife/db/sqlc/tx_rider_refund.go:67` through `:258`.
- Replayed requests skip new WeChat refund creation for already non-pending refund orders at `locallife/logic/rider_deposit_refund_service.go:151` through `:164`.
- Schema support lives in `locallife/db/migration/000268_add_rider_deposit_withdrawal_idempotency.up.sql`.

Impact after sync: repeated same-key rider-deposit withdrawal retries now resolve to the original refund plan instead of producing another local refund plan. The remaining work is validation discipline: rerun focused API/logic/sqlc/Mini Program idempotency tests before changing this flow, and keep provider refund callback/query recovery evidence tracked separately.

Discussion direction: no new idempotency design is needed for this item unless fresh tests or source review uncover drift.

### F10 [Medium] Rider deposit recharge reuses a pending local payment row but recreates the WeChat order with the same out_trade_no

Flow: rider deposit.

Evidence:
- Existing pending local payment is reused at `locallife/api/rider.go:405` through `:415`.
- The handler still calls `CreateJSAPIOrder` using the same `outTradeNo` at `locallife/api/rider.go:463` through `:471`.
- Duplicate `OUT_TRADE_NO_USED` maps to conflict at `locallife/logic/direct_payment_order_errors.go:45` through `:47`.
- Any create failure closes the local payment order at `locallife/api/rider.go:472` through `:481`; `UpdatePaymentOrderToClosed` only closes pending rows at `locallife/db/query/payment_order.sql:169` through `:174`.
- A later callback for a closed/failed order is treated as an anomaly refund path at `locallife/api/payment_callback.go:890` through `:907`.

Impact: if the first prepay creation succeeded but the client retries before payment completion, the second call can receive duplicate out-trade rejection and close the still-valid local payment order. If the user then pays or has already paid through the first session, the callback can fall into anomaly refund rather than deposit credit.

Discussion direction: when reusing a pending payment order with a prepay id, regenerate pay params from the stored prepay id or query the remote order instead of recreating and closing on duplicate create.

### F11 [High, partially fixed] Rider Baofu income withdrawal formerly lacked request idempotency

Current-code status, 2026-06-15 sync: request-idempotency portion fixed/stale. Current shared Baofu withdrawal create requires `Idempotency-Key`, stores `idempotency_key` plus `idempotency_request_hash` on `baofu_withdrawal_orders`, enforces per-owner key uniqueness and key/hash pair checks, replays same owner/key/request, rejects conflicting reuse, and writes a submitted async-worker provider command before dispatch. Remaining risk is provider-positive evidence and ambiguous create/manual recovery, not missing request idempotency.

Flow: rider income and Baofu withdrawal.

Historical finding basis:
- The original review found that rider income withdrawal creation had only frontend submit suppression and fresh backend request numbers, so a retry could create multiple local/provider withdrawal attempts before provider balance changed.
- That request-idempotency gap has since been fixed. This item remains open only for the provider-evidence/manual-recovery side of the original money-movement risk.

Current code evidence:
- Shared Baofu withdrawal create now requires `Idempotency-Key`, rejects missing or too-long keys, and returns `200 OK` on replay at `locallife/api/baofu_withdrawal.go:278` through `:325`.
- `CreateWithdrawal` computes a request hash from `(owner_type, owner_id, amount)`, replays same owner/key/hash, rejects conflicting reuse, checks provider balance, and persists the withdrawal order with `idempotency_key` plus `idempotency_request_hash` at `locallife/logic/baofu_withdraw_service.go:124` through `:247`.
- The order and submitted async provider command are written in one transaction by `CreateBaofuWithdrawalOrderWithSubmittedCommandTx` at `locallife/db/sqlc/tx_baofu_withdrawal.go:23` through `:58`.
- SQL and migrations support lookup and enforcement through `GetBaofuWithdrawalOrderByIdempotency` at `locallife/db/query/baofu_withdrawal_order.sql:36` through `:42`, migration `000260_add_baofu_withdrawal_idempotency.up.sql`, and migration `000261_harden_baofu_withdrawal_idempotency_pair.up.sql`.

Impact after sync: duplicate local Baofu withdrawal order creation from repeated same-key requests is no longer an active finding. The active residual risk is different: provider-positive withdrawal callback/funds evidence, ambiguous provider-create recovery, and manual reconciliation drills still need stronger proof.

Discussion direction: preserve the existing shared Baofu withdrawal idempotency contract; focus follow-up work on provider evidence and recovery operations instead of another request-idempotency redesign.

### F12 [Medium] Claim-recovery payment success resolves recovery by claim id instead of recovery id/target

Flow: rider claims and recovery.

Evidence:
- The payment attach includes `claim_id`, `recovery_id`, and `recovery_target` at `locallife/db/sqlc/tx_payment_success.go:247` through `:250`.
- The success transaction ignores target for lookup and calls `GetClaimRecoveryByClaimID` at `locallife/db/sqlc/tx_payment_success.go:259`.
- `GetClaimRecoveryByClaimID` returns the latest recovery for the claim regardless of target at `locallife/db/query/claim_recovery.sql:37` through `:42`.
- A target-aware query exists at `locallife/db/query/claim_recovery.sql:44` through `:50`.
- `claim_recoveries` has an index on `claim_id`, but no uniqueness preventing multiple recovery rows per claim at `locallife/db/migration/000119_add_claim_recoveries_and_remove_evidence.up.sql:3` through `:20`.
- Other recovery-dispute paths already model target-aware lookup, for example `CreateRiderRecoveryDispute` uses rider target context at `locallife/logic/recovery_dispute.go:95` through `:103` and creates rider-target disputes at `:137` through `:144`.

Impact: if the domain ever has multiple recoveries for one claim, or stale/historical rows coexist, a paid claim-recovery payment can mismatch against the latest claim recovery and fail after payment, or mark the wrong row. This is a latent consistency risk because the attach already carries the precise recovery id.

Discussion direction: resolve by `recovery_id` first, validate `claim_id` and `recovery_target`, then mark that exact row paid.

## Per-Flow Review Notes

### Application And Identity

Reviewed rider registration load/edit/upload/delete/OCR/submit/review/credential governance paths against `rider-application-onboarding.slice.md`. New findings are F1 and F2. The stale copied `/onboarding/rider` Mini Program wrappers remain dead-code cleanup debt already documented in the slice/completeness audit; no live backend route was found for that stale path.

### Workbench, Status, Location

Reviewed profile/status/current-region/online/offline/location/geofence/workbench/notification paths against `rider-workbench-status-location.slice.md`. New findings are F3 and F4. Location upload and geofence still recheck online/current active delivery ownership; no additional implementation issue was found in that branch beyond the already-documented multiple-active-delivery product ambiguity.

### Delivery Lifecycle

Reviewed recommend/grab/pickup/start-delivery/confirm-delivery/pending-dispatch cleanup/realtime visibility/navigation paths against `rider-delivery-lifecycle.slice.md`. New findings are F5 through F8. Pending-dispatch cancellation, pool removal, and `delivery_pool_gone` convergence matched the documented branch.

### Deposit

Reviewed recharge, payment query/callback fact application, withdrawal prepare/refund fact application, withdrawal status recovery, credit expiry, and legacy refund worker paths against `rider-deposit.slice.md`. New findings are F9 and F10. Terminal refund fact application looked idempotent and owner/business scoped.

### Income And Baofu

Reviewed income reads, profit-sharing trigger/command/callback/recovery/fact application, settlement-account onboarding, verify-fee payment, and Baofu withdrawal create/callback/recovery paths against `rider-income-and-baofu-withdrawal.slice.md`. New finding is F11. Profit-sharing terminal application and Baofu withdrawal terminal status application matched the documented idempotent terminal-row behavior.

### Claims And Recovery

Reviewed rider claims list/detail/decision/behavior summary, recovery detail/payment, payment fact success, dispute create/review/result effects, overdue block, and release action paths against `rider-claims-and-recovery.slice.md`. New finding is F12. Dispute duplicate handling, target ownership checks, auto-resolution/retry, rejected resume, approved waive/release, and overdue suspension/release paths matched the documented branches.

## Validation

- This pass changed only this markdown review artifact.
- No business code was modified and no backend/frontend tests were run for this review-only pass.
