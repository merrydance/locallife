# Backend Payment Runbook Template

Use this template when asking for changes or reviews related to WeChat payment and related operational flows.

## Payment Change Request

Target area: `locallife/wechat/` or related backend payment flows

Request:

- Implement or adjust <payment, refund, complaint, shipping, subsidy, or runbook-related change>
- Keep WeChat integration details inside the integration boundary and keep business decisions in logic or worker layers
- Tell me whether the change requires updates to payment runbooks, callback handling, config wiring, or audit records
- Run the smallest relevant validation command and report what was executed

Optional context:

- Affected package or endpoint: <path>
- Payment flow or callback involved: <details>
- Related docs: `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_REFACTOR_EXECUTION_PLAN_2026-03-24.md`, `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`

## Payment Review Request

Request:

- Review this payment-related change with findings first, ordered by severity
- Check whether request flow, callback flow, worker flow, ledger or audit persistence, and operator runbook expectations all remain consistent
- Flag hidden defaults, silent fallback behavior, missing callback propagation, or missing auditability
- Call out missing config validation, missing runbook updates, and missing tests for failure or retry paths

Optional context:

- Changed files or PR scope: <paths>
- Expected flow after the change: <details>