# Phase0 队列/回调容量验证清单（草案）

## 目标
- 验证支付回调与异步任务队列在峰值下的处理能力。

## 覆盖范围
- 支付回调：/v1/webhooks/wechat-pay/notify
- 退款回调：/v1/webhooks/wechat-pay/refund-notify
- 分账回调：/v1/webhooks/wechat-ecommerce/profit-sharing-notify
- 异步任务队列（asynq）：critical/default

## 验证项
- 回调峰值 QPS 压测下的成功率与延迟
- asynq 队列积压增长速率与清空时间
- 失败/重试比例

## 通过标准（起步值）
- 回调处理 P95 < 200ms（仅验证+入队）
- 回调错误率 < 0.5%
- 队列积压可在 10 分钟内回落
- 任务失败率 < 1%

## 备注
- 需要回调模拟器或压测工具注入请求
- 需接入 asynq/Redis 指标用于度量
