# Phase2 分账失败重试与补偿（草案）

## 目标
- 自动发现分账失败/超时记录并重试
- 降低人工介入与资金对账风险

## 调度策略
- 定时扫描 `profit_sharing_orders` 状态为 `pending/failed/processing` 且创建时间超过 10 分钟
- 批量重试（默认每次 200 条）

## 实现要点
- 调度器：ProfitSharingRecoveryScheduler
- 任务：payment:process_profit_sharing
- 重试队列：critical

## 风险控制
- 仅对超过最小年龄的记录重试
- 重试次数限制（MaxRetry=5）
- 若 payment_order 缺少 order_id，跳过并记录告警

## 后续
- 增加失败原因记录与人工处理入口
- 对接对账报表，标记无法自动恢复的记录
