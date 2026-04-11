# LocalLife 后端订单链路与资金链路梳理

## 1. 范围与口径

本次梳理范围限定为后端生产路径，目标是回答两件事：

1. 订单从创建到完成、取消、超时、配送、打印的主链路在哪里。
2. 与金钱相关的所有主链路在哪里，包括支付、退款、分账、充值、押金、提现、赔付、追偿、账单流水与恢复任务。

梳理口径说明：

- 以可达的 API、logic、db/sqlc、worker、scheduler、webhook 路径为准。
- 优先记录真实生产入口，不把纯测试代码和只读统计查询当作资金动作主链路。
- 会补充少量结构性观察，指出当前代码里容易产生认知偏差或后续漂移的地方。

## 2. 订单主链路

### 2.1 下单建单

主入口：

- [api/order.go](../api/order.go) 的 `createOrder`
- [logic/order_service.go](../logic/order_service.go) 的 `OrderService.CreateOrder`
- [db/sqlc/tx_create_order.go](../db/sqlc/tx_create_order.go) 的 `CreateOrderTx`

主流程：

1. API 绑定请求并校验订单类型字段。
2. logic 层校验商户、桌台、预订、账单组、菜品定制、配送报价、优惠券、会员余额、预订押金抵扣。
3. 计算 `subtotal`、`discount_amount`、`delivery_fee`、`voucher_amount`、`balance_paid`、`total_amount`。
4. 通过 `CreateOrderTx` 原子创建：
   - `orders`
   - `order_items`
   - `order_status_log`
   - 可选的 `billing_group_orders`
   - 优惠券占用
   - 会员余额扣减与会员交易流水
5. 初始订单状态统一为 `pending`。
6. 创建成功后由 `OrderService.CreateOrder` 调度 30 分钟支付超时任务。

关键特点：

- 会员余额全额支付时，`CreateOrderTx` 会在同一事务中直接调用 `processOrderPaymentWithQueries`，避免“余额已扣但订单仍 pending”。
- 预订类订单会先走会话/预订校验，且押金模式下限制单预订只允许一个活跃订单。

### 2.2 订单支付发起

主入口：

- 单订单/预订支付：[api/payment_order.go](../api/payment_order.go) 的 `createPaymentOrder`
- 多订单合单支付：[api/payment_order.go](../api/payment_order.go) 的 `createCombinedPaymentOrder`

核心服务：

- [logic/payment_order_service.go](../logic/payment_order_service.go) 的 `PaymentOrderService.CreatePaymentOrder`
- [logic/combined_payment_service.go](../logic/combined_payment_service.go) 的 `CombinedPaymentService.CreateCombinedPaymentOrder`

当前真实主路径：

- 业务口径上，`order` 与 `reservation` 主支付都按“单订单的平台收付通普通支付”处理，不以商户数作为链路选择条件。
- 当前产品里，预约、堂食、外带天然只会生成 1 笔订单，因此都走平台收付通普通支付。
- 外卖只有在一次结算里生成多笔订单时才走合单支付；当前实际会触发这一路径的场景是多商户外卖购物车一起下单。
- 如果外卖只选择了 1 个商户，即使该商户下有多个商品，也只会生成 1 笔订单，因此仍走平台收付通普通支付，不走合单支付。
- [api/payment_order.go](../api/payment_order.go) 会把 `payment_type` 归一成兼容字段，旧客户端传 `native` 不再改变底层物理支付链路。

落库路径：

- 平台收付通普通支付：订单主支付见 [db/sqlc/tx_create_partner_payment.go](../db/sqlc/tx_create_partner_payment.go) 的 `CreatePartnerPaymentTx`
- 预订等普通支付结构见 [db/sqlc/tx_create_ecommerce_payment.go](../db/sqlc/tx_create_ecommerce_payment.go) 的 `CreateEcommercePaymentTx`
- 多订单合单支付：`CreateCombinedPaymentTx`，见 [db/sqlc/tx_create_combined_payment.go](../db/sqlc/tx_create_combined_payment.go)
- 合单关闭：`CloseCombinedPaymentOrderTx`，见 [db/sqlc/tx_close_combined_payment.go](../db/sqlc/tx_close_combined_payment.go)

