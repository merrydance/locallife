---
name: "后端支付变更与审查模板"
description: "Use when drafting a WeChat payment or platform-ecommerce backend implementation or review request. Trigger phrases: change payment flow, 微信支付, 平台收付通, 商户进件, 进件申请单, 商户注销, 支付回调, review callback handling, 退款链路, 分账, 商户提现, 结算账户, 消费者投诉, adjust refund path, inspect audit ledger consistency, update payment runbook. 适用于发起微信支付、平台收付通、商户进件与高风险资金链路相关后端实现、审查与运维闭环任务。"
---
# Backend Payment Runbook Template

Use this template when asking for changes or reviews related to WeChat payment, platform ecommerce, applyment, merchant account flows, or related operational paths.

## Payment Change Request

Target area: `locallife/wechat/` or related backend payment and platform-ecommerce flows

Request:

- Implement or adjust <applyment, settlement account, merchant closeout, payment, refund, complaint, shipping, subsidy, withdraw, or runbook-related change>
- Keep WeChat integration details inside the integration boundary and keep business decisions in logic or worker layers
- Confirm the official WeChat API purpose, request and response shape, required and conditional-required fields, field types, enums, statuses, and error codes before changing code; do not implement by memory
- Tell me whether the change requires updates to payment runbooks, callback handling, config wiring, or audit records
- Run the smallest relevant validation command and report what was executed
- State whether callback signature verification, idempotency, recovery scheduling, and persisted auditability were actually checked or remain unverified
- State how WeChat-side errors are logged and what clear caller-facing error semantics or operator guidance are returned

Optional context:

- Affected package or endpoint: <path>
- Payment, applyment, settlement, withdrawal, complaint, or callback flow involved: <details>
- Related docs: `.github/standards/domains/wechat-payment/README.md`, `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OFFICIAL_API_BASELINE_2026-04-14.md`, `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`

Use `.github/standards/domains/wechat-payment/historical/WECHAT_PAYMENT_REFACTOR_EXECUTION_PLAN_2026-03-24.md` only when the task changes historical rollout assumptions, stage ownership, or migration baseline.

## Payment Review Request

Request:

- Review this payment-related change with findings first, ordered by severity
- Check whether request flow, callback flow, worker flow, ledger or audit persistence, and operator runbook expectations all remain consistent
- Check whether the implementation really matches the official API purpose, request and response shape, required fields, conditional-required fields, field types, enums, states, and error codes instead of relying on guessed behavior
- Flag hidden defaults, silent fallback behavior, missing callback propagation, or missing auditability
- Call out missing config validation, missing runbook updates, and missing tests for failure or retry paths
- Call out missing structured logging or vague caller-facing error semantics for WeChat-side failures
- If rollout or execution-plan documents were touched, say whether they still belong in the active reference set or should be treated as historical rollout material

Optional context:

- Changed files or PR scope: <paths>
- Expected flow after the change: <details>