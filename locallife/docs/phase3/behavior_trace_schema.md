# 阶段 3：行为追溯数据口径（异常订单裁决）

## 核心概念
- 行为裁决：一次索赔/异常判定对应一条 `behavior_decision`。
- 追溯快照：按时间窗聚合的异常占比与数量 `behavior_trace_snapshot`。

## 追溯窗口
- 默认窗口：7 天、30 天（可配置）。
- 统计维度：
  - `total_orders`: 外卖订单数（已完成）
  - `abnormal_claims`: 异常索赔数（approved/auto-approved）
  - `abnormal_rate`: `abnormal_claims / total_orders`

## 追溯快照结构
- `decision_id`：关联裁决
- `window_days`：窗口天数
- `abnormal_count` / `total_count` / `abnormal_rate`
- `association_hits`：关联命中数组（如设备/地址共现）

## 数据口径统一
- “异常索赔”口径：`status IN ('auto-approved','approved')`。
- “完成订单”口径：`orders.status = 'completed'`。
- 订单/配送归因：外卖使用 `deliveries` 关联骑手，其他类型仅归属商户/用户。

## 规则引擎使用建议
- 规则条件以“窗口 + 异常率 + 绝对次数”组合，避免单一阈值误判。
- 低样本量需设置 `min_claims`/`min_orders`。

## 可解释输出
- 规则命中应输出：窗口、异常率、异常数、总单数、行为回溯摘要。
