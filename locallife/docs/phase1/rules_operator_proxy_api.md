# Phase1 运营商规则代理 API（草案）

## 目标
为运营商提供规则配置的受限入口，确保：
- 只操作自身 region_id 范围
- 不越权访问平台级规则
- 审计与灰度策略一致

## 设计原则
- 代理层仅转发平台规则 API 的安全子集
- 强制注入 region_id 约束（读写）
- 运营商仅能查看自身创建的规则与命中
- 写操作必须记录 actor_id/role

## 路由草案（运营商端）
- GET /v1/operators/me/rules
- GET /v1/operators/me/rules/{id}
- POST /v1/operators/me/rules
- POST /v1/operators/me/rules/{id}/versions
- POST /v1/operators/me/rules/{id}/publish
- POST /v1/operators/me/rules/{id}/rollback
- POST /v1/operators/me/rules/{id}/disable
- GET /v1/operators/me/rules/hits

## 访问控制
- 必须通过 `RoleOperator` 授权
- 规则 scope/gray_config 自动补充 operator 管辖 region_id
- 禁止 operator 调用平台端规则接口

## 数据隔离策略
- 读：仅返回 scope/gray_config 命中 operator 管辖 region_id 的规则
- 写：
  - scope/gray_config 若缺失 region_id，自动填充
  - 若包含 region_id，仅允许 operator 管辖范围

## API 行为对齐
- 规则状态与平台端一致：draft / active / disabled
- 发布/回滚/禁用逻辑与平台端一致
- 规则版本必须 status=published 才可发布

## 审计与命中
- rule_audits 记录 actor_role=operator
- rule_hits 仅返回 operator 管辖范围内的记录

## 后续事项
- 是否需要 operator 与 admin 共享规则（只读）
- 是否需要 operator 规则审核流
