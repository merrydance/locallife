# Rider Flow Variant Index

Status: created 2026-06-08
Purpose: branch checklist for the full rider-side codegraph. Detailed evidence is in the individual slice files.

## Application And Identity

- `GET /v1/rider/application`: existing application returns as-is; no application creates a new draft.
- Private document preview: existing asset ids resolve through `/v1/media/private-access`; non-owner owner-only documents are forbidden and preview failure does not mutate application state.
- Draft edit: basic fields and documents require editable draft; submitted can be reset explicitly, not silently reset by GET.
- Document/OCR: ID card front/back and health cert OCR have pending/processing/failed/succeeded branches, stale media guards, and health cert manual valid-end correction.
- Document delete: invalid type rejects, non-draft rejects, valid draft clears the bound media id/OCR payload and best-effort soft-deletes the old `media_assets` row.
- Submit: rejects non-draft, missing name/phone/docs, pending/failed/incomplete OCR, missing health cert name/valid_end, ID expiry, health cert expiry, and health-vs-ID name mismatch.
- Review: approved creates rider and user role; rejected/needs_resubmit returns the application to draft with reason. Review may run async or sync fallback.
- Credential governance: credential ledger activation, 7-day reminder, expiry suspension through `rider_profiles`, suspension-slot collision, and restore after renewed approval are all explicit branches.

## Workbench, Status, Location

- Profile: `/v1/rider/me` is a read-only `riders` profile/status/deposit/location/stats fetch; missing rider returns 404 and never creates rider state.
- Region: current region can be synced explicitly or via location upload; no region blocks status/deposit/online readiness.
- Status: online block branches are suspended, ineligible status, insufficient available deposit, and missing Baofu settlement account.
- Online: idempotent when already online; offline is blocked by active deliveries and idempotent when already offline.
- Location: future timestamps are clamped, records older than one hour are rejected, offline rider is rejected, supplied delivery id must be the current active delivery.
- Geofence: active delivery statuses route to pickup-arrival, pickup-confirm, or delivery-confirm events; auto transitions are best-effort and deduped per event type.
- Notification center: dashboard enters `/pages/notification/index?mode=rider`; generic `/v1/notifications` list/unread/read/read-all/delete is user-scoped, while rider categories and related-page jumps are client-side filters.

## Delivery

- New-order visibility: `delivery_pool_new` WebSocket messages are pushed to nearby online riders and replay-filtered against live `delivery_pool`; duplicate messages only affect the local badge.
- Recommend: requires rider context through user id; score list is enriched with merchant, delivery, item count, and real distance when available.
- Pending dispatch timeout: 3-minute operator alert dedupes through `delivery_timeout_alerts`; 20-minute merchant alert marks `deliveries.is_delayed`; 60-minute auto-cancel calls `CancelOrderTx`, removes `delivery_pool`, cancels delivery, and enqueues refund for successful external payment.
- Cancelled-pool convergence: successful order cancellation now removes the matching `delivery_pool` row; remaining gap is lack of observed `delivery_pool_gone` push for already-open clients on scheduler auto-cancel.
- Grab: rejects not rider, offline, inactive/suspended, missing Baofu settlement readiness, expired/missing pool, distance too far, order state not allowed, insufficient deposit, and rider profit-sharing bill not pending.
- Grab transaction: locks order/rider/pool, assigns delivery, updates rider profit-sharing bill, removes pool, freezes deposit, writes freeze log, syncs order status and order status log.
- Pickup: assigned -> picking; food-safety progression guard applies.
- Confirm pickup: picking -> picked; can be blocked when merchant fulfillment is not ready.
- Start delivery: picked -> delivering.
- Confirm delivery: delivering -> delivered; validates current rider location freshness and delivery radius, unfreezes deposit, creates unfreeze log, updates rider stats, order becomes `rider_delivered`.
- Navigation: authenticated `/v1/location/direction/bicycling` validates coordinates and returns provider route distance/duration/points without SQL writes; failure falls back to local map display.