超时处理：

- 平台收付通普通支付超时：[worker/task_payment_timeout.go](../worker/task_payment_timeout.go)
- 合单支付超时：[worker/task_combined_payment_timeout.go](../worker/task_combined_payment_timeout.go)
- 订单支付超时：[worker/task_order_timeout.go](../worker/task_order_timeout.go)

### 2.3 支付成功后的订单推进

核心事务：

- [db/sqlc/tx_payment_success.go](../db/sqlc/tx_payment_success.go) 的 `ProcessPaymentSuccessTx`
- [db/sqlc/tx_create_order.go](../db/sqlc/tx_create_order.go) 的 `processOrderPaymentWithQueries`

订单支付成功时的关键动作：

1. 锁定 `payment_orders` 做幂等判断。
2. 对 `business_type=order` 的支付单，调用 `processOrderPaymentWithQueries`。
3. 锁定订单与库存，做库存扣减。
4. 将订单从 `pending` 更新到 `paid`。
5. 预订订单会释放预留库存再转实销。

这里的边界非常重要：

- 支付成功只把订单推进到 `paid`，不会直接让商户进入 `preparing`。
- 外卖配送单与配送池并不是在支付成功时创建，而是在商户标记 `ready` 时进入下一阶段。

### 2.4 商户履约链路

入口：

- 商户接单：[api/order.go](../api/order.go) 的 `acceptOrder`
- 商户拒单：[api/order.go](../api/order.go) 的 `rejectOrder`
- 商户出餐完成：[api/order.go](../api/order.go) 的 `markOrderReady`
- 商户完成堂食/自取订单：[api/order.go](../api/order.go) 的 `completeOrder`

核心逻辑：

- [logic/merchant_order.go](../logic/merchant_order.go)
- [logic/order_service.go](../logic/order_service.go)

状态推进：

- `paid -> preparing`：`AcceptMerchantOrder`
- `paid -> cancelled`：`RejectMerchantOrder`
- `preparing -> ready`：`MarkMerchantOrderReady`
- `ready -> completed`：`CompleteMerchantOrder`，仅非外卖订单

外卖的特殊处理：

- 接单用 [db/sqlc/tx_takeout_order.go](../db/sqlc/tx_takeout_order.go) 的 `AcceptTakeoutOrderTx`
- 出餐时用同文件的 `MarkTakeoutOrderReadyTx`
- `ready` 时才创建 `deliveries` 并放入 `delivery_pool`

### 2.5 配送履约链路

核心逻辑：

- 抢单：[logic/delivery_grab.go](../logic/delivery_grab.go) 的 `GrabDeliveryOrder`
- 开始取餐、确认取餐、开始配送、确认送达：[logic/delivery_status.go](../logic/delivery_status.go)
- 用户确认收货：[logic/confirm_order.go](../logic/confirm_order.go) 的 `ConfirmTakeoutOrder`

订单状态主线：

- `ready -> courier_accepted`：骑手抢单
- `courier_accepted -> picked`：骑手确认取餐
- `picked -> delivering`：骑手开始配送
- `delivering -> rider_delivered`：骑手确认送达
- `rider_delivered -> completed`：用户确认收货

资金相关联动：

- 抢单会冻结骑手押金。
- 送达完成时会解冻押金并结转配送费相关状态。
- 外卖分账并不在支付成功立刻触发，而是等待微信结算事件。

### 2.6 取消、改菜、超时、打印

取消与改菜：

- 用户取消：[logic/order_service.go](../logic/order_service.go) 的 `CancelOrder`
- 商户拒单触发退款：[logic/merchant_reject_refund.go](../logic/merchant_reject_refund.go)
- 预订全款改菜：[api/order.go](../api/order.go) 的 `replaceOrder`，对应 [logic/replace_order.go](../logic/replace_order.go)

超时：

- 订单超时自动取消：[worker/task_order_timeout.go](../worker/task_order_timeout.go)
- 支付单超时关闭并尝试取消业务订单：[worker/task_payment_timeout.go](../worker/task_payment_timeout.go)

