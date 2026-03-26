---
applyTo: "locallife/wechat/**"
---

# Backend WeChat Instructions

Apply these rules for files under `locallife/wechat/`.

## Read First

- `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_REFACTOR_EXECUTION_PLAN_2026-03-24.md`
- `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`
- `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_COMPLAINT_SUBSIDY_FRONTEND_SPEC_2026-03-22.md`

## Role Of This Layer

- Keep this package responsible for WeChat-facing client integrations such as payment, ecommerce, shipping, complaint, bill, wxacode, and related security or content-check helpers.
- Use this package as an external integration boundary, not as a place to hide unrelated business workflows that should stay in logic, worker, or scheduler layers.

## Integration Conventions

- Preserve explicit client and interface boundaries so callers depend on stable integration interfaces instead of concrete implementation details.
- Keep request signing, transport details, and provider-specific error handling inside this integration boundary.
- Reuse existing payment, complaint, subsidy, and shipping client patterns instead of inventing a parallel client style for one endpoint.
- Keep business status transitions, ledger updates, and domain decisions outside this package unless they are strictly required to shape an external request or response.

## Boundary Checks

- Changes to WeChat payment flows should be reflected in upstream logic, callbacks, worker processing, and audit records rather than stopping inside the client package.
- If config, credential, or app/mch routing assumptions change, update the corresponding standards and validation paths instead of relying on implicit defaults.
- Payment or refund behavior should stay auditable and traceable through persisted records rather than becoming an in-memory side effect.

## Validation Defaults

- Prefer focused unit tests for client, payment, callback, and error-mapping behavior.
- If a change affects payment runbooks, service-provider config, or callbacks, update the linked domain standards alongside code.