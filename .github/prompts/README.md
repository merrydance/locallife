# Prompt Templates Index

This directory contains reusable prompt templates with normalized names.

## Naming Rule

- Use `<scope>-<intent>.prompt.md`
- Use `general-` for cross-workspace prompts
- Use `backend-`, `web-`, or `weapp-` when the prompt is area-specific

## Current Templates

The files listed here are the active reusable prompt set for this workspace. Prompt files that are not listed here should be treated as drift: either delete them, fold them into an indexed reusable prompt, or relocate the task-specific content outside `.github/prompts/`.

- `general-implementation.prompt.md`
- `general-incident-followup.prompt.md`
- `general-review.prompt.md`
- `backend-implementation.prompt.md`
- `backend-task-card-implementation.prompt.md`
- `backend-phase-batch-implementation.prompt.md`
- `backend-review-closure.prompt.md`
- `backend-sql-review.prompt.md`
- `backend-integration-test.prompt.md`
- `backend-payment-runbook.prompt.md`
- `business-flow-mermaid.prompt.md`
- `general-task-loop.prompt.md`
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
3. If the request is backend SQL or migration review-focused, use `backend-sql-review.prompt.md`.
4. If the request is backend task-card or phase-map driven, use the matching task-card or phase prompt.
5. If the request is an ordered task list that should be completed through implement, review, fix, and doc-sync stages, use `general-task-loop.prompt.md`.
6. If the request is a Mini Program payment flow, use `weapp-payment-flow.prompt.md`.
7. If the request is a Mini Program refactor blueprint before implementation, use `weapp-page-refactor-blueprint.prompt.md`.
8. If the request is about incident follow-up, escaped defect closure, or turning a postmortem into standards/gates/tests, use `general-incident-followup.prompt.md`.
9. Otherwise use the area-specific implementation or review prompt.
10. Use `general-implementation.prompt.md` or `general-review.prompt.md` only when the request spans multiple areas or the target area is still ambiguous.

## Prompt Boundaries

- `general-implementation.prompt.md`: multi-area or not-yet-scoped implementation requests.
- `general-incident-followup.prompt.md`: cross-area incident/postmortem follow-up that must convert findings into standards, prompts, workflows, tests, or runbooks.
- `general-review.prompt.md`: multi-area or not-yet-scoped review requests.
- `general-task-loop.prompt.md`: ordered task lists that should be executed through an implement-review-fix-doc loop until complete.
- `backend-implementation.prompt.md`: normal backend feature or bug-fix work outside payment-specialized or task-card-specialized flows.
- `backend-sql-review.prompt.md`: backend SQL, migration, sqlc propagation, index, or persistence-focused review requests.
- `backend-payment-runbook.prompt.md`: WeChat payment, callback, refund, runbook, or audit-ledger work.
- `weapp-implementation.prompt.md`: generic Mini Program page or component changes outside payment-specialized flows.
- `weapp-payment-flow.prompt.md`: Mini Program payment, login recovery after pay, result state, retry, or duplicate-tap guarding.
- `weapp-review.prompt.md`: Mini Program review requests, including overall upgrade audits that must combine current standards with historical blueprint-based gap detection.
- `weapp-page-refactor-blueprint.prompt.md`: request generation for a diagnosis-first refactor blueprint before coding.

Cross-cutting governance rule:

- When using `general-*` prompts for high-risk or cross-surface work, include a risk classification guess, validation scope, and expected residual-risk or release-readiness output instead of treating the task as routine.

## Routing Test Cases

Use these prompts to sanity-check description routing after future edits:

1. "实现一个后端接口，补齐 handler 到 sqlc 的闭环，并告诉我要不要 make sqlc。"
Expected target: `backend-implementation.prompt.md`

2. "审查这个 PR，重点看 DTO 字段改了之后有没有一路传到 handler、logic、sqlc 和测试。"
Expected target: `backend-review-closure.prompt.md`

3. "给微信支付回调和退款链路做一次实现和审查请求模板。"
Expected target: `backend-payment-runbook.prompt.md`

4. "审查这个 db/query 和 migration 变更，重点看 sqlc 传播、索引遗漏和事务风险。"
Expected target: `backend-sql-review.prompt.md`

5. "给这个小程序页面做重构蓝图，先诊断 setData 热点和弱网体验，再给实施方案。"
Expected target: `weapp-page-refactor-blueprint.prompt.md`

6. "修一下小程序支付完成后返回页状态丢失和重复点击支付的问题。"
Expected target: `weapp-payment-flow.prompt.md`

7. "改一下小程序页面的列表空态和错误态。"
Expected target: `weapp-implementation.prompt.md`

8. "从整体升级角度审查一下 weapp 的交互和风格，既看现行规范，也看历史蓝图里反复出现的问题。"
Expected target: `weapp-review.prompt.md`

9. "这个需求要同时改 backend 和 web，帮我整理一份实现请求。"
Expected target: `general-implementation.prompt.md`

10. "把这段报销审批流程整理成 Mermaid，补上驳回和超时分支。"
Expected target: `business-flow-mermaid.prompt.md`

11. "把这组任务按 开发 -> review -> 修复 -> review -> 文档同步 的顺序跑完，直到任务清单完成。"
Expected target: `general-task-loop.prompt.md`

12. "把这次线上事故的结论落成规则、workflow、测试和 runbook 更新清单。"
Expected target: `general-incident-followup.prompt.md`

## Maintenance Rule

When a new prompt is added, add one routing test case here if it introduces a new trigger surface. If it cannot justify a distinct test case, it probably should not be a separate prompt.

Do not keep one-off planning or implementation prompts under `.github/prompts/` after the task ends. If a prompt is too task-specific to index here, remove it instead of leaving an orphan file in the routing hot path.

When engineering governance rules under `.github/standards/engineering/` change, re-check whether `general-implementation`, `general-review`, and `general-task-loop` still ask for the right risk, validation, and hand-off information.