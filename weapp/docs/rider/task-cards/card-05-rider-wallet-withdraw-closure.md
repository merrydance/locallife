# CARD-05 骑手钱包、押金与提现闭环

状态：已实现，待人工回归

优先级：P1

所属阶段：Phase 2

## 问题目标

把 deposit/index 从“充值与账单页”提升到真实的押金账户页，补齐提现、失败回查和状态完备性。

## 影响范围

- weapp/miniprogram/pages/rider/deposit/index.ts
- weapp/miniprogram/pages/rider/deposit/index.wxml
- weapp/miniprogram/api/rider.ts
- locallife/api/rider.go

## 任务内容

- [x] 明确押金页职责，决定是否接入 /v1/rider/withdraw。
- [x] 为充值补支付取消、失败、回查与刷新策略，不只依赖 success callback。
- [x] 为余额、账单列表补页内 error/retry 和空态区分。
- [x] 若接提现，则补处理中、成功、失败、冻结中的展示与保护。

## 完成定义

- [x] 押金页具备完整的充值和账单状态表达。
- [x] 提现能力若后端已开放，则前端有真实入口；若暂不开放，则文档中明确说明并移除误导性预期。

## 验证要求

- [ ] 人工验证充值成功、取消、失败场景。
- [ ] 人工验证账单翻页与刷新。
- [ ] 如接提现，验证最小提现、余额不足、处理中场景。

## 完成记录

- [x] 代码完成
- [ ] 资金链路验证完成
- [ ] 回归完成

## 本次实现说明

- deposit/index 已接入真实 `/v1/rider/withdraw`，并补充提现可用性提示、冻结保护与处理中反馈。
- 充值流程已接入支付取消识别、支付后状态轮询与延迟回刷，不再只依赖微信支付 success callback。
- 余额与账单均补充了页内错误态、重试入口、下拉刷新与翻页状态。
- 已完成 `npm run quality:check`，充值/提现的真机资金链路验证仍待执行。