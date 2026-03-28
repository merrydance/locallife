# CARD-04 骑手异常、索赔、申诉域拆分

状态：已实现，待人工回归

优先级：P0

所属阶段：Phase 2

## 问题目标

把 exception、claim、appeal、recovery 从当前混杂状态拆开，形成符合后端真实语义的页面职责。

## 影响范围

- weapp/miniprogram/pages/rider/exception/index.ts
- weapp/miniprogram/pages/rider/exception/index.wxml
- weapp/miniprogram/pages/rider/claims/index.ts
- weapp/miniprogram/pages/rider/claims/index.wxml
- weapp/miniprogram/api/rider.ts
- weapp/miniprogram/api/rider-exception-handling.ts
- weapp/miniprogram/api/appeals-customer-service.ts
- locallife/api/rider.go
- locallife/api/appeal.go
- locallife/api/claim_recovery.go

## 任务内容

- [x] 将 exception/index 明确为异常上报页，只消费 /v1/rider/orders/:id/exception 与真实异常历史策略。
- [x] 决定异常历史如果后端暂无独立接口，是补接口还是改为只展示最近上报结果，不得继续拿 appeals 顶替。
- [x] claims/index 明确改造为“索赔与申诉页”或拆成 claim detail / appeal list，不再使用“异常上报”文案。
- [x] 修复 createRiderAppeal 的入参来源，claim_id 必须来自真实 claim，而不是 taskId 或 orderId。
- [x] 收口追偿支付流程，如果接口返回 pay_params，则必须接 wx.requestPayment，而不是只 await 成功。
- [x] 决定图片上传能力是否保留；若保留则补真实上传链路，若不保留则移除 UI。

## 完成定义

- [x] 异常、索赔、申诉、追偿四类动作各自有明确页面职责。
- [x] 用户不会在“异常上报”页面里误触 claim / appeal 语义。
- [x] 追偿支付形成真实闭环。

## 验证要求

- [ ] 人工验证异常上报成功与失败场景。
- [ ] 人工验证 claim、appeal、recovery 支付链路。
- [x] 执行最小相关质量检查。

## 完成记录

- [x] 代码完成
- [x] 领域拆分完成
- [ ] 回归完成

## 本次实现说明

- exception/index 已收口为纯异常上报页，移除了用 appeals 顶替异常历史的错误行为，改为展示最近一次提交结果。
- claims/index 已改为真实“索赔与申诉”页，区分索赔记录与我的申诉，并修复 claim_id 来源。
- 追偿支付已接入微信支付调用，异常上报页的图片上传 UI 已移除，避免继续保留无后端链路的假能力。
- 已完成 `npm run quality:check`，真机与弱网人工回归仍待执行。