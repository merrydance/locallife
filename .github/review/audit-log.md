# Audit Log

Append one section per formal backend audit or durable review pass.

## Template

### YYYY-MM-DD - Scope

- Scope:
- Reviewed paths:
- Findings logged:
- Durable docs updated:
- Remaining scope:

### 2026-04-13 - WeChat applyment query by out_request_no

- Scope: Formal audit of the then-retained WeChat applyment query surface, later superseded by legacy surface removal.
- Reviewed paths: artifacts/payment-channel-removal-context.md; artifacts/payment-channel-removal-review-fix-plan.md
- Findings logged: BE-AUDIT-2026-04-13-01, BE-AUDIT-2026-04-13-02, BE-AUDIT-2026-04-13-03
- Durable docs updated: .github/review/open-findings.md
- Remaining scope: Caller-side error mapping, persistence propagation, UI presentation, and recovery scheduler behavior were intentionally left out of scope for this audit pass.

### 2026-04-14 - Applyment query usage propagation review

- Scope: Repository-wide review of the historical applyment query propagation path, now retained only as archive context after legacy surface removal.
- Reviewed paths: artifacts/payment-channel-removal-context.md; artifacts/payment-channel-removal-review-fix-plan.md; locallife/api/merchant_application.go; locallife/logic/baofu_merchant_report_service.go; locallife/worker/baofu_merchant_report_recovery_scheduler.go
- Findings logged: BE-AUDIT-2026-04-14-01
- Durable docs updated: .github/review/open-findings.md
- Remaining scope: Frontend rendering and operator-facing workflows were not re-reviewed; test coverage for remote account_validation/legal_validation_url propagation remains a residual verification gap but no concrete production defect was proven there in this pass.

### 2026-04-22 - API 5xx error mapping and logging debt sweep

- Scope: Focused backend audit of repository-wide API handlers still returning 5xx through `errorResponse(...)`, plus evaluation of whether the repeated pattern should be pushed into durable review docs and changed-file Go guardrails.
- Reviewed paths: locallife/api/middleware.go; locallife/api/casbin_enforcer.go; locallife/api/scan.go; locallife/api/membership.go; locallife/api/risk_management.go; .github/standards/backend/ERROR_HANDLING.md; .github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md; .github/standards/backend/FORMAL_REVIEW_DURABILITY.md; .github/standards/backend/GO_PRACTICES.md; .github/scripts/backend_go_guard.sh
- Findings logged: BE-AUDIT-2026-04-22-01
- Durable docs updated: .github/review/open-findings.md; .github/review/audit-log.md; .github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md; .github/standards/backend/FORMAL_REVIEW_DURABILITY.md; .github/standards/backend/GO_PRACTICES.md; .github/scripts/backend_go_guard.sh
- Remaining scope: This pass did not convert existing production handlers yet. Raw internal-detail leakage on 4xx paths such as `errorResponse(fmt.Errorf(...%w...))`, and context-dependent nil/silent-fallback patterns, were reviewed as follow-up candidates but not promoted into a grep-based guard because the current signal-to-noise ratio is not yet good enough.

### 2026-05-19 - Subsidy authz and idempotency deferred pending platform re-enable

- Scope: Recorded the subsidy authz and idempotency follow-up state after confirming the WeChat platform e-commerce payment chain is disabled.
- Reviewed paths: artifacts/backend-subsidy-authz-task-card-2026-05-19.md; artifacts/backend-subsidy-idempotency-task-card-2026-05-19.md; artifacts/backend-ocr-subsidy-authz-idempotency-review-fix-index-2026-05-19.md; .github/review/open-findings.md
- Findings logged: BE-AUDIT-2026-05-19-01; BE-AUDIT-2026-05-19-02
- Durable docs updated: artifacts/backend-subsidy-authz-task-card-2026-05-19.md; artifacts/backend-subsidy-idempotency-task-card-2026-05-19.md; artifacts/backend-ocr-subsidy-authz-idempotency-review-fix-index-2026-05-19.md; .github/review/open-findings.md
- Remaining scope: No subsidy code changes were made in this pass. Reopen both deferred task cards when the platform payment chain is re-enabled, then implement the fixes before subsidy traffic resumes.

### 2026-05-29 - Baofu reservation refund and profit-sharing timing audit