## Deposit

- Recharge: rider must be approved/active and direct payment client configured; pending same user/business/amount payment is reused.
- Pending recharge recovery: Mini Program tries `/v1/payments/:id/query`, which may record/apply rider-deposit query facts, then falls back to `/v1/payments/:id` if remote query is unsupported/unavailable.
- Payment success: callback/recovery creates external fact/application; application marks payment processed, writes deposit log, creates refundable credit, and reconciles rider operational status.
- Withdrawal: rejects missing rider, inactive status, any frozen deposit, insufficient available deposit after pending refunds, and active deliveries.
- Refund prepare: locks rider and credits, reserves refundable credits oldest first, creates refund orders, creates hidden freeze logs, increases `riders.frozen_deposit`.
- Refund request: upstream create failure compensates immediately; SUCCESS/PROCESSING/CLOSED/ABNORMAL are accepted as processing and converge only through facts.
- Refund fact: success drains balance/frozen/credit and writes withdraw log; failed/abnormal/closed restores credit, unfreezes rider deposit, and marks refund failed/closed.
- Credit lifecycle: reminders are sent before expiry; expired credits are marked expired and alerted.

## Income And Baofu

- Income reads are read-only over `profit_sharing_orders`, filtered by rider id/date/status.
- Rider income status convergence: completed order schedules a Baofu share command, command moves pending/failed to processing, callback/query fact application moves processing to finished/failed, and amount mismatch blocks unsafe success.
- Baofu rider settlement account has profile_pending, verify_fee_pending, verify_fee_processing, opening_processing, ready, failed, abnormal, and voided states in shared onboarding infrastructure.
- Verify-fee payment: shared payment workflow polls/detail-reads generic payment state; `/v1/payments/:id/query` can apply direct query facts for success, failed, closed, or expired verify-fee results.
- Rider personal account opening does not use merchant report continuation; active binding directly means ready.
- Online and grab require Baofu settlement payment readiness.
- Baofu income withdrawal balance uses provider balance query with active binding/contract no.
- Withdrawal create creates `baofu_withdrawal_orders(status='processing')`; rejected acceptance changes to failed, unknown create result remains processing for recovery.
- Callback/recovery updates `baofu_withdrawal_orders` to succeeded/failed/returned and never regresses terminal rows.

## Claims And Recovery

- Rider claim list/detail only returns claims joined to deliveries whose `rider_id` is the current rider.
- Decision read is pure read of persisted behavior decisions and does not rerun adjudication.
- Rider recovery get/pay requires recovery target `rider` and matching delivery rider id.
- Recovery payment requires claim payout already completed, recovery status pending/overdue, and no existing unexpired payment order unless reusable.
- Recovery payment convergence uses shared payment workflow and generic payment query/detail recovery; callback and query facts both flow into the claim-recovery `ProcessPaymentSuccessTx` branch.
- Recovery dispute submit requires existing rider-target claim recovery, current rider ownership, within window, and idempotently returns an existing rider dispute on duplicate.
- Dispute create marks claim recovery `disputed` and writes event.
- Automatic review approval waives recovery, creates release behavior action, may create compensation payout action, penalizes claimant, and notifies both sides.
- Automatic review rejection resumes recovery to pending or overdue and notifies both sides.
- Overdue scheduler turns pending recoveries overdue and creates rider/merchant block behavior actions.

## Dead/Orphan Branches

- Resolved: old rider delivery detail API wrapper is backed by `GET /v1/delivery/:delivery_id`.
- Resolved: worker fallback new-order push emits `delivery_pool_new`.
- Resolved: stale pending-delivery auto-cancel removes the `delivery_pool` row through `CancelOrderTx`; realtime gone push remains an operations gap.
- Resolved: old rider appeal-compatible wrapper calls `/v1/rider/recovery-disputes` instead of missing `/v1/rider/appeals` routes.
- Legacy rider damage risk worker is intentionally a no-op.
- Legacy refund result worker refuses rider deposit refund application and points to payment facts.
