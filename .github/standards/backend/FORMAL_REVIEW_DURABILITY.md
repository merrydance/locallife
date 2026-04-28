# Formal Review Durability

> 作用：把 backend 的正式 review、subsystem audit 和 takeover-grade 风险审查，从一次性对话，转成可被后续成员继续使用的 durable project knowledge。

## 1. When To Use

本文件适用于以下类型的 backend 审查：

- 高风险路径 review
- subsystem audit
- takeover-grade risk assessment
- 暴露系统性缺口的正式 PR review

普通小修小补 review 不必把每次结果都写成冗长台账，但高价值发现不能只留在聊天记录里。

## 2. Required Durable Outputs

### 2.1 Active findings

正式 review 至少要有一个可继续跟踪 unresolved findings 的位置。

当前后端仓库已有 active ledger：

- `.github/review/open-findings.md`

每条 finding 至少包含：

- ID
- date
- severity
- subsystem
- concrete file references
- concise risk statement
- next action
- current status

### 2.2 Durable patterns

如果 finding 暴露的是可重复 bug class，而不是单点瑕疵，应更新对应长期规则，例如：

- `.github/standards/backend/README.md`
- 匹配的 domain README
- `.github/instructions/backend-locallife.instructions.md`
- `.github/prompts/backend-bugfix.prompt.md`
- `.github/prompts/backend-review-closure.prompt.md`
- 相关 workflow、脚本、测试或 runbook

典型模式包括：

- transaction boundary drift
- ownership checks inconsistent across entrypoints
- worker/scheduler paths forgotten during fixes
- generated artifacts frequently left stale
- silent error swallowing or nil-as-success fallbacks
- missing structured logging boundary for unexpected failures
- caller-facing error semantics that leak internal detail or stay too vague to act on

### 2.3 Audit run log

正式审查还应记录一次 durable audit pass。

当前后端仓库已有 audit log：

- `.github/review/audit-log.md`

至少记录：

- scope
- reviewed paths
- findings logged or none
- whether durable docs were updated
- remaining unreviewed scope

## 3. Closeout Rule

正式 backend review 不应在以下问题不清楚时宣称完成：

1. unresolved findings 是否已结构化记录
2. recurring pattern 是否已反馈到标准、instructions、prompts、workflow、测试或 runbook
3. 本轮 scope 与未覆盖范围是否对后续接手者清晰可见

## 4. Review Scope Bias

对 LocalLife backend，正式审查默认偏向：

- `api -> logic -> db/sqlc -> worker/scheduler/webhook`
- funds and state transitions
- object ownership and role boundaries
- regeneration and test-coverage gaps
- residual risk explicitly named instead of vague caveats

## 5. Practical Rule

如果某条 backend review finding 的价值高到足以在未来再次节省事故、回归或审查成本，它就不应该只留在一次聊天输出里。

对错误处理类 findings，durable 写回默认至少覆盖三件事中的一件，必要时同时覆盖：

- 标准或 instructions：明确业务错误、基础设施错误、日志边界和对前端语义的长期规则。
- prompts 或 checklist：让实现与 review 在默认入口就会主动检查静默吞错、5xx 跳过日志和错误语义泄漏。
- workflow、guard 或 focused tests：把高信噪比反模式做成可执行门禁，而不是完全依赖 reviewer 记忆。