打印：

- 接单/出餐触发打印：[logic/order_service.go](../logic/order_service.go) 的 `scheduleOrderPrint`
- 执行打印任务：[worker/task_print_order.go](../worker/task_print_order.go)
- 打印异常和重试链路挂在 `print_log` 及相关调度逻辑上

## 3. 资金相关链路总表

| 链路 | 主入口 | 主要落库/事务 | 第三方接口 | 异步/恢复 |
| --- | --- | --- | --- | --- |
| 平台收付通普通支付 | [api/payment_order.go](../api/payment_order.go) | `CreateEcommercePaymentTx` | `CreateCombineOrder` | 支付超时、支付回调、支付恢复 |
| 多订单合单支付 | [api/payment_order.go](../api/payment_order.go) | `CreateCombinedPaymentTx` | `CreateCombineOrder` | 合单超时、合单回调 |
| 预订押金支付 | [api/payment_order.go](../api/payment_order.go) | `CreateEcommercePaymentTx` | `CreateCombineOrder` | 支付成功后预订状态推进 |
| 预订加菜补差 | [logic/reservation_dishes.go](../logic/reservation_dishes.go) | `CreateEcommercePaymentTx` | `CreateCombineOrder` | 支付成功后 `reservation_addon` 处理 |
| 会员充值 | [api/membership.go](../api/membership.go) | `CreateEcommercePaymentTx` | `CreateCombineOrder` | 支付成功后会员余额入账 |
| 骑手押金充值 | [api/rider.go](../api/rider.go) | `CreatePaymentOrder` | `CreateJSAPIOrder` | 支付成功后押金入账 |
| 索赔追偿支付 | [api/claim_recovery.go](../api/claim_recovery.go) | `CreatePaymentOrder` | `CreateJSAPIOrder` | 支付成功后追偿单置 paid |
| 用户取消退款 | `CancelOrder -> ProcessTaskInitiateRefund` | `CreateRefundOrderTx` | 直连退款或收付通退款 | 退款结果回调、退款恢复 |
| 商户拒单退款 | [logic/merchant_reject_refund.go](../logic/merchant_reject_refund.go) | `CreateRefundOrderTx` | `CreateRefund` / `CreateEcommerceRefund` | 退款恢复 |
| 预订取消退款 | [logic/reservation.go](../logic/reservation.go) | `CreateRefundOrderTx` | `CreateEcommerceRefund` | 退款恢复 |
| 预订改菜退款 | [logic/reservation_dishes.go](../logic/reservation_dishes.go) | `CreateRefundOrder` | `CreateEcommerceRefund` | 无专门事务锁保护 |
| 骑手押金提现/退款 | [api/rider.go](../api/rider.go) | `PrepareRiderDepositRefundTx` / `ResolveRiderDepositRefundTx` | `CreateRefund` | 退款结果回调 |
| 商户提现 | [api/merchant_finance.go](../api/merchant_finance.go) | `CreateWithdrawalRecord` | `CreateEcommerceWithdraw` | 提现结果轮询恢复 |
| 运营商提现 | [api/operator_finance.go](../api/operator_finance.go) | `CreateWithdrawalRecord` | `CreateEcommerceWithdraw` | 提现结果轮询恢复 |
| 分账 | `支付成功后任务` 或 `结算事件回调` | `CreateProfitSharingOrder` 等 | `CreateProfitSharing` | 分账结果回调、分账恢复 |
| 分账回退 | 退款任务内 | `CreateProfitSharingReturn` | `CreateProfitSharingReturn` | 回退结果轮询恢复 |
| 索赔赔付/申诉补偿 | 风控提交索赔后分发任务 | `behavior_action` / `ReviewAppealWithCompensationTx` | `CreateTransfer` / `QueryTransfer` | 赔付恢复调度器 |
| 用户支付账单流水 | [api/payment_ledger.go](../api/payment_ledger.go) | `ListPaymentLedgerEntriesByUser` | 无 | 只读查询 |

## 4. 各资金链路展开

