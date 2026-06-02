# Merchant State Flow Remediation Log

Status: active handoff ledger
Created: 2026-05-31
Scope: merchant-side Mini Program/App state-flow audit repairs and adjacent high-risk fixes found during the audit loop

## Purpose

This file records fixes that were implemented after, or directly adjacent to, the merchant-side state-flow audit. It exists so a future context switch can answer three questions without reconstructing chat history:

1. Which audit finding was fixed?
2. Which commit contains the fix?
3. What risk remains after the fix?

When a future fix closes an audit finding, update this ledger, the parent row in `artifacts/merchant-state-flow-audit.md`, and the relevant slice under `artifacts/codegraph/merchant-state-flows/`.

## Repair Ledger

| Date | Flow | Commit | Risk | Fixed | Validation / Evidence | Remaining Risk |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-05-30 | `merchant-business-hours-auto-open` | `a0efffd3`, `9d47638e`, `7e9bd9d0` | G2 | Reworked the Mini Program business-hours time picker and sync action UI: time selection is usable, selected time is written back, the popup/body scroll conflict was fixed, first-slot values are preserved while adding another slot, and the sync action is placed as an independent right-side control. | WeApp build/debug loop during the business-hours session; commits `a0efffd3`, `9d47638e`, `7e9bd9d0`. | Backend scheduler multi-slot behavior still needs a focused DB/integration test. Special-date mixed closed/open semantics and legacy single-row helper queries remain audit follow-ups. |
| 2026-05-31 | `merchant-claim-recovery` | `024997f0` | G3 | Aligned merchant/rider claim recovery route and DTO contracts: Mini Program uses recovery-id `/v1/{role}/recoveries/:id` read/pay routes, merchant dispute APIs target `/v1/merchant/recovery-disputes/**`, list/detail expose `recovery_id` and `recovery_status`, and frontend status buckets use backend `disputed` with old `appealed` only as compatibility fallback. | Backend API/logic/worker/sqlc tests in the commit plus Mini Program `check-merchant-claim-recovery-contract.test.js`. Slice updated with fixed route/field notes. | Still needs SQL-level concurrent duplicate-dispute proof and full pay -> payment fact -> paid recovery -> release-action -> suspension-clear e2e coverage. Product still needs to decide whether a claim-id compatibility recovery route is required for external clients. |
| 2026-05-31 | `merchant-member-balance-adjust` | `62839932` | G3 | Hardened manual stored-value balance adjustment: backend now requires `Idempotency-Key`, adjustment transaction uses a durable idempotency key and conflict semantics, insufficient balance uses typed errors, member list pagination returns full total, Mini Program sends/reuses adjustment idempotency keys, transaction labels cover `adjustment_credit` and `adjustment_debit`, and order cancellation preserves principal/bonus split fields. | Backend membership API/logic/sqlc/order-status tests, generated docs/sqlc/mock, and Mini Program `check-merchant-member-balance-adjust-contract.test.js`. | Product should still explicitly confirm whether managers may manually mutate customer stored value. Generated direct balance SQL writers remain zombie/refactor candidates. |
| 2026-05-31 | `merchant-order-operations` | `6a19a9c0` | G3 | Fixed merchant manual refund creation from order detail: Mini Program refund wrapper now requires and sends `Idempotency-Key`, order detail creates a per-refund-draft key, reuses it for retry of the same unchanged draft, and refreshes it when the refund draft changes. | Mini Program `check-merchant-manual-refund-idempotency-contract.test.js`; payment/refund contract scripts updated. | Manual refund can now reach backend idempotency, but refund terminal convergence still depends on provider callback/query facts and the existing payment-domain outbox/recovery path. |
| 2026-05-31 | `merchant-order-operations` | `d3e84050` | G3 | Added recovery for existing normal-order refund rows stuck in `pending` after merchant reject or other order-refund paths: scheduler scans pending order refund rows for cancelled paid orders, dispatches refund tasks with the original `out_refund_no`, and refund worker validates reused refund rows against payment order and amount before calling the provider. | `go test ./worker -count=1`; focused `go test ./db/sqlc`; `make test-safety`; `make check-generated`; `make check-baofu-contract`; backend SQL guard and `git diff --check`. | Retry of terminal `failed` refund rows is still intentionally out of scope until provider error classification defines which failures are safe to retry. The UI/API visibility gap from this row is closed by the 2026-06-01 repair below. |
| 2026-06-01 | `merchant-order-operations` | `692ed4ef` | G3 | Added truthful merchant-reject refund submission state: backend returns `refund_submission.status/message/refund_id/out_refund_no`, preserves HTTP 200 when cancellation succeeds, distinguishes `accepted`, `pending_recovery`, `manual_required`, and `not_needed`, and keeps the reject response compatible with legacy top-level order fields. Mini Program and Flutter reject flows now display the backend refund message instead of hardcoded "已拒单并发起退款". | Focused logic/API/worker tests, `make swagger`, `make check-generated`, Mini Program lint, Flutter analyze, `make check-baofu-contract`, and broader safety validation in this branch. | Terminal `failed` refund rows are still not automatically retried until provider error classification defines safe retry behavior. Web was only protected by the compatible top-level order response; no Web UI copy change was made in this Mini Program/App-focused pass. |
| 2026-06-02 | `merchant-order-operations` | this commit | G2 | Aligned merchant order realtime contract: backend declares `order_update`, order service publishes merchant snapshots through that event type, and Mini Program order list/detail plus kitchen board/detail subscribe to `order_update` and rehydrate from backend truth instead of waiting for notification-shaped messages. | `go test ./logic -run 'TestOrderService.*MerchantOrder\\|TestOrderServiceAcceptMerchantOrder\\|TestOrderServiceMarkMerchantOrderReady' -count=1`; `go test ./websocket -count=1`; Mini Program `check:merchant-order-update-websocket-contract`, `compile`, and `lint`. | Detail/list/kitchen realtime refresh now has a Mini Program contract test. Flutter push/poll/websocket auto-accept dedupe and BLE no-duplicate-print proof remain separate open items. Terminal `failed` refund retry remains open pending provider error classification. |

## Open Queue After These Repairs

The highest-risk remaining merchant-side follow-ups are:

1. `merchant-order-operations`: classify Baofu refund create/provider errors and decide whether any terminal `failed` refund rows are safe to retry automatically.
2. `merchant-claim-recovery`: prove duplicate recovery-dispute submissions at DB/transaction level and add pay -> fact -> release e2e coverage.
3. `merchant-business-hours-auto-open`: add scheduler tests for multi-slot and special-date precedence.
4. `merchant-member-balance-adjust`: confirm manager permission for manual stored-value mutation and decide whether to retire unused direct balance SQL writers.

## Maintenance Rule

Every future repair from the merchant-state-flow audit must update all three places in the same branch:

1. This remediation log.
2. The matching row and next-step status in `artifacts/merchant-state-flow-audit.md`.
3. The matching `artifacts/codegraph/merchant-state-flows/<flow-id>.slice.md`.
