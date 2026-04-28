# Mini Program Standards Index

本目录是 LocalLife 小程序相关长期标准的权威入口。

## 目标

本目录中的文档用于回答三个问题：

- 小程序页面和组件交付默认到底该以哪份标准为准。
- 视觉与非视觉规则分别由哪份标准承担。
- 哪些文档只是专题补充、历史背景或运行时实现参考，而不是默认权威入口。

## 默认权威入口

- 页面与组件交付的默认主标准：`.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
- 顾客侧视觉、token、页面壳、触控热区和组件视觉模式：`.github/standards/weapp/DESIGN_SYSTEM.md`
- 非顾客侧视觉、page shell、克制型 TDesign-first 表达与组件视觉模式：`.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`
- 审查时可直接复用的检查清单：`.github/standards/weapp/REVIEW_CHECKLIST.md`

默认执行规则：

- 当前长期生效的 weapp 活文档只有 4 份：`PAGE_DELIVERY_BASELINE.md`、`DESIGN_SYSTEM.md`、`NON_CONSUMER_DESIGN_SYSTEM.md`、`REVIEW_CHECKLIST.md`。
- 不再把交互、性能、反馈、API 契约拆成多个默认必读标准。
- `PAGE_DELIVERY_BASELINE.md` 是小程序页面与组件交付的单一默认权威来源。
- `DESIGN_SYSTEM.md` 负责顾客侧视觉与组件视觉基线；`NON_CONSUMER_DESIGN_SYSTEM.md` 负责非顾客侧视觉与组件视觉基线；两者都不得削弱 `PAGE_DELIVERY_BASELINE.md` 的硬性要求。
- 若同一问题在多份 weapp 标准中出现冲突，以 `PAGE_DELIVERY_BASELINE.md` 为准；视觉 token 和组件视觉细节按页面角色侧选择对应设计文档。

## 默认实现热路径

- 页面与组件的非视觉交付，默认先读 `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`。
- 顾客侧页面 UI 结构、小屏手感、品牌化视觉与组件视觉基线，读 `.github/standards/weapp/DESIGN_SYSTEM.md`。
- 商户、运营、平台、骑手等非顾客侧页面的 UI 结构、小屏手感、page shell 与克制型 TDesign-first 表达，读 `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`。
- 非顾客侧页面若处于实现收口、样式统一或快速自检阶段，再补读 `.github/standards/weapp/NON_CONSUMER_PAGE_EXECUTION_CHECKLIST.md` 作为执行压缩清单；它是辅助收口文档，不是新的并行权威层。
- 页面审查与复审时，配合 `.github/standards/weapp/REVIEW_CHECKLIST.md` 使用。
- 触达已知超级 service 热点时，同时参考 `weapp/docs/architecture-ownership/` 下的 ownership notes；这些 note 不是新的标准层，但它们是 gate 校验的一部分，用于阻止超级 service 无说明扩张。

## TDesign 来源口径

- 小程序用户可见组件默认先查 TDesign MCP 组件列表与组件文档，再决定是否需要本地补充。
- TDesign 组件存在且官方支持的组合方式已经能承接任务时，不应再新增本地 notice/card/panel/footer 一类用户可见样式壳。
- 若 MCP 文档与仓库实际依赖版本不一致，以 `weapp/package.json` 中安装的版本能力为准。
- 当前非顾客侧标准中维护了一份按仓库实际依赖版本核对过的 TDesign Miniprogram 组件清单，见 `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md` 中的 TDesign 组件清单小节；prompt 与 instructions 应引用这一路径，不再各自维护会漂移的另一套口径。

## 专题补充与历史参考

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md`
- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`

作用：

- 保留为分主题背景材料、历史拆分规则来源或专项深挖参考。
- 不再作为默认实现、默认 review 或 app-wide instruction 的并行权威入口。
- 不在这些文档中继续新增新的长期硬规则；若规则需要长期生效，应回收到 4 份活文档之一。
- 若这些文档与 `PAGE_DELIVERY_BASELINE.md` 的执行口径冲突，以 `PAGE_DELIVERY_BASELINE.md` 为准。

## 审查辅助清单

- `.github/standards/weapp/REVIEW_CHECKLIST.md`

作用：

- 提供可直接贴进 PR review 的压缩版审查清单。
- 它不是新的权威层；清单项只负责压缩检查，不负责再定义一套并行标准。
- 做整体升级型 review 时，可配合历史蓝图文档一起使用。

## 当前运行时实现参考

- `weapp/miniprogram/utils/user-facing.ts`
- `weapp/miniprogram/utils/prompt-feedback.ts`

作用：

- 提供小程序当前运行时的错误映射、用户文案提取、Toast/Modal 去重与全局提示守卫实现。

说明：

- 运行时实现用于回答“现在代码里具体怎么接”，不替代 `PAGE_DELIVERY_BASELINE.md`、`DESIGN_SYSTEM.md` 和 `NON_CONSUMER_DESIGN_SYSTEM.md` 的长期标准地位。

## 历史材料说明

以下文档仍可作为历史参考，但不再应被视为当前长期标准的首选入口：

- `.github/standards/weapp/api/README.md`
- `weapp/docs/MERCHANT_BACKEND_WEAPP_MAPPING_MATRIX_2026-04-06.md`
- `weapp/docs/RIDER_BACKEND_WEAPP_MAPPING_MATRIX_2026-04-07.md`

这些文档主要记录 API 结构说明、角色侧映射关系与阶段性治理背景。后续如需查阅历史背景、治理脉络或角色到后端契约的映射，可以继续参考；但新增长期规则不应继续落在这些文档中。

## 整体升级审计的使用方式

- 常规实现和常规 review 先按 `PAGE_DELIVERY_BASELINE.md` 与角色侧对应的设计文档判断是否合规。
- 如果任务目标是“整体升级”“统一体验”“把页面做得更友好更一致”，再叠加历史蓝图材料做升级审计。
- 历史蓝图只用于补充升级方向和旧问题模式，不再作为默认执行标准。

## 推荐阅读顺序

当任务涉及 `weapp/` 时，建议按以下顺序读取：

1. `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
2. 顾客侧任务读 `.github/standards/weapp/DESIGN_SYSTEM.md`；非顾客侧任务读 `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`
3. `.github/standards/weapp/REVIEW_CHECKLIST.md`（当任务是 review 或复审时读取）
4. `.github/standards/weapp/README.md`
5. `weapp/miniprogram/utils/user-facing.ts`（仅在需要核对错误对象与用户文案映射实现时读取）
6. `weapp/miniprogram/utils/prompt-feedback.ts`（仅在需要核对 Toast/Modal 去重守卫时读取）

