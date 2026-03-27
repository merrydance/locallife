# CARD-06 统一回调重复认领兜底与告警

状态：已完成（待评审）

优先级：P0

所属阶段：Phase 1

## 问题目标

统一 duplicate claim、stale claim、release fail、recovery scheduler 等失败分支的行为和告警，确保回调不会悄悄丢失。

## 影响范围

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- [locallife/db/sqlc/tx_notification.go](locallife/db/sqlc/tx_notification.go)
- [locallife/worker/wechat_notification_recovery_scheduler.go](locallife/worker/wechat_notification_recovery_scheduler.go)

## 任务内容

- [x] 盘点所有 tryClaimNotification / releaseNotification 的失败分支。
- [x] 统一失败路径的响应策略和告警模板。
- [x] 明确 stale claim 的释放与重试预期，避免不同回调类型各自为政。
- [x] 评估是否需要增加 metrics 或更明确的 reason 标签。

## 完成定义

- [x] 不同回调类型的 duplicate claim 行为一致。
- [x] release fail 不会被误判为已处理成功。
- [x] recovery scheduler 与主处理链语义一致，不会产生重复告警风暴。

## 验证要求

- [x] 增加 release fail 单测。
- [x] 增加 stale claim recovery 单测。
- [x] 验证 metrics / alert label 可区分失败类型。

## 依赖与风险

- 需要在 CARD-05 改完 ACK 策略后统一收口。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成

补充说明：

- `payment_callback.go` 现已把 duplicate/stale claim 的处理与 release fail 统一到共享 helper，release 失败会统一记录 `payment_callback_failures_total{type,reason}` 并携带 callback type、reason 发送告警。
- 主链已为 `payment`、`refund`、`ecommerce_refund`、`profit_sharing`、`combine_payment`、`applyment`、`settlement` 接入明确的 callback type 标签。
- `tx_notification.go` 本轮未改源代码；现有 claim/release 事务能力已满足本次语义收口，主要增量在 API 层的统一响应与观测。