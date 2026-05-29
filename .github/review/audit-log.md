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

- Scope: Formal audit of the WeChat integration implementation for GET /v3/ecommerce/applyments/out-request-no/{out_request_no}, limited to the backend interface implementation itself and excluding caller behavior in api, logic, worker, or UI layers.
- Reviewed paths: locallife/wechat/ecommerce.go; locallife/wechat/direct_payment.go; locallife/wechat/interface.go; locallife/wechat/ecommerce_test.go
- Findings logged: BE-AUDIT-2026-04-13-01, BE-AUDIT-2026-04-13-02, BE-AUDIT-2026-04-13-03
- Durable docs updated: .github/review/open-findings.md
- Remaining scope: Caller-side error mapping, persistence propagation, UI presentation, and recovery scheduler behavior were intentionally left out of scope for this audit pass.

### 2026-04-14 - Applyment query usage propagation review

- Scope: Repository-wide review of production code that calls QueryEcommerceApplymentByOutRequestNo directly or consumes its upgraded status/field semantics in api, logic, worker, and persistence paths.
- Reviewed paths: locallife/wechat/ecommerce.go; locallife/api/ecommerce_applyment.go; locallife/api/ecommerce_applyment_test.go; locallife/logic/ecommerce_applyment_submission.go; locallife/logic/ecommerce_applyment_submission_test.go; locallife/worker/applyment_recovery_scheduler.go; locallife/worker/applyment_support.go; locallife/worker/task_process_payment.go; locallife/worker/applyment_recovery_scheduler_test.go; locallife/worker/applyment_support_test.go; locallife/db/migration/000197_expand_ecommerce_applyment_statuses.up.sql; locallife/db/query/ecommerce_applyment.sql
- Findings logged: BE-AUDIT-2026-04-14-01
- Durable docs updated: .github/review/open-findings.md
- Remaining scope: Frontend rendering and operator-facing workflows were not re-reviewed; test coverage for remote account_validation/legal_validation_url propagation remains a residual verification gap but no concrete production defect was proven there in this pass.

### 2026-04-22 - API 5xx error mapping and logging debt sweep

- Scope: Focused backend audit of repository-wide API handlers still returning 5xx through `errorResponse(...)`, plus evaluation of whether the repeated pattern should be pushed into durable review docs and changed-file Go guardrails.
- Reviewed paths: locallife/api/server.go; locallife/api/applyment_bank_catalog.go; locallife/api/complaint.go; locallife/api/subsidy.go; locallife/api/profit_sharing_capability.go; locallife/api/ecommerce_applyment.go; locallife/api/operator_application_admin.go; locallife/api/merchant_cancel_withdraw.go; .github/standards/backend/ERROR_HANDLING.md; .github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md; .github/standards/backend/FORMAL_REVIEW_DURABILITY.md; .github/standards/backend/GO_PRACTICES.md; .github/scripts/backend_go_guard.sh
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