## 文档归属规则

后续维护时，默认按下面的归属关系落内容：

- 活文档：可以新增长期硬规则。
- 专题参考文档：只能补充背景、案例、拆分视角、常见误区和诊断提示，不能再新增默认 gate。
- instructions：只能保留 Read First、始终生效的执行约束和路径级反模式，不重复标准正文。
- prompts：只组织任务输入、输出结构和验收方式，不承载长期规范。
- 运行时实现参考：只回答“代码里现在怎么接”，不定义产品或交付规则。
- ownership notes：只记录受保护超级 service 的 owner、临时边界和 gate 约束，不升级成新的并行权威标准。

建议按下表判断新内容该放哪里：

| 内容类型 | 应放位置 |
| :--- | :--- |
| 页面交付的默认硬规则 | `PAGE_DELIVERY_BASELINE.md` |
| 顾客侧视觉、token、组件视觉表达 | `DESIGN_SYSTEM.md` |
| 非顾客侧视觉、page shell、TDesign-first 表达 | `NON_CONSUMER_DESIGN_SYSTEM.md` |
| 压缩版审查问题 | `REVIEW_CHECKLIST.md` |
| 交互、性能、API 主题展开 | 对应专题参考文档 |
| 风险分级、验证证据、剩余风险、发布口径 | 工程治理标准 |
| 错误对象、提示守卫、运行时接入细节 | `weapp/miniprogram/utils/*` |

## 维护规则

- 新增小程序长期规则时，优先放入本目录下已有权威文档，而不是在 instructions 或 prompts 中重复复制。
- 不要再把长期硬规则写入专题参考文档；专题文档只保留背景、例子、展开说明或历史上下文。
- instructions 只应保留执行约束和 Read First 入口，不应重复本目录中的完整正文。
- prompts 只应组织任务输入和验收方式，不应承载长期标准正文。
- 运行时实现参考只保留守卫、错误对象、接入工具、例外项和当前实现细节，不再重复页面交互或 API contract 的主规范。
- 阶段性改造说明、评分计划、任务卡与迁移纪要应放在 `weapp/docs/` 或 historical 目录，不应出现在默认热路径中。