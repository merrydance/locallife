# Phase1 规则执行入口（草案）

## 目标
- 在核心链路引入统一规则执行入口，支持灰度与可回退。

## 接入策略
- 默认 Noop（允许）
- 支持旁路模式（只记录命中，不影响结果）
- 支持强制模式（deny 直接拦截）
 - 通过 RULES_ENGINE_ENABLED 开关启用 DB 规则引擎
- gray_config 支持按 region_id/merchant_id/user_id 灰度生效

## 接入点（初版）
- 订单创建：createOrder
- 预订确认/下单
- 分账执行
- 索赔/异常裁决

## 备注
- 先接入最小路径，保证不影响现有逻辑。
