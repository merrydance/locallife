# Prompt Templates Index

This directory contains reusable prompt templates with normalized names.

## Naming Rule

- Use `<scope>-<intent>.prompt.md`
- Use `general-` for cross-workspace prompts
- Use `backend-`, `web-`, or `weapp-` when the prompt is area-specific

## Current Templates

- `general-implementation.prompt.md`
- `general-review.prompt.md`
- `backend-implementation.prompt.md`
- `backend-task-card-implementation.prompt.md`
- `backend-phase-batch-implementation.prompt.md`
- `backend-review-closure.prompt.md`
- `backend-integration-test.prompt.md`
- `backend-payment-runbook.prompt.md`
- `business-flow-mermaid.prompt.md`
- `web-implementation.prompt.md`
- `web-review.prompt.md`
- `weapp-implementation.prompt.md`
- `weapp-page-refactor-blueprint.prompt.md`
- `weapp-payment-flow.prompt.md`
- `weapp-review.prompt.md`

## Usage Rule

Prefer the most specific prompt template that matches the task. If the task is general and spans multiple areas, start with a `general-` template and add concrete context paths.

## Routing Order

Use this order to avoid prompt collisions:

1. If the request is diagramming only, use `business-flow-mermaid.prompt.md`.
2. If the request is backend payment-specific, use `backend-payment-runbook.prompt.md`.
3. If the request is backend task-card or phase-map driven, use the matching task-card or phase prompt.
4. If the request is a Mini Program payment flow, use `weapp-payment-flow.prompt.md`.
5. If the request is a Mini Program refactor blueprint before implementation, use `weapp-page-refactor-blueprint.prompt.md`.
6. Otherwise use the area-specific implementation or review prompt.
7. Use `general-implementation.prompt.md` or `general-review.prompt.md` only when the request spans multiple areas or the target area is still ambiguous.

## Prompt Boundaries

- `general-implementation.prompt.md`: multi-area or not-yet-scoped implementation requests.
- `general-review.prompt.md`: multi-area or not-yet-scoped review requests.
- `backend-implementation.prompt.md`: normal backend feature or bug-fix work outside payment-specialized or task-card-specialized flows.
- `backend-payment-runbook.prompt.md`: WeChat payment, callback, refund, runbook, or audit-ledger work.
- `weapp-implementation.prompt.md`: generic Mini Program page or component changes outside payment-specialized flows.
- `weapp-payment-flow.prompt.md`: Mini Program payment, login recovery after pay, result state, retry, or duplicate-tap guarding.
- `weapp-page-refactor-blueprint.prompt.md`: request generation for a diagnosis-first refactor blueprint before coding.

## Routing Test Cases

Use these prompts to sanity-check description routing after future edits:

1. "实现一个后端接口，补齐 handler 到 sqlc 的闭环，并告诉我要不要 make sqlc。"
Expected target: `backend-implementation.prompt.md`

2. "审查这个 PR，重点看 DTO 字段改了之后有没有一路传到 handler、logic、sqlc 和测试。"
Expected target: `backend-review-closure.prompt.md`

3. "给微信支付回调和退款链路做一次实现和审查请求模板。"
Expected target: `backend-payment-runbook.prompt.md`

4. "给这个小程序页面做重构蓝图，先诊断 setData 热点和弱网体验，再给实施方案。"
Expected target: `weapp-page-refactor-blueprint.prompt.md`

5. "修一下小程序支付完成后返回页状态丢失和重复点击支付的问题。"
Expected target: `weapp-payment-flow.prompt.md`

6. "改一下小程序页面的列表空态和错误态。"
Expected target: `weapp-implementation.prompt.md`

7. "这个需求要同时改 backend 和 web，帮我整理一份实现请求。"
Expected target: `general-implementation.prompt.md`

8. "把这段报销审批流程整理成 Mermaid，补上驳回和超时分支。"
Expected target: `business-flow-mermaid.prompt.md`

## Maintenance Rule

When a new prompt is added, add one routing test case here if it introduces a new trigger surface. If it cannot justify a distinct test case, it probably should not be a separate prompt.