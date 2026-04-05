# 小程序 AI 文档分层重构方案

## 1. 目标

本方案用于重构 LocalLife 小程序相关的 instructions、prompts、standards 与历史说明文档，解决以下问题：

- 规则重复分散，权威边界不清。
- design system 混入执行约束、评审清单、服务贯通检查，职责过重。
- API 文档当前是重构纪要，不是长期有效的交互契约。
- prompts 更偏工程补线，缺少用户任务与交互质量输入。
- instructions 同时承担了标准文档和执行清单两种角色，导致后续维护成本持续上升。

本方案的目标不是一次性重写所有文档，而是先建立清晰层级，再按层级回填内容。

## 2. 重构原则

### 2.1 唯一权威

- 同一类规则只能有一个主文档。
- instructions 只做执行约束，不重复 standards 正文。
- prompts 只负责任务路由与任务输入，不承载长期规则正文。
- 历史改造说明不进入默认热路径。

### 2.2 体验优先但不牺牲工程闭环

- 保留现有体系中对全链路闭环、假成功、分页真值、弱网重试、提示去重的强约束。
- 新体系必须补齐任务流体验、状态设计、表单体验、回退恢复、跨页一致性等更高阶约束。

### 2.3 分层而不是叠层

- 每层只解决一类问题。
- 允许上层引用下层，不允许同层互相复制正文。
- 允许 instructions 链接 standards，不允许大段改写后再复制一遍。

## 3. 目标分层

### Layer 0: 共享前端反馈标准

文档：

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`

职责：

- 定义跨 Web 与小程序通用的提示、错误反馈、成功反馈、首屏失败承接原则。

允许包含：

- 一事一次提示
- 原始后端错误不可直出
- 首屏失败必须页内承接
- 提示通道选择原则

禁止包含：

- 小程序特有页面骨架
- TDesign 组件选型
- 小程序 API 契约细节

### Layer 1: 小程序基础设计系统

目标文档：

- `.github/standards/weapp/README.md`
- `.github/standards/weapp/DESIGN_SYSTEM.md`

职责：

- 只保留设计基础、token、组件选型原则、页面骨架、通用布局与可视层级规则。

允许包含：

- 颜色、字号、间距、圆角、安全区 token
- 页面壳结构
- 按钮、标签、卡片、弹层等基础模式
- TDesign 优先原则
- 视觉层级、触控热区、信息密度、对比度、输入可读性等基础 UX 规则

禁止包含：

- 服务层到页面层的贯通清单
- 提交前验证门槛
- PR 审查要点
- 异步作业、支付轮询、分页真值等 API 消费契约

### Layer 2: 小程序交互与任务流标准

目标文档：

- `.github/standards/weapp/INTERACTION_STANDARDS.md`

职责：

- 定义页面级交互体验、状态设计、任务流承接、弱网与恢复策略。

允许包含：

- 首屏、局部刷新、静默回读、提交中等状态设计
- 列表页、搜索页、详情页、表单页、支付页的体验基线
- 回退恢复、重入恢复、切前后台恢复
- 防重复点击、危险操作确认、结果承接、空态与错误态可行动性

禁止包含：

- token 细节表格
- 组件 API 级规范
- 路由 prompt 使用说明

### Layer 3: 小程序 API 交互契约

目标文档：

- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`

职责：

- 定义小程序消费后端接口时必须遵守的交互与状态契约。

允许包含：

- 分页真值来源
- 鉴权失效恢复
- 乐观更新与回读校验边界
- 上传、OCR、支付、轮询、异步确认的前端承接语义
- 错误映射责任和前端展示边界
- 重试、去重、幂等、防抖与防重入规则

禁止包含：

- API 重构过程记录
- 已完成项清单
- 下一步计划
- 兼容别名流水账

### Layer 4: 执行型 instructions

文档：

- `.github/instructions/weapp-mini-program.instructions.md`
- `.github/instructions/weapp-pages.instructions.md`
- `.github/instructions/weapp-components.instructions.md`

职责：

- 根据目录自动注入最小必要执行规则。

允许包含：

- 当前层职责
- 必读标准链接
- 禁止事项
- 验证默认命令

禁止包含：

- 大段设计系统正文
- 大段提示系统正文
- 可被 standards 单独维护的完整规则列表

### Layer 5: prompts

文档：

- `.github/prompts/weapp-implementation.prompt.md`
- `.github/prompts/weapp-review.prompt.md`
- `.github/prompts/weapp-page-refactor-blueprint.prompt.md`
- `.github/prompts/weapp-payment-flow.prompt.md`

职责：

- 组织 AI 任务输入、输出和验收项。

允许包含：

- 任务背景输入项
- 产出结构
- 验收清单
- 明确的高风险场景补充要求

禁止包含：

- 长期标准正文
- 规范治理说明
- 和 instructions 重复的默认规则列表

### Layer 6: 历史与迁移文档

建议位置：

- `weapp/docs/`
- `.github/standards/weapp/historical/`

职责：

- 保留历史改造计划、已完成改造说明、阶段性审计记录。

允许包含：

- 改造纪要
- 评分计划
- 阶段性 rollout 结论

禁止进入：

