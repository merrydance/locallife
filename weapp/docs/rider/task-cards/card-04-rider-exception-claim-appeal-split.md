# CARD-04 骑手索赔、申诉、追偿域收口

状态：已实现，待人工回归

优先级：P0

所属阶段：Phase 2

## 问题目标

把 claims、appeal、recovery 从旧混杂状态收口成“列表页加详情页”两层职责，形成符合后端真实语义的骑手处理面。

## 影响范围

- weapp/miniprogram/pages/rider/claims/index.ts
- weapp/miniprogram/pages/rider/claims/index.wxml
- weapp/miniprogram/pages/rider/claims/detail/index.ts
- weapp/miniprogram/pages/rider/claims/detail/index.wxml
- weapp/miniprogram/api/appeals-customer-service.ts
- locallife/api/appeal.go
- locallife/api/claim_recovery.go

## 任务内容

- [x] 下线 exception/index 与 rider-exception-handling.ts，移除已判定为伪需求的异常报备与延时报备前端承接。
- [x] claims/index 收口为真实索赔列表页，使用 bucket 过滤，不再混入申诉表单或追偿动作。
- [x] 新增 claims/detail/index，承接 claim detail、decision、recovery、appeal 全链动作与结果回看。
- [x] 修复 createRiderAppeal 的入参来源，claim_id 必须来自真实 claim，而不是 taskId 或 orderId。
- [x] 收口追偿支付流程，如果接口返回 pay_params，则必须接 wx.requestPayment，而不是只 await 成功。
- [x] 删除无真实后端链路的异常图片上传、异常历史和延时报备 UI。

## 完成定义

- [x] 索赔、申诉、追偿三类动作各自有明确页面职责。
- [x] 用户不会再从 rider 域进入伪异常报备页面误触 claim / appeal 语义。
- [x] 追偿支付形成真实闭环。

## 验证要求

- [ ] 人工验证 claim、appeal、recovery 支付链路。
- [ ] 人工验证 claims 列表、详情、支付回流与申诉回流。
- [x] 执行最小相关质量检查。

## 完成记录

- [x] 代码完成
- [x] 领域拆分完成
- [ ] 回归完成

## 本次实现说明

- exception/index 与相关旧 service 已整体下线，不再作为 rider 域保留页面。
- claims/index 已改为真实索赔列表页，区分 bucket 与我的申诉，不再承担详情内动作。
- claims/detail/index 已承接责任判定、行为摘要、追偿支付、申诉提交与处理结果回看。
- 追偿支付已接入微信支付调用，并补上支付后的状态回读。
- 已完成 `npm run quality:check`，真机与弱网人工回归仍待执行。