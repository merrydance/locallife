---
name: "后端支付变更与审查模板"
description: "Use when drafting a WeChat, Baofoo/Baofu, platform-ecommerce, or external payment backend implementation or review request. Trigger phrases: change payment flow, 微信支付, 宝付, Baofu, Baofoo, 宝财通, account opening, 开户, 平台收付通, 商户进件, 进件申请单, 商户注销, 支付回调, review callback handling, 退款链路, 分账, 商户提现, 结算账户, 消费者投诉, adjust refund path, inspect audit ledger consistency, update payment domain guidance. 适用于发起微信支付、宝付宝财通、平台收付通、商户进件与高风险资金链路相关后端实现、审查与运维闭环任务。"
---
# Backend Payment Domain Template

Use this template when asking for changes or reviews related to WeChat payment, Baofoo/Baofu BaoCaiTong, platform ecommerce, applyment, merchant account flows, or related payment-domain paths.

Choose the provider domain README first:

- WeChat payment and platform-ecommerce work: `.github/standards/domains/wechat-payment/README.md`
- Baofoo/Baofu/BaoCaiTong account, merchant-report, aggregate payment, share, refund, withdrawal, or callback work: `.github/standards/domains/baofu-payment/README.md`

Use `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md` as the shared backend rule for provider contract truth, field matrices, error mapping, explicit downgrade, and drift review.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the active provider and capability group before writing the request. Do not keep relying on stale context.

## Payment Change Request

Target area: `locallife/wechat/`, `locallife/baofu/`, or related backend payment and platform-ecommerce flows

Request:

- Implement or adjust <applyment, settlement account, account opening, merchant report, aggregate payment, payment, refund, complaint, shipping, subsidy, share, withdraw, callback, or payment-domain-related change>
- Name the active provider and capability group first; do not treat the task as an isolated endpoint patch when the flow is part of an async or grouped external-payment capability
- Keep provider integration details inside the integration boundary and keep business decisions in logic or worker layers
- Confirm the official provider API purpose, request and response shape, required and conditional-required fields, field types, enums, statuses, and error codes before changing code; do not implement by memory
- Check the matching provider domain README before editing callers. If the active capability group, source matrix, field matrix, or official link set is missing, say so and update the provider domain standard as part of the task
- If the active capability group is applyment, use the applyment section in `.github/standards/domains/wechat-payment/README.md` as the repo-internal routing baseline; do not let that replace the official API pages
- If the active capability group is Baofoo/Baofu/BaoCaiTong, use `.github/standards/domains/baofu-payment/CONTRACT_SOURCE_MATRIX.md` and `.github/standards/domains/baofu-payment/BAOCAITONG_FIELD_CONTRACT_MATRIX.md` as the field-drift baseline before editing DTOs, parsers, request builders, callbacks, or smoke scripts
- Tell me whether the change requires updates to payment-domain guidance, callback handling, config wiring, or audit records
- Run the smallest relevant validation command and report what was executed
- State whether callback signature verification, idempotency, recovery scheduling, and persisted auditability were actually checked or remain unverified
- State which caller, persistence, worker, scheduler, callback, and frontend consumers were reviewed for propagation, and whether any were intentionally left out of scope
- State how provider-side errors are logged and what clear caller-facing error semantics or operator guidance are returned

Optional context:

- Affected package or endpoint: <path>
- Payment, applyment, account-opening, settlement, withdrawal, complaint, share, refund, merchant-report, or callback flow involved: <details>
- Related docs: `.github/standards/domains/wechat-payment/README.md` or `.github/standards/domains/baofu-payment/README.md`
- Applyment-specific repo context when the capability group is applyment: use the applyment section in `.github/standards/domains/wechat-payment/README.md`
- Baofoo/Baofu-specific repo context: use `.github/standards/domains/baofu-payment/README.md`

## Payment Review Request

Request:

- Review this payment-related change with findings first, ordered by severity
- Name the active provider and capability group and check whether the author treated it as a grouped async flow instead of a single endpoint edit
- Check whether request flow, callback flow, worker flow, ledger or audit persistence, and operator-facing payment guidance all remain consistent
- Check whether the implementation really matches the official API purpose, request and response shape, required fields, conditional-required fields, field types, enums, states, and error codes instead of relying on guessed behavior
- Check whether the active capability group and official doc set are correctly represented in the matching provider domain README, and whether callers outside the provider boundary such as `locallife/wechat/` or `locallife/baofu/` were actually reviewed instead of assumed safe
- For Baofoo/Baofu work, check whether `CONTRACT_SOURCE_MATRIX.md`, `BAOCAITONG_FIELD_CONTRACT_MATRIX.md`, `API_CONTRACT_COVERAGE_AUDIT.md`, and `make check-baofu-contract` remain aligned with the code change
- Flag hidden defaults, silent fallback behavior, missing callback propagation, or missing auditability
- Call out missing config validation, missing payment-domain guidance updates, and missing tests for failure or retry paths
- Call out missing structured logging or vague caller-facing error semantics for provider-side failures
- If rollout or execution-plan documents were touched, say whether they still belong in the active reference set or should be treated as historical rollout material

Optional context:

- Changed files or PR scope: <paths>
- Expected flow after the change: <details>
