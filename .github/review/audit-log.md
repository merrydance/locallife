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