### 4.1 平台收付通普通支付

生产入口：

- [api/payment_order.go](../api/payment_order.go)
- [logic/payment_order_service.go](../logic/payment_order_service.go)

当前实现要点：

- 业务语义上，`order` 和 `reservation` 主支付都属于平台收付通普通支付，不走微信多订单合单下单接口。
- 当前产品里，预约、堂食、外带，以及单商户外卖，都会落到这条平台收付通普通支付链路。
- 外卖如果只选了一个商户，即使包含多个商品，也只生成一笔订单，因此仍是平台收付通普通支付。
- 只有一次结算需要支付多笔订单时才进入合单支付；当前真实业务场景是多商户外卖购物车一起下单。
- 个别单商户收付通支线会复用 `combined_payment_orders + payment_orders` 的本地结构承接回调与恢复，但这不改变其业务口径仍是平台收付通普通支付。
- 幂等复用已有 pending 支付单，并在已有 `prepay_id` 时重新签名生成 `pay_params`。

### 4.2 合单支付

生产入口：

- [logic/combined_payment_service.go](../logic/combined_payment_service.go)

特点：

- 合单支付的定义是“一次支付多笔订单”，不是“跨商户”本身；只是当前产品里多订单场景恰好来自多商户外卖结算。
- 外卖若只选择一个商户，只会生成一笔订单，应走平台收付通普通支付而不是合单支付。
- 支持最多 50 个订单合并支付。
- 在微信下单失败时会主动把本地合单和子支付单关闭，避免僵尸数据。
- 合单回调由 [api/payment_callback.go](../api/payment_callback.go) 的 `handleCombinePaymentNotify` 处理，对应独立地址 `/v1/webhooks/wechat-ecommerce/combine-notify`。

### 4.3 直连支付支线

这部分不是当前用户点单主路径，但仍是生产资金链：

- 骑手押金充值：[api/rider.go](../api/rider.go)
- 索赔追偿支付：[logic/claim_recovery_payment.go](../logic/claim_recovery_payment.go)

共性：

- 直接创建 `payment_orders`
- 调用 [wechat/payment.go](../wechat/payment.go) 的直连支付客户端 `CreateJSAPIOrder`
- 支付回调走 `handlePaymentNotify`

### 4.4 支付回调与支付成功处理

回调入口：

- 直连支付回调：`handlePaymentNotify`
- 平台收付通普通支付回调：`handleEcommercePaymentNotify`
- 合单支付回调：`handleCombinePaymentNotify`
- 订单结算事件：`handleOrderSettlementNotify`
- 直连退款回调：`handleRefundNotify`
- 收付通退款回调：`handleEcommerceRefundNotify`
- 分账回调：`handleProfitSharingNotify`

共同机制：

- 验签
- 通知去重占位 `TryClaimWechatNotification`
- 归属校验
- 快速返回微信
- 后续业务通过 worker 异步推进

支付成功异步处理入口：

- [worker/task_process_payment.go](../worker/task_process_payment.go) 的 `ProcessTaskPaymentSuccess`

按 `business_type` 分流到：

- `order`
- `reservation`
- `reservation_addon`
- `membership_recharge`
- `rider_deposit`
- `claim_recovery`

### 4.5 退款链路

主入口分三类：

1. 用户取消订单后调度退款任务。
2. 商户拒单时直接发起退款。
3. 支付金额异常或关闭订单后仍到账时，系统自动触发异常退款。

核心文件：

- [worker/task_process_payment.go](../worker/task_process_payment.go) 的 `ProcessTaskInitiateRefund`
- [db/sqlc/tx_refund.go](../db/sqlc/tx_refund.go) 的 `CreateRefundOrderTx`
- [logic/merchant_reject_refund.go](../logic/merchant_reject_refund.go)
- [worker/refund_recovery_scheduler.go](../worker/refund_recovery_scheduler.go)

关键差异：

- 普通订单退款会先处理分账回退，再做收付通退款。
- 骑手押金提现本质上也是退款，只是按押金信用拆分到多笔 `refund_orders`。
- 退款结果真正落库在 `ProcessTaskRefundResult`。

