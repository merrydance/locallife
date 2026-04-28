# Frontend Architecture Baseline

本文件定义 LocalLife 前端共同遵循的架构底线，适用于 Web、小程序和 Flutter 商户端。

它解决一个反复出现的问题：前端把后端 API、DTO 或接口数量直接镜像成页面、卡片、组件和提示文案，导致页面看起来像字段面板或接口调试台，而不是帮助用户完成任务的产品界面。

## 1. Core Principle

- 前端设计的第一驱动力是人的行为、目标和当前任务，不是 API 形状。
- 后端 API 是事实、能力和约束的来源，但不是页面结构、组件边界或交互顺序的来源。
- UI 是用户意图的表达，不是后端数据结构的截图。
- API 返回值进入 UI 前，必须经过明确的 adapter、view model、provider、controller 或 use case 转换。

## 2. Required Design Order

复杂页面、管理面、商户端工作流、资金/审核/支付/订单等状态流转页面，默认按以下顺序推进：

1. 识别目标用户、当前任务、成功条件和失败后果。
2. 盘点真实后端能力：实体、字段、状态、动作、权限、分页、异步结果和错误语义。
3. 把共同服务同一连续目标的能力组合成任务域，而不是按接口数量拆页面。
4. 判断任务域 owner：页面、页面组、领域组件、provider、controller、workflow 或 use case。
5. 设计 ViewState：loading、empty、error、stale data、disabled、submitting、success、unknown result、retry 和 re-entry 状态。
6. 决定页面边界、组件边界和首屏信息层级。
7. 最后选择具体 UI 组件、样式和反馈通道。

如果实现直接从接口文件、旧 DOM/WXML/Widget 树或 API 字段列表开始设计页面，应视为架构风险，而不是普通样式选择。

## 3. Layer Responsibilities

### 3.1 Data Source Or Repository

- 负责 API 调用、缓存、请求参数、响应解析和后端字段适配。
- 屏蔽 REST、GraphQL、接口路径、原始字段名和后端错误细节。
- 不决定页面布局，也不向 UI 暴露原始后端错误、英文诊断或内部字段。

### 3.2 Domain Or Use Case

- 负责业务意图和任务编排，例如接单、提交审核、支付、回查状态、保存草稿、处理重复点击和恢复未知结果。
- 可以组合多个 repository 能力，但必须说明状态真值、失败边界和副作用边界。
- 不依赖具体 UI 框架 API，例如非 UI 层不持有 `BuildContext`，不直接操作页面实例，也不把路由跳转藏进展示组件。

### 3.3 Presentation State

- 负责把领域结果转换成 ViewState。
- 持有筛选、分页、提交中、禁用、空态、错误态、重试态、回读态和用户可见状态映射。
- 是 UI 的稳定输入边界；UI 不应自行解释后端枚举、权限或错误语义。

### 3.4 UI Components

- 负责渲染 ViewState，并把用户动作转发成 action。
- 可以拥有纯展示型局部状态，但不拥有业务流、请求副作用、跨页恢复或后端状态解释。
- 展示组件接收领域对象、view model 或 props，不直接消费原始 API JSON。

## 4. Anti-Patterns

- API 平铺：一个接口对应一个页面、一个接口返回块对应一张卡片、一个字段对应一行展示，而没有解释它服务的用户任务。
- 实体镜像：页面按后端 entity 字段完整铺开，却没有主任务、主操作和首屏优先级。
- 组件堆叠：为了覆盖所有 API 能力，把低频能力、未来能力和无关操作堆进同一页。
- 解释性大卡片补洞：用“这里用于...”一类顶部说明卡解释页面边界，而不是用任务结构、状态、标签、动作层级和字段约束让页面自明。
- UI 吸收逻辑：Widget、WXML、React component 或 presentational component 直接拼接口、解释状态、处理重试、控制副作用或承担 workflow。
- 本地伪真值：前端用本地状态、默认值、过滤结果或乐观文案假装后端已经确认业务结果。
- 反馈堆叠：Toast、Modal、说明卡、状态块和跳转结果页重复描述同一个动作结果。

## 5. Review Questions

每次前端实现或 review 至少检查：

1. 页面是从用户任务推导出来的，还是从 API/DTO 形状平铺出来的。
2. 多个接口是否被组合成一个清晰任务域，或被合理拆成多个独立任务域。
3. ViewState 是否完整承接 loading、empty、error、submitting、unknown result、retry 和 re-entry。
4. UI 是否只消费领域对象或 view model，而不是直接消费原始 API JSON。
5. 解释性文案是否在弥补信息架构失败；如果删掉顶部说明卡，页面是否仍能被理解。
6. 状态真值、权限真值、金额真值和最终结果真值是否来自后端契约。
7. 失败、弱网、重复点击、返回重入和刷新后状态是否仍由同一个 owner 收住。

## 6. Authority Relationship

- 本文件是跨 Web、小程序和 Flutter 商户端的前端架构基线。
- 端侧标准可以补充组件、视觉、运行时和验证细节，但不得削弱本文件的任务驱动、领域分层和 ViewState 边界。
- 小程序页面交付仍以 `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` 为端侧默认入口；Web UI 仍以 `.github/standards/web/WEB_UI_STANDARDS.md` 为端侧默认入口；Flutter 商户端仍以 `.github/standards/flutter/README.md` 为端侧默认入口。