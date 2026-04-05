# 小程序 9 份核心文档改写蓝本

## 1. 使用方式

本蓝本用于指导后续逐份重写当前小程序相关的 instructions、standards 和 prompts。

每份文档统一给出：

- 目标角色
- 当前主要问题
- 保留内容
- 删除或迁移内容
- 必须新增内容
- 建议的新结构

## 2. 改写优先级

优先级从高到低：

1. `.github/standards/weapp/api/README.md`
2. `.github/standards/weapp/DESIGN_SYSTEM.md`
3. `.github/instructions/weapp-mini-program.instructions.md`
4. `.github/instructions/weapp-pages.instructions.md`
5. `.github/instructions/weapp-components.instructions.md`
6. `.github/prompts/weapp-implementation.prompt.md`
7. `.github/prompts/weapp-review.prompt.md`
8. `.github/prompts/weapp-page-refactor-blueprint.prompt.md`
9. `.github/prompts/weapp-payment-flow.prompt.md`

## 3. 文档逐份蓝本

### 3.1 `.github/instructions/weapp-mini-program.instructions.md`

目标角色：

- 小程序目录级入口执行 instruction

当前主要问题：

- 同时承担了设计标准、提示系统摘要和执行清单三种角色。
- 与 `USER_FEEDBACK_STANDARDS.md`、`DESIGN_SYSTEM.md`、`miniprogram-prompt-system.md` 存在正文重复。
- 规则很多，但执行优先级没有被压缩成最小必要集。

保留内容：

- Read First
- High-Risk Anti-Patterns
- Validation Defaults
- Scope Reminders

删除或迁移内容：

- 将大部分 UI 规则迁回 `DESIGN_SYSTEM.md`
- 将提示规则引用到 `USER_FEEDBACK_STANDARDS.md` 和 `miniprogram-prompt-system.md`
- 将全路径完整性解释迁回 `INTERACTION_STANDARDS.md` 和 `API_INTERACTION_CONTRACT.md`

必须新增内容：

- 首屏失败、局部刷新失败、静默回读失败的区分要求
- 页面切前后台、重新进入、登录恢复的统一恢复要求
- 统一的重复点击与提交中状态约束

建议的新结构：

```text
# Mini Program Instructions
## Read First
## Directory Role
## Must-Follow Rules
## High-Risk Anti-Patterns
## Validation Defaults
## Scope Reminders
```

### 3.2 `.github/instructions/weapp-pages.instructions.md`

目标角色：

- 页面层执行 instruction

当前主要问题：

- 更像风险清单，不像页面体验约束。
- 页面状态机、回退恢复、滚动稳定性、输入态体验等还没有形成规则。

保留内容：

- Role Of This Surface
- Page Rules

删除或迁移内容：

- 与总 instruction 重复的 TDesign 与 token 总规则
- 可由交互标准统一描述的通用弱网原则

必须新增内容：

- 页面状态机建议：首屏、局部刷新、提交中、回读失败、已缓存数据
- 搜索页、列表页、详情页、表单页的差异化要求
- 滚动位置保留、输入法遮挡、底部操作栏与内容区关系
- 用户返回页面后的状态恢复策略

建议的新结构：

```text
# Mini Program Pages Instructions
## Role Of This Surface
## Page State Rules
## Task Flow Rules
## Data And Interaction Rules
## Recovery Rules
## Validation Defaults
```

### 3.3 `.github/instructions/weapp-components.instructions.md`

目标角色：

- 共享组件层执行 instruction

当前主要问题：

- 只有通用原则，缺少组件 API 与事件契约约束。
- 对可复用组件应如何表达 loading、empty、disabled 等语义说明不足。

保留内容：

- Role Of This Layer
- Component Rules

删除或迁移内容：

- 与总 instruction 重复的 token 和 TDesign 选择说明

必须新增内容：

- props 命名、事件命名、默认值、禁用态、空态、加载态规则
- 共享组件不得持有业务路由决策和页面级接口请求的要求
- 共享组件允许的样式外扩方式
- 图标、热区、长文案截断、图片失败态的基础可用性规则

建议的新结构：

```text
# Mini Program Components Instructions
## Role Of This Layer
## Component Contract Rules
## State Semantics
## Styling Boundaries
## Validation Defaults
```

### 3.4 `.github/standards/weapp/DESIGN_SYSTEM.md`

目标角色：

- 小程序基础设计系统

当前主要问题：

- 设计系统、交互标准、执行清单、代码审查项混在一起。
- 既有基础 token，也有交付验收与服务贯通检查，职责过重。

保留内容：

- 设计原则
- token 系统
- 卡片、按钮、标签、弹层、页面骨架等基础模式
- 标题副标题层级

删除或迁移内容：

- 规范优先级与同步来源，迁到 `README.md` 或治理文档
- 组件库使用指南中的执行口径，保留简版即可
- 开发最佳实践，迁到 instructions
- 交互架构规范，迁到 `INTERACTION_STANDARDS.md`
- 页面与服务贯通检查，迁到 `INTERACTION_STANDARDS.md` 或 `API_INTERACTION_CONTRACT.md`
- 共享组件与页面边界，迁到 components instruction
- 禁止出现的退化模式与 Quality Gate，迁到 instructions 和 review prompt

必须新增内容：

- 触控热区下限
- 文本层级与信息密度建议
- 输入可读性、键盘遮挡、长列表视线组织的基础 UX 规则
- 动效节奏与骨架屏一致性约束

建议的新结构：

```text
# LocalLife Mini Program Design System
## Scope
## Foundations
## Tokens
## Layout Patterns
## Component Patterns
## Readability And Touch Targets
## Motion And Skeleton Guidance
```

