# Phase2 分账规则配置化（草案）

## 目标
- 将分账比例从硬编码迁移为可配置规则
- 支持按区域/商户/订单类型覆盖
- 保留默认兜底规则，支持灰度与回滚

## 规则模型（简化版）
- 作用范围：
  - order_source（takeout/dine_in/takeaway/reservation/all）
  - region_id（可空）
  - merchant_id（可空）
- 配置字段：
  - platform_rate（平台分成百分比）
  - operator_rate（运营商分成百分比）
  - rider_enabled（是否启用骑手分成）
- 生效控制：status、priority、effective_at、expires_at

## 匹配优先级
1. merchant_id 匹配
2. region_id 匹配
3. order_source 精确匹配
4. order_source=all 兜底
5. priority 由小到大

## 数据表
- profit_sharing_configs
- profit_sharing_config_audits
  - 触发器自动写入 create/update/delete

## 后端接入点
- worker: ProcessTaskProfitSharing
  - 使用 GetActiveProfitSharingConfig 获取当前分账规则
  - 未命中时回落默认比例（2%/3%）

## 管理接口（草案）
- POST /v1/platform/profit-sharing/configs
- GET /v1/platform/profit-sharing/configs
- PATCH /v1/platform/profit-sharing/configs/{id}
- POST /v1/platform/profit-sharing/configs/{id}/disable

## 运营商只读接口（草案）
- GET /v1/operators/me/profit-sharing/configs

## 后续
- 配置后台（运营商/平台）
- 审计与变更记录
- 规则灰度与回滚策略
