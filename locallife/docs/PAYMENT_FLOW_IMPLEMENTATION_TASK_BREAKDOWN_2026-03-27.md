# Payment Flow 实施任务分解（函数级）

日期：2026-03-27

关联计划：
- locallife/docs/PAYMENT_FLOW_REMEDIATION_PLAN_2026-03-26.md

## 目标

把 P0/P1 的整改项拆到函数和测试用例级别，保证改动可并行、可回滚、可验收。

## P0-1 回调归属校验（先做）

### 代码改动

1. 文件：locallife/api/payment_callback.go
- 函数：handleCombinePaymentNotify
- 任务：
  - 新增对 combine_mchid 与当前服务商 mchid 的一致性校验。
  - 新增对 combine_appid 与当前服务商 appid 的一致性校验。
  - 对不匹配回调返回 FAIL（触发微信重试）并记录安全告警。

2. 文件：locallife/api/payment_callback.go
- 函数：handleEcommerceRefundNotify
- 任务：
  - 新增 sp_mchid 与当前服务商 mchid 一致性校验。
  - 不匹配时返回 FAIL，防止跨租户污染。

3. 文件：locallife/api/payment_callback.go
- 函数：handleProfitSharingNotify
- 任务：
  - 新增 mchid 校验（与服务商侧配置一致）。
  - 新增 sub_mchid 与本地分账单关联子商户的一致性校验（若无法一次性补齐，先告警并 FAIL）。

### 测试改动

1. 文件：locallife/api/payment_callback_test.go
- 用例新增：
  - 合单回调 combine_mchid 不匹配 -> 期望 FAIL。
  - 收付通退款回调 sp_mchid 不匹配 -> 期望 FAIL。

## P0-2 通知认领释放失败兜底（先做）

### 代码改动

1. 文件：locallife/db/sqlc/tx_notification.go
- 新增方法：MarkWechatNotificationProcessed
- 任务：
  - 在通知处理成功后写 processed_at。
  - 可选回填 out_trade_no/transaction_id，增强可观测性。

2. 文件：locallife/api/payment_callback.go
- 函数：tryClaimNotification / tryClaimApplymentNotification
- 任务：
  - 当 claim 失败（重复通知）时，读取现有记录：
    - processed_at 已写入 -> 返回 SUCCESS（已完成）。
    - processed_at 为空且创建时间较新 -> 返回 FAIL（处理中，允许重试）。
    - processed_at 为空且创建时间过久 -> 尝试释放占位并返回 FAIL（促使重试重新进入主逻辑）。

3. 文件：locallife/api/payment_callback.go
- 各成功出口任务：
  - handlePaymentNotify
  - handleRefundNotify
  - handleEcommerceRefundNotify
  - handleCombinePaymentNotify
  - handleProfitSharingNotify
  - handleSettlementNotify
- 任务：
  - 在返回 SUCCESS 前调用 markProcessed。

### 测试改动

1. 文件：locallife/api/payment_callback_test.go
- 用例新增：
  - 已认领但未 processed 的重复通知 -> 返回 FAIL。
  - 已认领且 processed 的重复通知 -> 返回 SUCCESS。

## P1-1 金额异常与恢复调度误推进（第二批）

### 代码改动

1. 文件：locallife/db/query/payment_order.sql
- SQL：ListPaidUnprocessedPaymentOrders
- 任务：
  - 排除“异常到账待退款”订单（通过新增标记字段或通过 refund_orders pending/processing 关联排除）。

2. 文件：locallife/worker/payment_recovery_scheduler.go
- 函数：runOnce
- 任务：
  - 增加恢复投递前的二次保护校验，避免异常到账订单进入 ProcessPaymentSuccess。

3. 文件：locallife/api/payment_callback.go
- 函数：金额异常分支（单笔与合单）
- 任务：
  - 统一写入异常标记或创建 pending refund_order 后再返回 SUCCESS。

### 测试改动

1. 文件：locallife/worker/payment_recovery_scheduler_test.go（如不存在则新增）
- 用例新增：
  - 金额异常订单不应被恢复调度入队。

## P1-2 无 OrderID 业务自动退款闭环（第二批）

### 代码改动

1. 文件：locallife/api/payment_callback.go
- 函数：金额异常分支（单笔、合单）
- 任务：
  - 去除仅 OrderID 可退款限制，改为以 payment_order_id 为中心分发退款。

2. 文件：locallife/worker/task_process_payment.go
- 函数：ProcessTaskAnomalyRefund
- 任务：
  - 为 membership_recharge、rider_deposit 提供商户定位或直连退款路径。
  - 无法自动化时输出可恢复告警并保持后续可重试。

### 测试改动

1. 文件：locallife/worker/task_process_payment_test.go
- 用例新增：
  - membership_recharge 金额异常可自动退款。
  - rider_deposit 金额异常走直连退款。

## 执行顺序

1. 先做 P0-1 与 P0-2（本轮开始）。
2. 跑 api/worker 相关单测并修复回归。
3. 再做 P1-1 与 P1-2。

## 最小验证命令

- go test ./api -run 'Test.*PaymentCallback|Test.*Wechat.*Notify'
- go test ./worker -run 'Test.*Payment.*|Test.*Refund.*'