### 4.6 会员充值

入口：

- [api/membership.go](../api/membership.go)
- 预处理规则匹配：[logic/membership_recharge.go](../logic/membership_recharge.go)

到账：

- [db/sqlc/tx_payment_success.go](../db/sqlc/tx_payment_success.go) 中 `membership_recharge` 分支
- 增加会员本金、赠金、余额，并写 `membership_transactions`

### 4.7 骑手押金充值与提现

充值：

- [api/rider.go](../api/rider.go) 创建 `rider_deposit` 类型支付单
- 支付成功后在 `ProcessPaymentSuccessTx` 中：
  - 增加骑手押金余额
  - 写 `rider_deposits`
  - 写 `rider_deposit_credits`

提现：

- [api/rider.go](../api/rider.go) 的 `withdrawRider`
- [logic/rider_deposit_refund_service.go](../logic/rider_deposit_refund_service.go)
- [db/sqlc/tx_rider_refund.go](../db/sqlc/tx_rider_refund.go)

这条链本质不是微信提现接口，而是把历史押金充值单拆成若干笔微信退款，逐笔退回。

### 4.8 商户提现与运营商提现

入口：

- 商户提现：[api/merchant_finance.go](../api/merchant_finance.go)
- 运营商提现：[api/operator_finance.go](../api/operator_finance.go)

共同模式：

1. 先创建 `withdrawal_records`
2. 调用 `CreateEcommerceWithdraw`
3. 立即保存 `withdraw_id` 与状态
4. 通过 [api/payment_callback.go](../api/payment_callback.go) 的提现回调处理微信 `MCHWITHDRAW.CHANGE` 事件，并在 durable 落库后 ack
5. 通过 [worker/task_merchant_withdraw_result.go](../worker/task_merchant_withdraw_result.go) 轮询微信提现结果，作为回调缺失或延迟时的恢复兜底
6. [worker/merchant_withdraw_recovery_scheduler.go](../worker/merchant_withdraw_recovery_scheduler.go) 做 pending 恢复

### 4.9 分账、结算与分账回退

主入口：

- [worker/task_process_payment.go](../worker/task_process_payment.go) 的 `ProcessTaskProfitSharing`
- 外卖结算触发：[api/payment_callback.go](../api/payment_callback.go) 的 `handleOrderSettlementNotify`

当前分账策略：

- 堂食/自取/预订：支付成功后即可分账或标记 finished
- 外卖：等待微信 `trade_manage_order_settlement` 事件后才触发真正分账
- 分账对象可包括平台、运营商、骑手个人收款账号

恢复机制：

- [worker/profit_sharing_recovery_scheduler.go](../worker/profit_sharing_recovery_scheduler.go)
- 同时负责：
  - failed/stale 分账重试
  - 已完成订单但缺失分账单的补偿创建
  - 卡在 processing 的分账回退单轮询补偿

查询/财务展示：

- 商户结算列表与时间线：[api/merchant_finance.go](../api/merchant_finance.go)

### 4.10 索赔赔付与追偿

这里有两条相反方向的资金链：

1. 平台/商户/骑手向用户赔付
2. 商户/骑手向平台补缴追偿

赔付链：

- 异步任务：[worker/task_claim_refund.go](../worker/task_claim_refund.go)
- 恢复调度器：[worker/claim_refund_recovery_scheduler.go](../worker/claim_refund_recovery_scheduler.go)
- 底层支付接口：`CreateTransfer` / `QueryTransfer`
- 持久化 outbox：`behavior_action`

追偿链：

- 商户支付入口：[api/claim_recovery.go](../api/claim_recovery.go)
- 骑手支付入口：[api/claim_recovery.go](../api/claim_recovery.go)
- 逻辑实现：[logic/claim_recovery_payment.go](../logic/claim_recovery_payment.go)
- 支付成功后在 `ProcessPaymentSuccessTx` 的 `claim_recovery` 分支把追偿单标记为已支付，并解除对应限制

### 4.11 支付账单流水

入口：

