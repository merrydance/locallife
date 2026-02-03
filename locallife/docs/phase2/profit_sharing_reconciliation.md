# Phase2 分账对账与差错处理（草案）

## 目标
- 形成可复核的分账对账口径
- 提供差错处理与人工介入路径

## 对账口径（建议）
- 时间窗口：按日/周/月
- 维度：status（pending/processing/finished/failed）
- 指标：订单数、GMV、平台/运营商分成合计

## 查询接口（草案）
- GetProfitSharingReconciliationSummary(start, end)
  - 输出：status、total_orders、total_amount、total_platform_commission、total_operator_commission
- API: GET /v1/platform/stats/profit-sharing/reconciliation
  - 参数：start_date、end_date

## 差错处理流程
1. 对账发现异常（失败率激增/金额不平）
2. 标记异常订单
3. 触发分账重试
4. 超过重试次数进入人工复核

## 风险控制
- 对账脚本只读
- 异常处理需记录审计日志