- instructions 的 Read First 热路径
- standards 的权威目录首页

## 4. 目标目录结构

建议最终目录如下：

```text
.github/
  standards/
    frontend/
      USER_FEEDBACK_STANDARDS.md
    weapp/
      README.md
      DESIGN_SYSTEM.md
      INTERACTION_STANDARDS.md
      API_INTERACTION_CONTRACT.md
      historical/
        API_REFACTOR_2026_Q1.md
  instructions/
    weapp-mini-program.instructions.md
    weapp-pages.instructions.md
    weapp-components.instructions.md
  prompts/
    weapp-implementation.prompt.md
    weapp-review.prompt.md
    weapp-page-refactor-blueprint.prompt.md
    weapp-payment-flow.prompt.md

weapp/
  docs/
    miniprogram-prompt-system.md
    WEAPP_DOCUMENTATION_ARCHITECTURE_2026-04-04.md
    WEAPP_DOCUMENT_REWRITE_BLUEPRINTS_2026-04-04.md
```

## 5. 现有文档到目标层级的映射

| 当前文档 | 当前问题 | 目标角色 | 目标动作 |
| --- | --- | --- | --- |
| `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md` | 角色清晰 | Layer 0 | 保持不动 |
| `weapp/docs/miniprogram-prompt-system.md` | 内容有效，但和 instructions 有重复 | Runtime-specific prompt standard | 保留，删掉被上移到 Layer 0 的重复部分 |
| `.github/standards/weapp/DESIGN_SYSTEM.md` | 职责过重 | Layer 1 | 瘦身，仅保留 foundation 和基础模式 |
| `.github/standards/weapp/api/README.md` | 是迁移纪要，不是标准 | Layer 6 historical | 迁出热路径，并改写为历史文档 |
| `.github/instructions/weapp-mini-program.instructions.md` | 重复标准正文 | Layer 4 | 收缩成入口执行清单 |
| `.github/instructions/weapp-pages.instructions.md` | 缺页面状态机与恢复细则 | Layer 4 | 补页面交互职责，删除重复总规则 |
| `.github/instructions/weapp-components.instructions.md` | 偏薄 | Layer 4 | 增加组件契约规则 |
| `weapp-*.prompt.md` | 工程输入多，体验输入少 | Layer 5 | 增加用户任务、状态与恢复输入项 |

## 6. 每层的唯一权威定义

- 提示策略、错误反馈、success Toast 取舍：Layer 0
- token、组件、布局骨架、安全区：Layer 1
- 列表、表单、搜索、支付、弱网、重入恢复：Layer 2
- 分页真值、异步作业、鉴权失效、轮询承接：Layer 3
- 当前目录下必须执行什么：Layer 4
- AI 任务如何提问和验收：Layer 5
- 为什么当年这么改：Layer 6

## 7. 建议的迁移顺序

### Phase 1: 建立新权威入口

- 新增 `.github/standards/weapp/README.md`
- 新增 `.github/standards/weapp/INTERACTION_STANDARDS.md`
- 新增 `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- 将 `.github/instructions/weapp-mini-program.instructions.md` 的 Read First 改为指向新入口

### Phase 2: 瘦身旧 DESIGN_SYSTEM

- 保留 token、基础组件、页面骨架、弹层与安全区
- 移出交互架构、服务贯通检查、组件边界、质量门槛
- 把设计系统恢复成真正的 foundation 文档

### Phase 3: 重写 instructions

- 总 instruction 只保留入口级禁令和默认验证
- 页面 instruction 聚焦页面职责、状态机和恢复策略
- 组件 instruction 聚焦 API 边界、事件契约和复用规则

### Phase 4: 重写 prompts

- 为实现 prompt 增加用户角色、任务频率、成功定义、弱网敏感度输入
- 为 review prompt 增加交互审查维度
- 为 refactor blueprint prompt 增加固定评分模型
- 为 payment prompt 增加跨页一致性和未知结果承接要求

### Phase 5: 历史文档归档

- 将 `.github/standards/weapp/api/README.md` 迁入 historical
- 保留引用入口，但不再出现在默认 Read First 中

## 8. 新体系的验收标准

满足以下条件时，可认为小程序文档体系完成了结构升级：

- 任意一条规则都能回答“唯一权威在哪”。
- instructions 总长度明显下降，但执行信息密度上升。
- prompts 不再依赖复制标准正文，而是依赖更高质量的任务输入。
- `DESIGN_SYSTEM.md` 不再混入流程审查与代码交付检查。
- API 交互契约文档不包含阶段性改造流水账。
- 历史说明文档退出 Read First 热路径。

## 9. 不在本轮范围内的事项

- 不在本轮直接修改所有现有页面代码。
- 不在本轮重新设计全部组件样式。
- 不在本轮引入新的 agent 或 prompt 类型。
- 不在本轮处理 Web 或 Backend 文档体系。

## 10. 推荐落地方式

建议以“先增量、再迁移、后删除”的方式落地：

1. 先创建新权威文档。
2. 再让 instructions 改为引用新权威文档。
3. 再逐步瘦身旧文档。
4. 最后把旧的迁移型文档归档。

这样可以避免在重构标准期间让 AI 路由和团队协作同时失效。