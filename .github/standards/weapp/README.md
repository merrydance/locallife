# Mini Program Standards Index

本目录是 LocalLife 小程序相关长期标准的权威入口。

## 目标

本目录中的文档用于回答三个问题：

- 这条规则属于设计基础、交互体验、还是 API 消费契约。
- 这条规则的唯一权威文档是什么。
- 这条规则是否属于长期标准，而不是阶段性改造记录。

## 权威层级

### Layer 0: 共享前端反馈标准

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`

适用范围：

- 所有用户可见的提示、错误反馈、成功反馈、首屏失败承接。

### Layer 1: 小程序基础设计系统

- `.github/standards/weapp/DESIGN_SYSTEM.md`

适用范围：

- token、页面骨架、基础组件模式、布局、安全区、视觉层级、基础可读性约束。

### Layer 2: 小程序交互与任务流标准

- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`

适用范围：

- 页面状态、任务流、弱网恢复、回退恢复、页面重入、主次操作、空态与错误态可行动性。
- 首屏请求预算、预加载策略、角色隔离、请求扇出控制、弱网下的预热降级。

### Layer 3: 小程序 API 交互契约

- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`

适用范围：

- 分页真值、鉴权失效恢复、异步作业承接、支付轮询与未知结果、重试、防重入、乐观更新边界。

## 当前仍保留的运行时补充文档

- `weapp/docs/miniprogram-prompt-system.md`

作用：

- 提供小程序运行时提示系统、错误映射、Toast 去重与页面接入细节。

说明：

- 这份文档补充 Layer 0，但不替代 Layer 0。
- 如果与共享前端反馈标准冲突，应以共享标准为准。

## 历史材料说明

以下文档仍可作为历史参考，但不再应被视为当前长期标准的首选入口：

- `.github/standards/weapp/api/README.md`

该文件当前主要记录 API 重构过程、兼容性与阶段性改造成果。后续如需查阅历史背景，可以继续参考；但新增规则不应继续落在该文档中。

## 推荐阅读顺序

当任务涉及 `weapp/` 时，建议按以下顺序读取：

1. `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
2. `.github/standards/weapp/README.md`
3. `.github/standards/weapp/DESIGN_SYSTEM.md`
4. `.github/standards/weapp/INTERACTION_STANDARDS.md`
5. `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`
6. `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
7. `weapp/docs/miniprogram-prompt-system.md`

## 维护规则

- 新增小程序长期规则时，优先放入本目录下已有权威文档，而不是在 instructions 或 prompts 中重复复制。
- instructions 只应保留执行约束和 Read First 入口，不应重复本目录中的完整正文。
- prompts 只应组织任务输入和验收方式，不应承载长期标准正文。
- 阶段性改造说明、评分计划、任务卡与迁移纪要应放在 `weapp/docs/` 或 historical 目录，不应出现在默认热路径中。