- Scope: Focused G3 audit of Baofu reservation and reservation_addon profit-sharing readiness versus still-supported post-payment refund actions after the Baofu refund slice fix.
- Reviewed paths: artifacts/baofu-refund-slice-fix-plan.md; locallife/db/query/profit_sharing_order.sql; locallife/worker/baofu_payment_recovery_scheduler.go; locallife/worker/task_baofu_profit_sharing.go; locallife/logic/baofu_profit_sharing_trigger.go; locallife/logic/reservation.go; locallife/logic/reservation_dishes.go; locallife/logic/replace_order.go; locallife/logic/payment_fact_application_service.go; locallife/db/sqlc/profit_sharing_order_recovery_test.go
- Findings logged: BE-AUDIT-2026-05-29-01
- Durable docs updated: artifacts/baofu-refund-slice-fix-plan.md; .github/review/open-findings.md; .github/review/audit-log.md
- Remaining scope: No timing fix was implemented in this pass. The business cutoff for reservation refunds, dish modification, replacement-order refunds, and profit sharing still needs to be chosen before changing SQL, scheduler readiness, generated sqlc output, and regression tests.

### 2026-05-29 - Baofu reservation completed-share design

- Scope: Converted the Baofu reservation refund/share timing finding into an implementation-ready design after the business cutoff was chosen.
- Reviewed paths: artifacts/baofu-refund-slice-fix-plan.md; artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md; .github/review/open-findings.md
- Findings logged: No new finding. `BE-AUDIT-2026-05-29-01` remains open until implementation and validation.
- Durable docs updated: artifacts/baofu-refund-slice-fix-plan.md; artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md; .github/review/open-findings.md; .github/review/audit-log.md
- Remaining scope: Implement the design: restrict reservation profit sharing to merchant `completed` + `completed_at`, add completion-triggered enqueue with scheduler fallback, and refactor dish-change/replacement-order refund creation into guarded DB transactions.

### 2026-05-29 - Baofu reservation completed-share implementation closure

- Scope: Implemented and validated `BE-AUDIT-2026-05-29-01` across SQL/sqlc readiness, worker defensive checks, reservation completion trigger, shared Baofu profit-sharing config resolution, and refund transaction composition for dish changes and replacement orders.
- Reviewed paths: locallife/db/query/profit_sharing_order.sql; locallife/db/sqlc/profit_sharing_order.sql.go; locallife/db/sqlc/profit_sharing_order_recovery_test.go; locallife/db/sqlc/tx_refund.go; locallife/db/sqlc/tx_reservation.go; locallife/db/sqlc/tx_replace_order.go; locallife/db/sqlc/store.go; locallife/logic/baofu_profit_sharing_config.go; locallife/logic/payment_fact_application_service.go; locallife/logic/reservation.go; locallife/logic/reservation_dishes.go; locallife/logic/replace_order.go; locallife/worker/baofu_payment_recovery_scheduler.go
- Findings logged: No new finding. `BE-AUDIT-2026-05-29-01` marked resolved.
- Durable docs updated: .github/review/open-findings.md; .github/review/audit-log.md; artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md
- Remaining scope: No historical-data migration or remediation was required because the issue was found before affected historical records existed. Post-share refund remains outside this first-version scope.

### 2026-05-29 - Baofu reservation successful partial-refund net-share follow-up

- Scope: Reviewed and tightened the completed-share implementation after clarifying that successful partial refunds should not permanently block reservation sharing; the later Baofu share bill should use the payment-order net amount.
- Reviewed paths: locallife/db/query/profit_sharing_order.sql; locallife/db/query/refund_order.sql; locallife/db/sqlc/tx_baofu_profit_sharing.go; locallife/db/sqlc/profit_sharing_order_recovery_test.go; locallife/logic/reservation_profit_sharing.go; locallife/logic/baofu_profit_sharing_trigger.go; locallife/logic/order_service_confirm_test.go; locallife/worker/baofu_payment_recovery_scheduler.go; artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md; artifacts/baofu-refund-slice-fix-plan.md
- Findings logged: No new finding. `BE-AUDIT-2026-05-29-01` remains resolved with a clarified invariant.
- Durable docs updated: .github/review/open-findings.md; .github/review/audit-log.md; artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md; artifacts/baofu-refund-slice-fix-plan.md
- Remaining scope: Net-amount sharing is deliberately scoped to `reservation` and `reservation_addon` payment orders. Ordinary `order` payment orders with successful refunds remain excluded from automatic Baofu sharing; post-share refund remains outside this first-version scope.

### 2026-06-16 - Table QR scene parsing and persistence recovery audit

