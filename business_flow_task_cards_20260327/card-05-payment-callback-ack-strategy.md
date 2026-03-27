# CARD-05 修正重复认领未知状态时的回调 ACK 策略

状态：已完成（待评审）

优先级：P0

所属阶段：Phase 1

## 问题目标

在 duplicate claim 后查询本地通知失败时，不再返回 SUCCESS 吞掉微信后续重试。

## 影响范围

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go#L64)
- [locallife/api/payment_callback.go](locallife/api/payment_callback.go#L67)

## 任务内容

- [x] 将 duplicate claim lookup failed 从 ACK SUCCESS 改为可重试的 FAIL。
- [x] 检查同类逻辑是否同时出现在 payment、refund、combine payment、profit sharing、applyment 回调中。
- [x] 定义该分支的告警内容，避免只记录 warn 日志。
- [x] 确认返回 FAIL 时不会导致本地 claim 释放逻辑死锁。

## 完成定义

- [x] 未知状态时不再返回 SUCCESS。
- [x] 微信重试后仍能再次进入业务处理。
- [x] 失败分支有明确告警和可追踪上下文。

## 验证要求

- [x] 增加单测覆盖 duplicate claim + lookup failed。
- [x] 增加单测覆盖后续重试成功进入处理。
- [x] 跑 payment_callback 相关测试。

## 依赖与风险

- CARD-06 会继续补齐 release 失败与 stale claim 等其他恢复路径。

## 完成记录

- [x] 代码完成
- [x] 测试完成
- [ ] 评审完成