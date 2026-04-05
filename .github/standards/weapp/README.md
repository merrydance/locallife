# Mini Program Standards Index

本目录是 LocalLife 小程序相关长期标准的权威入口。

## 目标

本目录中的文档用于回答三个问题：

- 这条规则属于设计基础、交互体验、还是 API 消费契约。
- 这条规则的唯一权威文档是什么。
- 这条规则是否属于长期标准，而不是阶段性改造记录。

## 默认审查视角

任何小程序页面的新建、重构或 review，默认都要从以下三个部分审查：

- 视觉：页面布局结构、组件配合使用、边距与间距、视觉层级、可读性、热区、安全区、空态与结果区的观感是否统一。
- 交互：主任务是否清楚、状态是否完整、反馈是否单一且清晰、弱网与重入恢复是否稳、返回与上下文保留是否合理。
- 性能：首屏请求预算、预加载边界、`onLoad` / `onShow` 重拉量、长列表渲染负担、弱网下的退化策略是否受控。

补充原则：

- 后端真值、分页、鉴权恢复、异步结果 contract 仍以 API 交互契约为底线，不因视觉、交互、性能三分法而被弱化。
- 做实现时，这三部分都要被显式考虑；做 review 时，这三部分都要被显式检查。

## 权威层级

### Layer 0: 共享前端反馈标准

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`

适用范围：

- 所有用户可见的提示、错误反馈、成功反馈、首屏失败承接。

### Layer 1: 小程序基础设计系统

- `.github/standards/weapp/DESIGN_SYSTEM.md`

适用范围：

- token、页面骨架、基础组件模式、布局、安全区、视觉层级、基础可读性约束。

对应默认审查视角：

- 视觉

### Layer 2: 小程序交互与任务流标准

- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`

适用范围：

- 页面状态、任务流、弱网恢复、回退恢复、页面重入、主次操作、空态与错误态可行动性。
- 首屏请求预算、预加载策略、角色隔离、请求扇出控制、弱网下的预热降级。

对应默认审查视角：

- 交互
- 性能

### Layer 3: 小程序 API 交互契约

- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`

适用范围：

- 分页真值、鉴权失效恢复、异步作业承接、支付轮询与未知结果、重试、防重入、乐观更新边界。

对应角色：

- 这是视觉、交互、性能三部分都必须共同遵守的底线 contract，不是第四种平行风格意见。

## 审查辅助清单

- `.github/standards/weapp/REVIEW_CHECKLIST.md`

作用：

- 提供可直接贴进 PR review 的压缩版审查清单。
- 它不是新的权威层，只是把 Layer 0 到 Layer 3 与运行时提示补充压成一个 review 热路径入口。
- 做整体升级型 review 时，可配合历史蓝图文档一起使用。

## 当前仍保留的运行时补充文档

- `weapp/docs/miniprogram-prompt-system.md`

作用：

- 提供小程序运行时提示系统、错误映射、Toast 去重与页面接入细节。

说明：

- 这份文档补充 Layer 0，但不替代 Layer 0。
- 如果与共享前端反馈标准冲突，应以共享标准为准。

## 规则裁定顺序

当同一个话题同时出现在多份 weapp 文档中时，按下面的维度裁定主文档：

- 视觉基础、token、按钮/标签/卡片/弹层结构、安全区、热区与骨架：以 `.github/standards/weapp/DESIGN_SYSTEM.md` 为准。
- 页面状态、任务流、弱网恢复、回退恢复、重入恢复、主次操作与结果承接：以 `.github/standards/weapp/INTERACTION_STANDARDS.md` 为准。
- 后端字段真值、状态枚举、分页真值、鉴权恢复、异步结果 contract、请求幂等与重试语义：以 `.github/standards/weapp/API_INTERACTION_CONTRACT.md` 为准。
- 请求时机、首屏预算、预加载边界、`onLoad` / `onShow` 重拉量控制：以 `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md` 为准。
- 提示守卫、Toast 去重、错误对象字段、页面接入工具、保留 success Toast 例外：以 `weapp/docs/miniprogram-prompt-system.md` 为准。

如果同一条规则在 standards、instructions、prompts 和运行时补充文档里都能搜到，应优先修改这里指定的主文档，再同步镜像层，而不是直接在镜像层各改一份。

## 历史材料说明

以下文档仍可作为历史参考，但不再应被视为当前长期标准的首选入口：

- `.github/standards/weapp/api/README.md`
- `weapp/docs/WEAPP_DOCUMENTATION_ARCHITECTURE_2026-04-04.md`
- `weapp/docs/WEAPP_DOCUMENT_REWRITE_BLUEPRINTS_2026-04-04.md`
- `weapp/docs/WEAPP_95_SCORE_IMPROVEMENT_PLAN_2026-03-27.md`

这些文档主要记录 API 重构过程、文档分层蓝图、阶段性质量判断与系统升级目标。后续如需查阅历史背景、治理脉络或整体升级方向，可以继续参考；但新增长期规则不应继续落在这些文档中。

## 历史蓝图在 Review 中的使用方式

- 常规实现 review 先按 Layer 0 到 Layer 3 的现行标准判断是否合规。
- 如果任务目标是“整体升级”“统一体验”“把页面做得更友好更一致”，应额外把上述历史蓝图文档作为升级审计输入。
- 历史蓝图可用于判断当前实现是否仍停留在旧问题模式，例如伪完成、弱网恢复缺失、首屏扇出过大、页面职责混乱、说明文案堆叠、跨页体验不一致。
- 如果历史蓝图和现行标准在具体规则上出现冲突，以现行标准为准；历史蓝图只负责补充“要升级到什么水平”和“过去反复出现过哪些系统性问题”。
- 做整体升级型 review 时，结论应同时回答两件事：当前实现有没有违反现行标准，以及它是否仍延续了历史蓝图已点名的低质量模式。

## 推荐阅读顺序

当任务涉及 `weapp/` 时，建议按以下顺序读取：

1. `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
2. `.github/standards/weapp/README.md`
3. `.github/standards/weapp/DESIGN_SYSTEM.md`
4. `.github/standards/weapp/INTERACTION_STANDARDS.md`
5. `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`
6. `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
7. `weapp/docs/miniprogram-prompt-system.md`
8. `.github/standards/weapp/REVIEW_CHECKLIST.md`

## 维护规则

- 新增小程序长期规则时，优先放入本目录下已有权威文档，而不是在 instructions 或 prompts 中重复复制。
- instructions 只应保留执行约束和 Read First 入口，不应重复本目录中的完整正文。
- prompts 只应组织任务输入和验收方式，不应承载长期标准正文。
- 运行时补充文档只保留守卫、错误对象、接入工具、例外项和当前实现细节，不再重复页面交互或 API contract 的主规范。
- 阶段性改造说明、评分计划、任务卡与迁移纪要应放在 `weapp/docs/` 或 historical 目录，不应出现在默认热路径中。