---
name: "微信小程序支付链路请求模板"
description: "Use when drafting a Mini Program payment-flow implementation or review request. Trigger phrases: fix payment flow, review pay result state, login recovery after pay, duplicate tap guard, weak-network pay retry, privacy consent before pay. 适用于发起微信小程序支付、登录恢复、支付结果回跳与重试语义相关任务。"
---
# Mini Program Payment Flow Template

Use this template when asking for a Mini Program payment or payment-adjacent flow change in `weapp/`.

## Payment Flow Change

Request:

- Implement or adjust <payment, retry, login recovery, privacy consent, subscribe message, or payment-result handling flow>
- Treat payment as a full user flow from page entry to result rendering, not only a button action
- Align any touched payment or order API usage with the real backend contract before changing page logic, and do not invent unsupported statuses, fields, or recovery branches
- Make loading, disabled, pending, success, failed, cancelled, and retry behavior explicit where relevant
- Prevent duplicate submission and make uncertain payment states visible to users
- Keep order page, payment page, result page, and history page semantics consistent when they represent the same payment state
- Run the smallest relevant validation command and report what was executed

Required context:

- Target page or service path: <path>
- Expected payment or recovery behavior: <details>
- User role and transaction goal: <details>

Optional context:

- Related backend payment callback or polling path: <path>
- Known weak-network or re-entry issue: <details>
- Payment preconditions or consent requirements: <details>
- Reference flow or page: <path>

Acceptance checklist:

- Login expiry and recovery are connected to the payment path
- Duplicate taps, stale polling, and delayed confirmation states are handled deliberately
- User-facing copy distinguishes success, failure, cancellation, and unknown result states
- Service calls, page state, event handlers, and view feedback stay wired end to end
- Leaving the app, returning from WeChat pay, or re-entering the page can reconnect to the correct payment state
- Unknown result states provide a credible next step such as status recheck, delayed confirmation guidance, or safe retry rules

Cross-page consistency checks:

- The same order or payment record is not shown as conflicting states across entry, result, and history surfaces
- If the final state is asynchronous, the UI makes clear which page owns the latest trustworthy status