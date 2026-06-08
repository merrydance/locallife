# Platform Operations Closed-Loop Backlog

Status: parked 2026-06-08
Source: cross-role issues separated from the rider-side audit
Scope: platform/operator/merchant/user operations closure items that should not be used as the rider-side closure verdict

## Why This Exists

The rider-side codegraph should answer whether a rider can see, act on, and recover their own business state. Some issues discovered while auditing rider flows are actually platform-level operating loops: dispatch intervention, refund exception handling, operator task closure, merchant timeout escalation, and unified incident timelines.

Those items are parked here so later platform, operator, merchant, and user role audits can own them without polluting the rider-side completeness judgment.

## Parked Cross-Role Gaps

1. Pending-dispatch operator handling is observable but not yet a full action loop.
   Evidence: `locallife/scheduler/operator_dispatch_alert.go`, `locallife/api/operator_dispatch_monitor.go`, `weapp/miniprogram/pages/operator/dispatch-hall/index.ts`.
   Current shape: 3-minute pending deliveries create an alert ledger and operator notification; the operator dispatch hall can read summaries/lists.
   Follow-up question: does operations need explicit assign/escalate/close/remark actions and an auditable handling status?

2. Stale-delivery external refund enqueue failure is logged but not clearly converted into a platform alert or compensation work item.
   Evidence: `locallife/scheduler/data_cleanup.go:1367`.
   Current shape: the order/delivery can be cancelled and the refund task enqueue can fail afterward; the branch logs the failure and comments that later/manual compensation is possible.
   Follow-up question: should this create a payment-domain/platform alert, durable retry row, or operator-visible compensation task?

3. Merchant 20-minute delay notification is not tied to an operator closure state.
   Evidence: `locallife/scheduler/data_cleanup.go:1381`.
   Current shape: delayed pending deliveries can mark `deliveries.is_delayed` and notify the merchant.
   Follow-up question: should merchant-facing delay, operator dispatch alert, and eventual auto-cancel share one incident/work-order timeline?

4. No unified cross-role timeout timeline was observed for one delivery.
   Current shape: rider pool state, operator dispatch alert, merchant delay alert, auto-cancel, and refund recovery are stored or emitted through separate paths.
   Follow-up question: should platform expose one timeline tying dispatch alert -> merchant alert -> auto-cancel -> refund status -> role notifications?

5. Rider credential lifecycle has rider-facing reminder/suspension/restore paths, but platform audit/report ownership should be confirmed separately.
   Evidence: `artifacts/codegraph/rider-state-flows/rider-application-onboarding.slice.md`.
   Current shape: rider-side audit records credential reminder, expiry suspension, and renewed-approval restore.
   Follow-up question: should platform expose a credential governance report showing reminder, suspension, restore, and collision outcomes?

6. Rider deposit abnormal refund facts have a payment-domain alert handoff, but manual handling procedures belong to platform operations.
   Evidence: `artifacts/codegraph/rider-state-flows/rider-deposit.slice.md`.
   Current shape: rider-side truth converges through refund facts and status reads; abnormal refund facts create alert/outbox handoff.
   Follow-up question: should platform define the manual reconciliation playbook, owner queue, and closure status for abnormal rider deposit refunds?

## Rider-Side Boundary

These items remain outside the rider-side closure verdict unless they directly affect the rider's own visible state or available actions.

The rider-side realtime recovery item discovered from this group is now handled in the rider slice: scheduler auto-cancel removes SQL pool truth and publishes best-effort `delivery_pool_gone` invalidation to online active riders inside the recommendation-visible radius. Remaining items here are platform/operator/merchant/user operating-loop questions.
