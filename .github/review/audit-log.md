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
- Reviewed paths: locallife/wechat/ecommerce.go; locallife/wechat/payment.go; locallife/wechat/interface.go; locallife/wechat/ecommerce_test.go
- Findings logged: BE-AUDIT-2026-04-13-01, BE-AUDIT-2026-04-13-02, BE-AUDIT-2026-04-13-03
- Durable docs updated: .github/review/open-findings.md
- Remaining scope: Caller-side error mapping, persistence propagation, UI presentation, and recovery scheduler behavior were intentionally left out of scope for this audit pass.

### 2026-04-14 - Applyment query usage propagation review

- Scope: Repository-wide review of production code that calls QueryEcommerceApplymentByOutRequestNo directly or consumes its upgraded status/field semantics in api, logic, worker, and persistence paths.
- Reviewed paths: locallife/wechat/ecommerce.go; locallife/api/ecommerce_applyment.go; locallife/api/ecommerce_applyment_test.go; locallife/logic/ecommerce_applyment_submission.go; locallife/logic/ecommerce_applyment_submission_test.go; locallife/worker/applyment_recovery_scheduler.go; locallife/worker/applyment_support.go; locallife/worker/task_process_payment.go; locallife/worker/applyment_recovery_scheduler_test.go; locallife/worker/applyment_support_test.go; locallife/db/migration/000197_expand_ecommerce_applyment_statuses.up.sql; locallife/db/query/ecommerce_applyment.sql
- Findings logged: BE-AUDIT-2026-04-14-01
- Durable docs updated: .github/review/open-findings.md
- Remaining scope: Frontend rendering and operator-facing workflows were not re-reviewed; test coverage for remote account_validation/legal_validation_url propagation remains a residual verification gap but no concrete production defect was proven there in this pass.