- [api/payment_ledger.go](../api/payment_ledger.go)
- [logic/payment_ledger_service.go](../logic/payment_ledger_service.go)

范围：

- 面向用户侧，只展示支付与退款流水
- 不是商户/运营商结算总账，也不覆盖提现流水

## 5. 恢复与补偿链路总览

| 恢复器/任务 | 作用 |
| --- | --- |
| [worker/payment_recovery_scheduler.go](../worker/payment_recovery_scheduler.go) | paid 但未 processed 的支付单重新入队 |
| [worker/refund_recovery_scheduler.go](../worker/refund_recovery_scheduler.go) | 已取消但未退的订单/预订重新触发退款 |
| [worker/merchant_withdraw_recovery_scheduler.go](../worker/merchant_withdraw_recovery_scheduler.go) | 商户/运营商 pending 提现结果轮询 |
| [worker/profit_sharing_recovery_scheduler.go](../worker/profit_sharing_recovery_scheduler.go) | 分账失败、缺失、回退卡死的补偿 |
| [worker/claim_refund_recovery_scheduler.go](../worker/claim_refund_recovery_scheduler.go) | 赔付动作 created/failed/running 的恢复执行 |
| [worker/task_payment_timeout.go](../worker/task_payment_timeout.go) | 支付单超时关闭与业务订单取消 |
| [worker/task_order_timeout.go](../worker/task_order_timeout.go) | 订单支付超时自动取消 |

## 6. 结构性观察与审查备注

### 6.1 `createPaymentOrder` 的对外字段与真实支付链已经分离

- API 仍暴露 `payment_type=native|miniprogram`，但 [api/payment_order.go](../api/payment_order.go) 会统一归一。
- 对 `order` / `reservation` 来说，真实主路径已经是收付通，不再由客户端支付类型决定。

这意味着：

- 前端看到的字段仍有兼容意义。
- 后端审查时应把“主支付链”定位到收付通，而不是历史直连支付。

### 6.2 `PaymentOrderService.CreatePaymentOrder` 中仍保留旧直连创建代码段

- [logic/payment_order_service.go](../logic/payment_order_service.go) 里，`order` 与 `reservation` 在前面已经提前返回到收付通路径。
- 后面仍保留一段“订单走直连或扫码支付”的旧代码。

就当前合法输入而言，这段代码对 `order` / `reservation` 已不再是主执行路径，属于容易让阅读者误判的残留实现。

### 6.3 运营商提现的事务版实现与公开 API 路径存在漂移

- [db/sqlc/tx_withdraw_operator.go](../db/sqlc/tx_withdraw_operator.go) 提供了 `WithdrawOperatorTx`
- 但当前公开 API [api/operator_finance.go](../api/operator_finance.go) 并未走这个事务，而是直接：
  - 创建 `withdrawal_record`
  - 调用微信提现接口
  - 再靠轮询更新状态

这说明 `WithdrawOperatorTx` 更像保留实现或未接通实现，后续维护时需要避免把它误当作当前生产主路径。

### 6.4 预订改菜退款路径没有复用退款并发保护事务

- [logic/reservation_dishes.go](../logic/reservation_dishes.go) 的部分退款路径直接调用 `CreateRefundOrder`
- 没有像主退款链那样统一走 [db/sqlc/tx_refund.go](../db/sqlc/tx_refund.go) 的 `CreateRefundOrderTx`

这意味着该支线没有复用“锁 payment_order + 校验累计已退款额”的标准并发保护，属于后续值得重点关注的资金分支。

## 7. 本次梳理结论

可以把当前后端理解成四层收口：

1. 订单主链以 `orders` 为中心，状态从 `pending -> paid -> preparing/ready -> 配送 -> completed/cancelled`。
2. 资金主链以 `payment_orders` / `combined_payment_orders` / `refund_orders` / `profit_sharing_orders` / `withdrawal_records` 为中心。
3. 支付、退款、分账、提现、赔付都依赖 webhook + worker + recovery scheduler 形成最终一致性。
4. 订单主支付已经基本统一到收付通体系，直连支付主要退居到押金、追偿等专用支线。