- Scope: Focused audit of the table QR generation, Mini Program scene parsing, and QR persistence update chain on the dine-in / merchant table path.
- Reviewed paths: locallife/api/scan.go; locallife/api/scan_test.go; locallife/api/table.go; locallife/db/sqlc/tx_table.go; locallife/db/migration/000140_create_media_assets.up.sql; locallife/db/migration/000009_add_tables_and_reservations.up.sql; weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts; weapp/miniprogram/pages/dine-in/menu/menu.ts; weapp/miniprogram/pages/merchant/tables/edit/index.ts; weapp/miniprogram/pages/merchant/tables/shared/table-edit-view.ts; weapp/miniprogram/pages/merchant/_utils/merchant-tables-shared.ts
- Findings logged: BE-AUDIT-2026-06-16-01, BE-AUDIT-2026-06-16-02
- Durable docs updated: artifacts/production-risk-audit/state-sequencing-audit-snapshot-2026-06-16.md; .github/review/open-findings.md; .github/review/audit-log.md
- Remaining scope: The table update transaction path that clears QR on table number change is still a non-finding for the scanned merchant edit flow; the WeChat `env_version` setting remains a separate evidence gap and was not promoted to a finding in this pass.

### 2026-06-22 - P0-F10/P0-F11 recovery gap confirmation

- Scope: Focused only on the already-audited `P0-F10` claim behavior action recovery gap and `P0-F11` rider deposit refund recovery gap, with no business-code changes.
- Reviewed paths: locallife/worker/task_claim_behavior_action.go; locallife/worker/claim_behavior_action_recovery_scheduler.go; locallife/worker/claim_refund_recovery_scheduler.go; locallife/worker/refund_recovery_scheduler.go; locallife/logic/rider_deposit_refund_service.go; locallife/worker/task_claim_behavior_action_test.go; locallife/worker/claim_behavior_action_recovery_scheduler_test.go; locallife/worker/refund_recovery_scheduler_test.go; locallife/logic/rider_deposit_refund_service_test.go; artifacts/production-risk-audit/state-sequencing-audit-snapshot-2026-06-16.md
- Findings logged: BE-AUDIT-2026-06-22-01, BE-AUDIT-2026-06-22-02
- Durable docs updated: artifacts/production-risk-audit/state-sequencing-audit-snapshot-2026-06-16.md; .github/review/open-findings.md; .github/review/audit-log.md
- Remaining scope: No code or test changes were made in this pass. The next step, if we switch from audit to implementation, is a focused recovery fix plus regression coverage for stale `running` claim behavior actions and rider-deposit `pending`/`unknown` refunds.

### 2026-06-22 - P0-F11 rider deposit refund recovery fix closed

- Scope: Implemented the rider deposit refund recovery gap that was previously confirmed in `BE-AUDIT-2026-06-22-02`, keeping the fix limited to `RefundRecoveryScheduler` and its focused tests.
- Reviewed paths: locallife/db/query/refund_order.sql; locallife/worker/refund_recovery_scheduler.go; locallife/worker/refund_recovery_scheduler_test.go; locallife/db/sqlc/refund_order.sql.go; locallife/db/sqlc/querier.go; locallife/db/mock/store.go; locallife/logic/payment_fact_application_service.go; locallife/worker/payment_fact_application_scheduler.go; .github/review/open-findings.md; artifacts/production-risk-audit/state-sequencing-audit-snapshot-2026-06-16.md
- Findings logged: BE-AUDIT-2026-06-22-02 resolved
- Durable docs updated: .github/review/open-findings.md; .github/review/audit-log.md; artifacts/production-risk-audit/state-sequencing-audit-snapshot-2026-06-16.md; docs/superpowers/plans/2026-06-22-rider-deposit-refund-recovery.md
- Validation: `PATH=/usr/local/go/bin:$PATH go test ./worker -run 'TestRefundRecoverySchedulerRunOnce' -v` passed after the regression expectations were updated.
- Remaining scope: The recovery scan is now covered by focused worker tests; broader end-to-end verification of a real provider `unknown`/lost-callback scenario remains unexercised.

### 2026-06-23 - P0-F11 rider deposit refund recovery residual validation

- Scope: Reduced the residual risk left after the P0-F11 fix by adding DB-backed integration coverage for a rider-deposit refund that remains `pending` after an `unknown` direct-refund create outcome and receives no callback.
- Reviewed paths: locallife/integration/takeout_journey_integration_test.go; locallife/worker/refund_recovery_scheduler.go; locallife/logic/payment_fact_application_service.go; locallife/db/sqlc/tx_rider_refund.go; docs/superpowers/plans/2026-06-22-rider-deposit-refund-recovery.md
- Validation: `PATH=/usr/local/go/bin:$PATH go test -v -cover -count=1 -p 1 ./integration -run TestRiderDepositRefundPendingUnknownRecoveryIntegration` passed. The test verifies persisted convergence through `RefundRecoveryScheduler.RunOnce()` -> query fact/application -> `PaymentFactService.ApplyExternalPaymentFactApplication`.
- Remaining scope: Local integration now covers the internal recovery chain. True WeChat callback delivery loss, provider query availability, and production scheduler/queue observability still need operational monitoring or read-only production checks.
