---
name: "后端支付变更与审查模板"
description: "Use when drafting a WeChat payment or platform-ecommerce backend implementation or review request. Trigger phrases: change payment flow, 微信支付, 平台收付通, 商户进件, 进件申请单, 商户注销, 支付回调, review callback handling, 退款链路, 分账, 商户提现, 结算账户, 消费者投诉, adjust refund path, inspect audit ledger consistency, update payment domain guidance. 适用于发起微信支付、平台收付通、商户进件与高风险资金链路相关后端实现、审查与运维闭环任务。"
---
# Backend Payment Domain Template

Use this template when asking for changes or reviews related to WeChat payment, platform ecommerce, applyment, merchant account flows, or related payment-domain paths.

## Payment Change Request

Target area: `locallife/wechat/` or related backend payment and platform-ecommerce flows

Request:

- Implement or adjust <applyment, settlement account, merchant closeout, payment, refund, complaint, shipping, subsidy, withdraw, or payment-domain-related change>
- Name the active capability group first; do not treat the task as an isolated endpoint patch when the flow is part of an async or grouped platform-ecommerce capability
- Keep WeChat integration details inside the integration boundary and keep business decisions in logic or worker layers
- Confirm the official WeChat API purpose, request and response shape, required and conditional-required fields, field types, enums, statuses, and error codes before changing code; do not implement by memory
- Check `.github/standards/domains/wechat-payment/README.md` before editing callers; if the active capability group or official link set is missing, say so and update that README as part of the task
- If the active capability group is applyment, use the applyment section in `.github/standards/domains/wechat-payment/README.md` as the repo-internal routing baseline; do not let that replace the official API pages
- Tell me whether the change requires updates to payment-domain guidance, callback handling, config wiring, or audit records
- Run the smallest relevant validation command and report what was executed
- State whether callback signature verification, idempotency, recovery scheduling, and persisted auditability were actually checked or remain unverified
- State which caller, persistence, worker, scheduler, callback, and frontend consumers were reviewed for propagation, and whether any were intentionally left out of scope
- State how WeChat-side errors are logged and what clear caller-facing error semantics or operator guidance are returned

Optional context:

- Affected package or endpoint: <path>
- Payment, applyment, settlement, withdrawal, complaint, or callback flow involved: <details>
- Related docs: `.github/standards/domains/wechat-payment/README.md`
- Applyment-specific repo context when the capability group is applyment: use the applyment section in `.github/standards/domains/wechat-payment/README.md`

## Payment Review Request

Request:

- Review this payment-related change with findings first, ordered by severity
- Name the active capability group and check whether the author treated it as a grouped async flow instead of a single endpoint edit
- Check whether request flow, callback flow, worker flow, ledger or audit persistence, and operator-facing payment guidance all remain consistent
- Check whether the implementation really matches the official API purpose, request and response shape, required fields, conditional-required fields, field types, enums, states, and error codes instead of relying on guessed behavior
- Check whether the active capability group and official doc set are correctly represented in `.github/standards/domains/wechat-payment/README.md`, and whether callers outside `locallife/wechat/` were actually reviewed instead of assumed safe
- Flag hidden defaults, silent fallback behavior, missing callback propagation, or missing auditability
- Call out missing config validation, missing payment-domain guidance updates, and missing tests for failure or retry paths
- Call out missing structured logging or vague caller-facing error semantics for WeChat-side failures
- If rollout or execution-plan documents were touched, say whether they still belong in the active reference set or should be treated as historical rollout material

Optional context:

- Changed files or PR scope: <paths>
- Expected flow after the change: <details>