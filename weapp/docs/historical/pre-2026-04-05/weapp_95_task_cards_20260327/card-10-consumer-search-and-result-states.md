# CARD-10 消费侧搜索与结果状态完备性

状态：进行中（代码完成，待人工回归）

优先级：P1

所属阶段：Phase 2

## 问题目标

让搜索页明确区分 loading、empty、error、retry，而不是只靠 toast 表达失败。

## 影响范围

- [weapp/miniprogram/pages/takeout/search/index.ts](weapp/miniprogram/pages/takeout/search/index.ts)

## 任务内容

- [x] 增加显式 error state 与 retry 入口。
- [x] 区分“没有结果”和“请求失败”。
- [x] 统一建议态、结果态与空态之间的切换规则。

## 完成定义

- [ ] 搜索失败时页面内可恢复。
- [ ] 用户可明确感知空结果与失败结果的区别。

## 验证要求

- [ ] 人工验证弱网失败、空结果、有结果三类场景。

## 完成记录

- [x] 页面改造完成
- [ ] 三态回归完成
- [ ] 评审完成

补充说明：

- 搜索页已新增初始数据 `loading/error/retry`，并把结果页拆成 `searching / resultsError / empty / success` 四种明确状态，不再只依赖 toast 表达失败。
- 当搜索建议请求失败时，页面会降级为“无建议”而不是保留旧建议；当搜索结果失败时，页内会保留重试按钮和错误文案。
- 当前已通过编辑器诊断和定向 ESLint 校验，仍需在开发者工具或真机中补做弱网失败、空结果、有结果三类人工回归。