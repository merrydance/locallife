# Phase1 规则模型设计（草案）

## 目标
- 统一规则定义，支持范围/条件/动作/版本/灰度/审计。

## 规则对象
- 范围（scope）
  - domain：order / reservation / payment / profit_sharing / claim
  - region_id / merchant_id / user_role / order_type
- 条件（condition）
  - 状态、金额、时间、行为、频次
- 动作（action）
  - allow / deny / adjust_price / compensate / manual_review / alert
- 元数据
  - priority、version、status、effective_at、expires_at

## 灰度策略
- 维度：region_id、merchant_id、用户比例
- 策略：白名单优先 → 百分比放量 → 全量
- gray_config 示例：
  - {"region_id": [1101, 1102]}
  - {"merchant_id": [20001, 20002]}
  - {"user_id": [30001, 30002]}

## 审计与回放
- 规则命中记录（rule_id/version/decision/inputs/outputs）
- 支持回放与复盘

## 备注
- 本模型为 Phase1 草案，后续以最小可用为先。
