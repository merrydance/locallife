# Phase2 分账 SLA 与延迟监控（草案）

## 目标
- 衡量分账处理时效与稳定性
- 形成可监控的 SLA 指标口径

## 指标口径
- 总订单数、已完成、失败、待处理
- 平均完成耗时（秒）
- P95 完成耗时（秒）

## 查询接口（草案）
- GetProfitSharingSlaSummary(start, end)
- API: GET /v1/platform/stats/profit-sharing/sla
  - 参数：start_date、end_date

## 使用建议
- 以日/周为窗口监控 SLA 变化
- 失败率或 P95 突升触发告警
