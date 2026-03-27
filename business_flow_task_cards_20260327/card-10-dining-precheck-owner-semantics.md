# CARD-10 修正预检接口 reservation owner 语义

状态：已完成（待评审）

优先级：P1

所属阶段：Phase 3

## 问题目标

让 is_reservation_owner 只表达“当前用户是预约本人”，不再混入商户管理权限语义。

## 影响范围

- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go#L56)
- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go#L64)
- [locallife/logic/dining_session_precheck.go](locallife/logic/dining_session_precheck.go#L70)

## 任务内容

- [x] 把 is_reservation_owner 的赋值改为严格依据 reservation.user_id。
- [x] 如果仍需表达商户侧可管理权限，新增独立字段或内部状态。
- [x] 保持无权限用户仍然被正确拦截。

## 完成定义

- [x] 预约本人和商户侧查看者在响应中被正确区分。
- [x] 字段名和语义一致，不再误导调用方。

## 验证要求

- [x] 增加本人、商户 owner、商户员工、无权限用户四类单测。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成

补充说明：

- `is_reservation_owner` 现在仅表达“当前用户是否为预约本人”，不再把商户 owner 或商户员工混入 owner 语义。
- 商户侧的可见性仍由现有 merchant owner / merchant access 鉴权逻辑控制，不影响拦截行为。