### 3.5 `.github/standards/weapp/api/README.md`

目标角色：

- 历史迁移文档，不再作为标准

当前主要问题：

- 当前文件是重构纪要和迁移记录，不是长期标准。
- 含有“已完成”“下一步计划”“兼容性处理”等强时效内容。
- 被纳入 Read First 会误导后续实现者。

保留内容：

- 作为历史记录保留当前改造背景与迁移成果

删除或迁移内容：

- 不再作为 active standard 被引用
- 从 Read First 中移除

必须新增内容：

- 新建 `API_INTERACTION_CONTRACT.md` 取代其标准职责，覆盖：
  - 分页真值
  - 登录失效恢复
  - 乐观更新边界
  - 异步作业与 OCR 回填承接
  - 支付轮询与未知状态承接
  - 错误映射责任
  - 防重入和重试策略

建议的新结构：

```text
# WeApp API Refactor Historical Note
## Background
## What Changed
## Compatibility Notes
## Historical Risks
```

### 3.6 `.github/prompts/weapp-implementation.prompt.md`

目标角色：

- 常规实现任务的 AI 输入模板

当前主要问题：

- 输入过于工程化，缺少用户任务上下文。
- 验收项更关注闭环，较少关注交互质量。

保留内容：

- Target page or component path
- Desired behavior or UX change
- Reference page or service context
- 基础 acceptance checklist

删除或迁移内容：

- 不再重复标准正文，只保留引用

必须新增内容：

- 用户角色
- 任务频率
- 首访还是高频复访
- 核心成功标准
- 弱网敏感度
- 是否需要保留当前页面状态或滚动位置

建议的新结构：

```text
# Mini Program Implementation Template
## Scope
## Required Context
## UX Context
## Constraints
## Acceptance Checklist
```

### 3.7 `.github/prompts/weapp-review.prompt.md`

目标角色：

- 小程序代码与交互审查模板

当前主要问题：

- 现在更像代码连通性审查模板，不够像交互体验审查模板。
- 缺少认知负担、主次操作、空态可行动性、返回恢复等维度。

保留内容：

- findings first
- 高风险路径显式指出
- 若无问题需说明残余风险

删除或迁移内容：

- 不再承载过多 design system 正文

必须新增内容：

- 审查输出需区分代码缺陷、交互缺陷、残余风险
- 主次操作是否清晰
- 空态与错误态是否给出下一步
- 页面重入和返回后是否能恢复用户上下文
- 是否降低了认知负担和步骤复杂度

建议的新结构：

```text
# Mini Program Review Template
## Request
## Required Context
## Optional UX Context
## Review Dimensions
## Output Rules
```

### 3.8 `.github/prompts/weapp-page-refactor-blueprint.prompt.md`

目标角色：

- 诊断优先的页面重构蓝图模板

当前主要问题：

- 要求量化评分，但评分维度没有固定。
- 没有明确“不在本轮范围内的项”，容易让重构失控。

保留内容：

- 任务背景
- 诊断书
- 高分方案
- 实施要求

删除或迁移内容：

- 无需删除主体结构，重点是补齐评分框架

必须新增内容：

- 固定评分维度：信息架构、状态设计、交互效率、异常恢复、性能负担、组件边界
- 用户任务路径图
- 明确不改范围
- 明确优先优化项和可延后项

建议的新结构：

```text
# WeApp Page Refactor Blueprint
## Task Background
## User Task Path
## Diagnosis With Scores
## Blueprint
## Non-Goals
## Implementation Requirements
```

### 3.9 `.github/prompts/weapp-payment-flow.prompt.md`

目标角色：

- 支付及支付邻接流程的 AI 任务模板

当前主要问题：

- 已经覆盖较多状态，但跨页一致性和前后台切换恢复仍不够明确。
- 对“未知结果”后的承接与用户下一步指导还不够具体。

保留内容：

- 将支付视为完整任务流，而不是按钮动作
- loading、pending、success、failed、cancelled、retry 显式化
- duplicate taps 与 stale polling 的处理要求

删除或迁移内容：

- 无需删除主体结构，重点是补强状态恢复与跨页一致性

必须新增内容：

- 支付前置条件检查
- 支付时切后台或跳出小程序后的恢复要求
- 轮询超时和回调延迟时的承接页规范
- 未知结果状态的文案、按钮与回查策略
- 订单页、支付页、结果页、历史页之间的一致性检查

建议的新结构：

```text
# Mini Program Payment Flow Template
## Flow Scope
## Required Context
## Failure And Recovery Context
## Acceptance Checklist
## Cross-Page Consistency Checks
```

## 4. 新增文档建议

为了完成上述改写，建议新增以下文档，而不是继续把所有内容压在旧文档里：

- `.github/standards/weapp/README.md`
- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`

## 5. 逐步实施顺序

### Step 1

- 先新增 `README.md`、`INTERACTION_STANDARDS.md`、`API_INTERACTION_CONTRACT.md`

### Step 2

- 改写 `DESIGN_SYSTEM.md`，只保留 foundation 内容

### Step 3

- 改写三个 instructions，使其变成瘦而准的执行约束

### Step 4

- 改写四个 weapp prompts，补齐体验输入项和输出结构

### Step 5

- 将 `.github/standards/weapp/api/README.md` 迁为 historical 文档

## 6. 完成后的理想状态

完成重构后，团队在任何一个问题上都能迅速回答以下三个问题：

- 这条规则的唯一权威文档是什么。
- 这是设计基础、交互标准、API 契约，还是执行 instruction。
- 这是长期标准，还是阶段性历史材料。

只要这三个问题能被稳定回答，后续小程序体验和工程约束就会更容易长期演进。