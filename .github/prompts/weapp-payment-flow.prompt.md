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
- Make loading, disabled, pending, success, failed, cancelled, and retry behavior explicit where relevant
- Prevent duplicate submission and make uncertain payment states visible to users
- Run the smallest relevant validation command and report what was executed

Required context:

- Target page or service path: <path>
- Expected payment or recovery behavior: <details>

Optional context:

- Related backend payment callback or polling path: <path>
- Known weak-network or re-entry issue: <details>
- Reference flow or page: <path>

Acceptance checklist:

- Login expiry and recovery are connected to the payment path
- Duplicate taps, stale polling, and delayed confirmation states are handled deliberately
- User-facing copy distinguishes success, failure, cancellation, and unknown result states
- Service calls, page state, event handlers, and view feedback stay wired end to end