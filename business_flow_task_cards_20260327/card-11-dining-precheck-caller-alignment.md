# CARD-11 联动调用方收口预检权限表达

状态：已完成

优先级：P1

所属阶段：Phase 3

## 问题目标

确保预检接口语义修正后，调用方不会继续把“商户可管理”错误解释成“本人预约”。

## 影响范围

- [locallife/api/dining_session.go](locallife/api/dining_session.go)
- 相关前端或小程序调用方

## 任务内容

- [x] 检查 API response struct 是否需要新增字段表达商户管理权限。
- [x] 检查现有调用方是否依赖 is_reservation_owner 控制文案、按钮或跳转。
- [x] 如发生契约变更，补充接口说明和调用方适配。

## 完成定义

- [x] 新旧语义不会混淆。
- [x] 调用方不会展示错误入口或错误身份文案。

## 验证要求

- [x] 手工回归预检调用页面。
- [x] 若改动前端，执行对应 lint 或最小验证。

## 依赖与风险

- 依赖 CARD-10 完成接口语义修正。

## 调用面结论

- 后端 API 未新增字段。当前 `is_reservation_owner` 保持单一语义：仅表示“当前登录用户是否为该预约本人”。
- 小程序调用面仅发现一处消费：`weapp/miniprogram/pages/dining/index.ts` 只在 `reserved && is_reservation_owner` 时携带 `reservation_id` 去开台，请求语义正确，无需适配。
- Web 侧发现的 merchant 预检工具组件仅触发 `/dining-sessions/precheck`，并不消费 `is_reservation_owner` 返回值，不存在旧语义误用。
- 因当前调用方没有把“商户可管理”混入“本人预约”判断，本轮不新增 `can_manage_reservation` 等字段，避免无必要扩充契约。

## 完成记录

- [x] 调用方适配完成
- [x] 验证完成
- [x] 评审完成