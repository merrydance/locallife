# Phase2 分账规则审计与变更记录（草案）

## 目标
- 记录分账规则配置的变更轨迹
- 支持问题追溯与回滚依据

## 数据表
- profit_sharing_config_audits
  - action: create/update/delete
  - detail: before/after 快照
  - actor_id/actor_role：预留（当前由触发器写入为空）

## 触发器
- profit_sharing_configs 发生 INSERT/UPDATE/DELETE 时自动写入审计记录
- 由应用层设置 session 变量：app.actor_id / app.actor_role
- 可选：app.actor_detail（禁用原因等）

## 查询接口（草案）
- API: GET /v1/platform/stats/profit-sharing/config-audits
  - 参数：config_id（可选）、page、limit

## 后续
- 在管理 API 中补齐 actor 信息
- 提供审计查询接口与页面
