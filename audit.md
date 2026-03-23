正在优化工具选择...我会先做一次“支付链路全景盘点”，定位所有下单、预支付、回调、退款、对账、幂等等关键路径代码，再逐条做生产级鲁棒性审查并给出能否直接上线的结论。先从代码检索和测试覆盖现状开始。

搜索与 `**/locallife/**/*.go` 匹配的文件，574 匹配项

搜索正则表达式 `支付|pay|payment|refund|wechat|wxpay|alipay|notify|callback|trade|bill|reconcile` (`**/locallife/**/*.go`)，200 个结果

读取内存 weapp-performance.md

已定位到支付相关核心模块集中在 api、logic、worker、wechat、db/sqlc。下一步我会重点读取“创建支付单、回调验签、状态迁移、退款、超时补偿、对账”这些实现与测试，产出按严重级别排序的问题清单。

审查支付链路代码

搜索正则表达式 `payment|prepayment` (`locallife` · `locallife/**/*.go`)，30 个结果

搜索与 `locallife` · `locallife/**/*payment*.go` 匹配的文件，20 匹配项

搜索与 `locallife` · `locallife/**/*refund*.go` 匹配的文件，10 匹配项

搜索正则表达式 `callback|webhook` (`locallife` · `locallife/**/*.go`)，23 个结果

搜索正则表达式 `reconcil|billing` (`locallife` · `locallife/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/api/payment_order.go#1-1)，行 1 到 150

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service.go#1-1)，行 1 到 100

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#1-1)，行 1 到 150

读取 [](file:///home/sam/locallife/locallife/logic/refund_service.go#1-1)，行 1 到 100

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#150-150)，行 150 到 350

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service.go#100-100)，行 100 到 300

读取 [](file:///home/sam/locallife/locallife/worker/payment_recovery_scheduler.go#1-1)，行 1 到 100

读取 [](file:///home/sam/locallife/locallife/worker/task_payment_timeout.go#1-1)，行 1 到 100

读取 [](file:///home/sam/locallife/locallife/logic/refund_service.go#50-50)，行 50 到 200

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#150-150)，行 150 到 350

搜索正则表达式 `BeginTx|Tx\(|transaction|Transaction|commit|rollback` (`locallife` · `locallife/**/*.go`)，19 个结果

读取 [](file:///home/sam/locallife/locallife/worker/refund_recovery_scheduler.go)

搜索文本 `refund` (`locallife` · `locallife/api/*refund*.go`)，无结果

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#200-200)，行 200 到 350

搜索匹配 `locallife` · `locallife/api/*refund*.go` 的文件，无匹配项

搜索正则表达式 `ProcessPaymentSuccessTx|Tx\(` (`locallife` · `locallife/db/sqlc/*.go`)，15 个结果

搜索正则表达式 `payment.*Tx|refund.*Tx|ProcessPayment` (`locallife` · `locallife/db/sqlc/*.go`)，15 个结果

搜索与 `locallife` · `locallife/db/sqlc/*payment*.go` 匹配的文件，7 匹配项

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#150-150)，行 150 到 355

搜索正则表达式 `refund.*notify|refund.*callback` (`locallife` · `locallife/worker/*.go`)，无结果

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#200-200)，行 200 到 355

搜索文本 `refund` (`locallife` · `locallife/api/payment_callback.go`)，50 个结果

读取 [](file:///home/sam/locallife/locallife/worker/refund_recovery_scheduler.go#147-147)，行 147 到 147

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_payment_success.go)

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#150-150)，行 150 到 355

搜索正则表达式 `refund_recovery|Refund` (`locallife` · `locallife/worker/*.go`)，10 个结果

搜索匹配 `locallife` · `locallife/integration/*payment*.go` 的文件，无匹配项

读取 [](file:///home/sam/locallife/locallife/worker/task_claim_refund.go)

读取 [](file:///home/sam/locallife/locallife)

搜索文本 `payment` (`locallife` · `locallife/**/*test.go`)，无结果

搜索文本 `bill|reconcil` (`locallife` · `locallife/worker/*.go`)，20 个结果

搜索匹配 `locallife` · `locallife/worker/*bill*.go` 的文件，1 个匹配项

搜索文本 `POST` (`locallife` · `locallife/api/payment_order.go`)，5 个结果

读取 [](file:///home/sam/locallife/locallife/api/payment_order.go#150-150)，行 150 到 300

搜索文本 `combined` (`locallife` · `locallife/logic/combined_payment_service.go`)，30 个结果

读取 [](file:///home/sam/locallife/locallife/worker/bill_reconciliation_scheduler.go#1-1)，行 1 到 200

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_payment_success.go#27-27)，行 27 到 140

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#100-100)，行 100 到 150

搜索文本 `close` (`locallife` · `locallife/api/payment_order.go`)，18 个结果

搜索文本 `Update` (`locallife` · `locallife/db/sqlc/payment_order.sql.go`)，30 个结果

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#100-100)，行 100 到 120

搜索文本 `Check` (`locallife` · `locallife/db/sqlc/wechat_notification.sql.go`)，4 个结果

读取 [](file:///home/sam/locallife/locallife/logic/combined_payment_service.go#1-1)，行 1 到 100

搜索文本 `ProcessPaymentSuccess` (`locallife` · `locallife/worker/task_process_payment.go`)，6 个结果

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#100-100)，行 100 到 420

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#200-200)，行 200 到 400

搜索文本 `ClosePaymentOrder` (`locallife` · `locallife/logic/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/api)

已创建记忆文件 [](file:///home/sam/.config/Code/User/workspaceStorage/daccee4eb373b9faf9120a9da7f9f219/GitHub.copilot-chat/memory-tool/memories/NDZlYzhjOWUtODEwMC00YTIwLWFhNmUtMGU5YmJiMTYzYmI3/payment_audit.md)

读取 [](file:///home/sam/.config/Code/User/workspaceStorage/daccee4eb373b9faf9120a9da7f9f219/GitHub.copilot-chat/chat-session-resources/46ec8c9e-8100-4a20-aa6e-0e9bbb163bb7/call_N1Z9YNmDJMBj87cgXzrtcSba__vscode-1774148769820/content.txt#1-1)，行 1 到 260

读取 [](file:///home/sam/.config/Code/User/workspaceStorage/daccee4eb373b9faf9120a9da7f9f219/GitHub.copilot-chat/chat-session-resources/46ec8c9e-8100-4a20-aa6e-0e9bbb163bb7/call_N1Z9YNmDJMBj87cgXzrtcSba__vscode-1774148769820/content.txt#261-261)，行 261 到 415

我拿到了初步风险清单，接下来会逐条在源码里做“证据校验”（函数实现+行号+是否真实可触发），避免误报，再给你最终是否可上线结论。

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#1-1)，行 1 到 420

读取 [](file:///home/sam/locallife/locallife/worker/task_payment_timeout.go)

读取 [](file:///home/sam/locallife/locallife/logic/refund_service.go#1-1)，行 1 到 260

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#420-420)，行 420 到 760

我已经确认回调侧确实有“检查与落库分离”的并发窗口。现在继续审查退款、超时任务、支付成功事务和合单事务，确认是否还有会导致资金错账的关键缺陷。

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#1-1)，行 1 到 360

读取 [](file:///home/sam/locallife/locallife/worker/payment_recovery_scheduler.go)

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_payment_success.go)

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service.go#260-260)，行 260 到 392

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#360-360)，行 360 到 920

读取 [](file:///home/sam/locallife/locallife/logic/combined_payment_service.go)

读取 [](file:///home/sam/locallife/locallife/worker/refund_recovery_scheduler.go)

读取 [](file:///home/sam/locallife/locallife/worker/bill_reconciliation_scheduler.go#1-1)，行 1 到 340

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_create_combined_payment.go)

读取 [](file:///home/sam/locallife/locallife/worker/bill_reconciliation_scheduler.go#340-340)，行 340 到 432

我开始做精确行号定位与“是否已被 SQL 条件约束”核对，尤其是 UpdatePaymentOrderToPaid 的 SQL 细节和通知表唯一约束，这能判断哪些问题是确定性缺陷。

搜索正则表达式 `func \(q \*Queries\) UpdatePaymentOrderToPaid|UPDATE payment_orders\s+SET status = 'paid'|CheckNotificationExists|CreateWechatNotification` (`**/locallife/db/sqlc/**/*.go`)，37 个结果

搜索正则表达式 `notification already processed|failed to record notification ID|UpdatePaymentOrderToPaid\(|amount mismatch|manual review required` (`**/locallife/api/payment_callback.go`)，16 个结果

搜索正则表达式 `ClosePaymentOrder\(|CloseOrder\(|payment order does not belong to you|only pending payment orders can be closed` (`**/locallife/logic/payment_order_service.go`)，5 个结果

搜索正则表达式 `CreateRefundOrder\(|GetTotalRefundedByPaymentOrder|UpdateRefundOrderToFailed|RefundStatusProcessing` (`**/locallife/logic/refund_service.go`)，22 个结果

读取 [](file:///home/sam/locallife/locallife/db/sqlc/payment_order.sql.go#820-820)，行 820 到 926

读取 [](file:///home/sam/locallife/locallife/db/sqlc/wechat_notification.sql.go#1-1)，行 1 到 120

搜索正则表达式 `UNIQUE|wechat_notification|create table wechat_notifications|PRIMARY KEY \(id\)` (`**/locallife/db/migration/**/*.sql`)，178 个结果

读取 [](file:///home/sam/locallife/locallife/db/migration/000016_add_wechat_notifications.up.sql)

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#900-900)，行 900 到 1110

读取 [](file:///home/sam/locallife/locallife/api/payment_callback_test.go#1-1)，行 1 到 360

搜索正则表达式 `refund_orders|UNIQUE|payment_order_id` (`**/locallife/db/migration/000011_add_payment_orders.up.sql`)，30 个结果

搜索正则表达式 `func \(processor \*RedisTaskProcessor\) ProcessTaskInitiateRefund|ProcessTaskRefundResult|CreateRefundOrder\(|GetRefundOrderByOutRefundNo|DistributeTaskProcessRefund` (`**/locallife/worker/task_process_payment.go`)，10 个结果

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#1290-1290)，行 1290 到 1455

读取 [](file:///home/sam/locallife/locallife/worker/task_combined_payment_timeout.go)

读取 [](file:///home/sam/locallife/locallife/worker/task_reservation_timeout.go)

接下来我补查“支付单创建与超时任务投递”的一致性边界，确认是否存在“创建成功但未投递补偿任务”这类上线隐患。

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service_test.go)

读取 [](file:///home/sam/locallife/locallife/api/payment_order.go#1-1)，行 1 到 380

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service.go#1-1)，行 1 到 260

Ran terminal command:  cd /home/sam/locallife/locallife && go test ./api -run 'Payment|Refund|Combined|Callback' -count=1
Completed with input:  cd /home/sam/locallife/locallife && go test ./api -run 'Payment|Refund|Combined|Callback' -count=1

读取 [](file:///home/sam/.config/Code/User/workspaceStorage/daccee4eb373b9faf9120a9da7f9f219/GitHub.copilot-chat/chat-session-resources/46ec8c9e-8100-4a20-aa6e-0e9bbb163bb7/call_5qMHus1QQNW1B4ctr20PZ8A8__vscode-1774148769994/content.txt#1-1)，行 1 到 120

读取 [](file:///home/sam/.config/Code/User/workspaceStorage/daccee4eb373b9faf9120a9da7f9f219/GitHub.copilot-chat/chat-session-resources/46ec8c9e-8100-4a20-aa6e-0e9bbb163bb7/call_5qMHus1QQNW1B4ctr20PZ8A8__vscode-1774148769994/content.txt#288-288)，行 288 到 288

Ran terminal command:  go test ./api -run 'Payment|Refund|Combined|Callback' -count=1 2>&1 | tail -n 40

Ran terminal command:  go test ./api -run TestHandlePaymentNotifyFullFlow -count=1
Completed with input:  go test ./api -run TestHandlePaymentNotifyFullFlow -count=1

Ran terminal command:  go test ./logic -run 'Payment|Refund|Combined' -count=1 2>&1 | tail -n 40

Ran terminal command:  go test ./worker -run 'Payment|Refund|Reconciliation|Timeout|Combined' -count=1 2>&1 | tail -n 60

搜索正则表达式 `UpdatePaymentOrderToClosed\(|CancelOrderTx\(|payment order is not pending, skip timeout processing` (`**/locallife/worker/task_payment_timeout.go`)，3 个结果

搜索正则表达式 `UpdateCombinedPaymentOrderToClosed\(|combined payment order is not pending, skip timeout processing|UpdatePaymentOrderToClosed\(` (`**/locallife/worker/task_combined_payment_timeout.go`)，3 个结果

搜索正则表达式 `CreateCombinedPaymentTx\(|CreateCombineOrder\(|UpdateCombinedPaymentOrderPrepay\(|create combined payment: %w` (`**/locallife/logic/combined_payment_service.go`)，4 个结果

搜索正则表达式 `if err := svc.paymentClient.CloseOrder\(|_ = svc.paymentClient.CloseOrder\(|UpdatePaymentOrderToClosed\(` (`**/locallife/logic/payment_order_service.go`)，3 个结果

搜索正则表达式 `DistributeTaskPaymentOrderTimeout\(|DistributeTaskCombinedPaymentOrderTimeout\(` (`**/locallife/api/payment_order.go`)，2 个结果

搜索正则表达式 `failed to record notification|CheckNotificationExists\(|CreateWechatNotification\(|UpdatePaymentOrderToPaid\(|payment amount mismatch|DistributeTaskProcessPaymentSuccess\(|task enqueue failed|payment order already paid` (`**/locallife/api/payment_callback.go`)，27 个结果

搜索正则表达式 `暂时略过排序|死锁|GetOrderForUpdate` (`**/locallife/db/sqlc/tx_create_combined_payment.go`)，5 个结果

搜索正则表达式 `CloseExpiredPaymentOrders|combined payment|CloseExpiredCombined|combined_payment_orders|UpdateCombinedPaymentOrderToClosed` (`**/locallife/scheduler/**/*.go`)，1 个结果

我已经完成主链路源码核查和测试现状验证，下面整理成正式审查结论：按严重级别给出可触发条件、后果和修复优先级，并明确你现在是否能直接上线。结论：现在不建议直接上线。  
从支付主链路审查结果看，存在会影响资金一致性与补偿可靠性的高风险缺陷，至少要先修完 Critical 和 High 项再发版。

**Critical**
1. 回调幂等存在并发窗口，检查与落库非原子
- 位置: payment_callback.go, payment_callback.go, 000016_add_wechat_notifications.up.sql
- 问题: 先 CheckNotificationExists，再执行业务更新，最后才 CreateWechatNotification。并发重复通知时，两个请求都可能通过前置检查并执行业务逻辑。
- 触发: 微信重试、网关重放、双实例并发处理同一 notification id。
- 后果: 重复状态迁移、重复入队、重复告警，资金链路出现不可预期分叉。
- 备注: 表主键能拦住“重复插入通知记录”，但拦不住“重复执行业务逻辑”。

2. 支付超时任务不是原子流程，失败重试会被短路，导致订单与支付状态永久不一致
- 位置: task_payment_timeout.go, task_payment_timeout.go, task_payment_timeout.go
- 问题: 先把 payment_order 关为 closed，再取消业务订单。若取消订单失败，任务重试时因为“已非 pending”直接跳过。
- 后果: payment_order 已关闭，但业务 order 仍 pending，形成卡单；后续补偿链路难自动修复。

3. 合单超时任务同类问题，先关合单再逐个关子单，失败后重试同样被短路
- 位置: task_combined_payment_timeout.go, task_combined_payment_timeout.go, task_combined_payment_timeout.go
- 问题: 状态更新顺序导致部分失败后不可重入完成。
- 后果: combined_payment_order 与子 payment_orders 状态漂移，后续回调/人工处理复杂度高。

**High**
1. 金额不一致分支仅标失败+告警，不自动退款
- 位置: payment_callback.go, payment_callback.go
- 问题: 检测到实收金额不符后返回 SUCCESS，未触发自动退款。
- 后果: 用户扣款但业务未完成，强依赖人工处理，财务投诉风险高。

2. 退款创建存在并发超退窗口（检查累计退款与创建退款非事务化）
- 位置: refund_service.go, refund_service.go, 000011_add_payment_orders.up.sql
- 问题: 并发请求都可能读到同一 alreadyRefunded，再各自创建退款单；表层没有防超退唯一约束。
- 后果: 可能产生超额退款请求或脏退款记录，增加清算与对账压力。

3. 用户主动关单时忽略微信关单失败
- 位置: payment_order_service.go, payment_order_service.go
- 问题: 先本地 closed，再调用微信 CloseOrder，且错误被吞掉。
- 后果: 本地已关闭但微信侧未关闭，可能继续支付并触发“关单后支付”的异常分支。

4. 创建支付单后，超时任务投递失败被忽略
- 位置: payment_order.go, payment_order.go
- 问题: timeout task 分发错误未处理。
- 后果: 依赖其他清理机制兜底，时效与一致性不可控，尤其合单侧风险更高。

5. 合单创建流程中，先写本地事务再调微信；外部失败时会留下待支付本地痕迹
- 位置: combined_payment_service.go, combined_payment_service.go, combined_payment_service.go
- 问题: 外部调用失败后，已有本地 combined/payment 子单。
- 后果: 需要额外补偿/回收逻辑，否则会积累“僵尸待支付单”。

6. 合单事务内多订单加锁未排序，代码注释已提示潜在死锁
- 位置: tx_create_combined_payment.go, tx_create_combined_payment.go
- 问题: 对多个订单 GetOrderForUpdate 未统一排序。
- 后果: 并发创建合单时，死锁概率上升，形成偶发高延迟和失败尖峰。

**测试与发布信号**
1. 现有回调测试覆盖了“简单幂等分支”，但缺少并发重放/竞态用例
- 位置: payment_callback_test.go
2. 我实测支付相关 API 测试当前有失败（不是鲁棒性增强，而是当前行为与测试期望已偏离）
- 失败点: TestHandlePaymentNotifyFullFlow 的“金额不匹配”场景，触发了 UpdatePaymentOrderToFailed 的未预期调用（定位在 payment_callback.go）
3. worker 包支付关键任务几乎无针对性测试（命令输出为 no tests to run）

**是否可以直接上线**
不可以。  
以生产级鲁棒性标准看，当前版本在“资金一致性、补偿可重入、异常自动化处置”三个维度都未达标。

**建议的最小上线门槛（先做这 6 件）**
1. 回调幂等改为原子占位（先写入通知占位/去重键成功后再执行业务），失败必须可重试。
2. 超时任务改成单事务状态机推进，确保失败可重入，不会因前置状态变化被短路。
3. 金额不匹配改为自动退款+强告警，不走纯人工兜底。
4. 退款创建加事务锁或数据库约束，消除并发超退窗口。
5. 关单链路改为“先尝试微信关单并校验结果，再落本地状态”或引入可靠补偿任务。
6. 补齐并发与故障注入测试（重复回调、回调与超时竞态、任务入队失败、退款并发）。

如果你愿意，我可以下一步直接给你一份“可落地修复清单”，按 P0/P1 拆到具体函数和 SQL 改法，方便你分配开发。


正在优化工具选择...我会先收集支付相关实现的入口、核心流程和测试覆盖面，再按生产级鲁棒性的标准做审查，重点找会导致资金错误、重复扣款、状态不一致和补偿失败的问题。

读取内存

先建立审查范围和清单，接下来会定位所有支付、退款、结算、回调、幂等等相关代码与测试。

Created 5 todos

这一轮先把支付域的代码面找全，结果会用来确定审查边界，避免漏掉隐藏分支。

搜索正则表达式 `支付|付款|pay|payment|refund|退款|settle|结算|charge|prepay|wechat pay|wxpay|微信支付|回调|notify` (`locallife` · `locallife/**/*.go`)，124 个结果

搜索与 `locallife` · `locallife/**/*pay*.go` 匹配的文件，28 匹配项

搜索与 `locallife` · `locallife/**/*refund*.go` 匹配的文件，10 匹配项

搜索与 `locallife` · `locallife/**/*bill*.go` 匹配的文件，4 匹配项

代码面已经收敛到订单支付、合单支付、会员支付、退款、回调、超时关闭、恢复任务和对账几个核心模块。下一步我会读主流程和事务边界，判断是否存在资金一致性风险。

梳理支付链路

已在 代码库 中搜索 "支付下单支付入口payment API微信支付checkout"、8 结果

搜索正则表达式 `func.*[Pp]ay|func.*[Oo]rder.*[Pp]ay|func.*[Cc]heckout` (`locallife` · `locallife/**/*.go`)，19 个结果

搜索匹配 `locallife` · `locallife/**/*pay* -type f` 的文件，无匹配项

搜索匹配 `locallife` · `locallife/**/*refund* -type f` 的文件，无匹配项

搜索正则表达式 `退款|refund|ApplyRefund|HandleRefund|RefundCallback` (`locallife` · `locallife/**/*.go`)，13 个结果

搜索正则表达式 `callback|Callback|notify|Notify|PaymentNotification` (`locallife` · `locallife/**/*.go`)，6 个结果

搜索匹配 `locallife` · `locallife/**/*callback* -type f` 的文件，无匹配项

搜索匹配 `locallife` · `locallife/**/*notify* -type f` 的文件，无匹配项

搜索正则表达式 `分账|profit_sharing|ProfitSharing` (`locallife` · `locallife/**/*.go`)，20 个结果

搜索正则表达式 `settlement|reconciliation|对账|结算` (`locallife` · `locallife/**/*.go`)，19 个结果

搜索正则表达式 `payment.*webhook|wechat.*notify|handlePayment|HandleWechatPay` (`locallife` · `locallife/api/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/api)

搜索正则表达式 `createRefund|applyRefund|RefundApply|RefundRequest` (`locallife` · `locallife/**/*.go`)，16 个结果

搜索正则表达式 `refund|RefundRecovery|compensation|recover` (`locallife` · `locallife/worker/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/worker)

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#1-1)，行 1 到 120

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service.go#1-1)，行 1 到 100

搜索正则表达式 `payment|Payment|refund|Refund` (`locallife` · `locallife/**/*test.go`)，无结果

读取 [](file:///home/sam/locallife/locallife/logic/combined_payment_service.go#1-1)，行 1 到 120

搜索与 `locallife` · `locallife/**/*payment*test*.go` 匹配的文件，9 匹配项

搜索正则表达式 `ProcessPayment|payment.*state|order.*state|pending.*paid` (`locallife` · `locallife/**/*.go`)，17 个结果

读取 [](file:///home/sam/locallife/locallife/worker/refund_recovery_scheduler.go#1-1)，行 1 到 80

读取 [](file:///home/sam/locallife/locallife/worker/profit_sharing_recovery_scheduler.go#1-1)，行 1 到 80

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#100-100)，行 100 到 250

搜索正则表达式 `TaskProcessPaymentSuccess|TaskPaymentTimeout|TaskCombinedPaymentTimeout|DistributeTask` (`locallife` · `locallife/**/*.go`)，20 个结果

搜索正则表达式 `func.*order.*payment|reservation.*payment|takeout.*payment` (`locallife` · `locallife/api/**/*.go`)，17 个结果

搜索正则表达式 `membership|member.*pay` (`locallife` · `locallife/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/worker/payment_recovery_scheduler.go#1-1)，行 1 到 50

搜索正则表达式 `ProcessPaymentSuccess|refund_order|profit_sharing|payment_order` (`locallife` · `locallife/db/sqlc/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/db/sqlc)

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#1-1)，行 1 到 150

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_payment_success.go#1-1)，行 1 到 100

搜索正则表达式 `order.*refund|refund.*apply|claim.*refund` (`locallife` · `locallife/logic/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/db/sqlc/refund_order.sql.go#1-1)，行 1 到 100

读取 [](file:///home/sam/locallife/locallife/db/sqlc/profit_sharing_order.sql.go#1-1)，行 1 到 100

搜索正则表达式 `outstanding|pending.*refund|failed.*refund|status.*check` (`locallife` · `locallife/worker/**/*.go`)，20 个结果

读取 [](file:///home/sam/locallife/locallife/api/payment_order.go#1-1)，行 1 到 100

读取 [](file:///home/sam/locallife/locallife/logic/order_service.go#1-1)，行 1 到 80

这一轮会同时读取探索结果和关键实现文件，目的是把“资金状态怎么流转、失败怎么补偿、重复通知怎么处理”这三件事落到代码细节上。

读取 [](file:///home/sam/locallife/locallife/api/payment_order.go#1-1)，行 1 到 260

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#1-1)，行 1 到 320

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service.go#1-1)，行 1 到 360

读取 [](file:///home/sam/.config/Code/User/workspaceStorage/daccee4eb373b9faf9120a9da7f9f219/GitHub.copilot-chat/chat-session-resources/b751ff12-2054-4a12-9c77-e78aa9dbb602/call_HOQ1h742fnQjVwi8fFFSa89G__vscode-1774148769824/content.txt#1-1)，行 1 到 260

读取 [](file:///home/sam/locallife/locallife/logic/order_payment.go)

已经看到几个关键模式：支付回调先改状态、后记通知ID、再投递异步任务，这类顺序很容易暴露一致性缺口。现在继续看事务代码和退款执行代码，确认这些缺口是否真的被兜住。

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#321-321)，行 321 到 520

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_payment_success.go)

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#1-1)，行 1 到 360

读取 [](file:///home/sam/locallife/locallife/worker/payment_recovery_scheduler.go)

读取 [](file:///home/sam/locallife/locallife/worker/refund_recovery_scheduler.go)

读取 [](file:///home/sam/locallife/locallife/logic/combined_payment_service.go#1-1)，行 1 到 320

接下来直接定位支付成功、退款发起、退款结果处理和合单回调的函数实现，判断幂等、事务边界和补偿是否闭环。

搜索正则表达式 `func \(processor \*RedisTaskProcessor\) ProcessTaskPaymentSuccess|func \(processor \*RedisTaskProcessor\) ProcessTaskProcessRefund|func \(processor \*RedisTaskProcessor\) ProcessTaskRefund|func \(processor \*RedisTaskProcessor\) ProcessTaskProcessRefundResult|func \(processor \*RedisTaskProcessor\) ProcessTaskProfitSharing|func \(server \*Server\) handleCombinedPaymentNotify` (`locallife` · `locallife/{worker,api}/**/*.go`)，5 个结果

搜索文本 `CreateCombinedPaymentTx|ListPaidUnrefundedPaymentOrders|CheckNotificationExists|CreateWechatNotification|UpdatePaymentOrderToPaid|UpdatePaymentOrderToFailed|UpdatePaymentOrderProcessedAt|GetPaymentOrderForUpdate` (`locallife` · `locallife/db/sqlc/**/*.go`)，100 个结果

搜索文本 `membership_recharge|reservation_addon|rider_deposit|profit_sharing` (`locallife` · `locallife/{logic,api,worker,db/sqlc}/**/*.go`)，120 个结果

这一轮读取实际执行函数和底层 SQL 语句，重点验证是否靠数据库原子性兜底，还是只靠代码约定。

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#425-425)，行 425 到 840

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment.go#885-885)，行 885 到 1560

读取 [](file:///home/sam/locallife/locallife/db/sqlc/tx_create_combined_payment.go)

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#1380-1380)，行 1380 到 1565

读取 [](file:///home/sam/locallife/locallife/db/sqlc/payment_order.sql.go#521-521)，行 521 到 820

读取 [](file:///home/sam/locallife/locallife/db/sqlc/wechat_notification.sql.go#1-1)，行 1 到 120

目前高风险问题已经集中在 4 类：回调幂等原子性、退款幂等键设计、合单支付状态机、恢复任务误扫。最后再补测试和关键 SQL 细节，然后给你是否可直接上线的结论。

搜索文本 `CheckNotificationExists|CreateWechatNotification|UpdatePaymentOrderToPaid|DistributeTaskProcessPaymentSuccess|DistributeTaskProcessRefund|UpdatePaymentOrderToFailed` (`**/locallife/api/payment_callback.go`)，33 个结果

搜索正则表达式 `func \(processor \*RedisTaskProcessor\) ProcessTaskInitiateRefund|outRefundNo := fmt\.Sprintf\("RF%d%d"|GetRefundOrderByOutRefundNo|CreateRefundOrder\(|UpdatePaymentOrderToRefunded|UpdateRefundOrderToSuccess|UpdateRefundOrderToFailed` (`**/locallife/worker/task_process_payment.go`)，30 个结果

搜索文本 `ListPaidUnrefundedPaymentOrders|po.status = 'paid'|o.status = 'cancelled'|GetLatestPaymentOrderByOrder|UpdatePaymentOrderToClosed|PaymentType:   "miniprogram"` (`**/locallife/db/sqlc/**/*.go`)，30 个结果

搜索文本 `ProcessTaskPaymentSuccess|UpdatePaymentOrderProcessedAt|unknown business type|return nil` (`**/locallife/db/sqlc/tx_payment_success.go|locallife/worker/task_process_payment.go`)，无结果

搜索文本 `payment_callback|ProcessTaskPaymentSuccess|ProcessTaskInitiateRefund|combined payment|profit sharing|settlement` (`**/locallife/**/*test.go`)，47 个结果

我还要确认测试到底覆盖了哪些失败分支，尤其是“微信重试 + 本地半成功”这种最危险的场景。

读取 [](file:///home/sam/locallife/locallife/api/payment_callback_test.go#1-1)，行 1 到 320

读取 [](file:///home/sam/locallife/locallife/worker/task_process_payment_test.go#1-1)，行 1 到 360

读取 [](file:///home/sam/locallife/locallife/logic/combined_payment_service_test.go#1-1)，行 1 到 260

读取 [](file:///home/sam/locallife/locallife/logic/payment_order_service_test.go#1-1)，行 1 到 280

读取 [](file:///home/sam/locallife/locallife/integration/takeout_journey_integration_test.go#580-580)，行 580 到 690

最后补一轮精确定位测试覆盖点，确认哪些关键失败场景完全没被验证。

搜索正则表达式 `TestHandlePaymentNotify|TestHandleRefundNotify|TestHandleCombinePaymentNotify|TestHandleEcommerceRefundNotify|TestHandleOrderSettlementNotify` (`**/locallife/api/payment_callback_test.go`)，10 个结果

搜索正则表达式 `ProcessTaskPaymentSuccess\(|ProcessTaskInitiateRefund\(|ProcessTaskRefundResult\(|ProcessTaskProfitSharing\(` (`**/locallife/**/*test.go`)，4 个结果

搜索文本 `amount mismatch|closed/failed order|enqueue failed|duplicate notification|settlement_time empty|refund recovery|profit sharing return` (`**/locallife/**/*test.go`)，1 个结果

我已经定位到回调测试入口，但还要看它是否覆盖“本地状态已变更但通知ID没落库”这种真正的生产事故场景。

读取 [](file:///home/sam/locallife/locallife/api/payment_callback_test.go#416-416)，行 416 到 760

我还缺合单支付成功回调这一块的执行顺序，因为它决定你多商户订单是否会出现部分支付单落地、部分没处理的状态漂移。

读取 [](file:///home/sam/locallife/locallife/api/payment_callback.go#900-900)，行 900 到 1095

读取 [](file:///home/sam/locallife/locallife/db/sqlc/payment_order.sql.go#92-92)，行 92 到 140

读取 [](file:///home/sam/locallife/locallife/logic/membership_payment.go)

读取 [](file:///home/sam/locallife/locallife/logic/refund_service.go#1-1)，行 1 到 260

我补到两个更实质的问题了：合单创建失败后会留下悬空待支付记录；单笔支付在微信下单成功但本地写 prepay_id 失败时也会留下半成功状态。这两类都属于典型的生产级资金状态机缺口。

搜索文本 `CreateJSAPIOrder|UpdatePaymentOrderPrepayId|UpdatePaymentOrderToClosed` (`**/locallife/logic/payment_order_service.go`)，5 个结果

搜索正则表达式 `CreateCombinedPaymentTx|CreateCombineOrder|UpdateCombinedPaymentOrderPrepay|return result, fmt.Errorf\("create combine order|return result, fmt.Errorf\("update combined payment prepay` (`**/locallife/logic/combined_payment_service.go`)，8 个结果

搜索正则表达式 `UpdatePaymentOrderToClosed\(ctx, existingPO.ID\)|CreateCombinedPaymentOrder\(|SetPaymentOrderCombinedID|CreateCombinedPaymentSubOrder` (`**/locallife/db/sqlc/tx_create_combined_payment.go`)，6 个结果

搜索文本 `GetCombinedPaymentSubOrderByOutTradeNo|GetLatestPaymentOrderByOrder|PaymentType != "profit_sharing"` (`**/locallife/api/payment_callback.go`)，4 个结果

我又确认到一个更深的设计冲突：业务模型允许同一支付单多次部分退款，但异步退款任务却把退款单号固定成 payment_order_id + order_id，这会把多次退款压成同一个外部退款单。这个上线会直接带来退款金额错误。

搜索正则表达式 `GetTotalRefundedByPaymentOrder|alreadyRefunded\+input\.RefundAmount|OutRefundNo\(s\.clock.Now\(\)\)` (`**/locallife/logic/refund_service.go`)，3 个结果

搜索正则表达式 `outRefundNo := fmt.Sprintf\("RF%d%d"|refund order already exists, reusing` (`**/locallife/worker/task_process_payment.go`)，2 个结果

搜索文本 `CreateCombineOrderError|update combined payment prepay` (`**/locallife/logic/combined_payment_service_test.go`)，2 个结果

搜索文本 `payment success task enqueue failed|TASK_ENQUEUE_FAILURE|manual intervention may be required` (`**/locallife/api/payment_callback.go`)，4 个结果

我会跑一组聚焦支付域的现有测试，确认现状至少不是“测试全红”，同时也能判断哪些风险没有被测试覆盖。

78/80 个测试已通过(97.5%)

现有支付测试并不是全绿。至少有 2 个回调测试已失败，而且失败原因是代码路径新增了状态变更 UpdatePaymentOrderToFailed，但测试没有覆盖或未同步，这本身也说明支付回调的行为和测试基线已经出现漂移。

Created 5 todos

**结论**
不建议直接上线。

按生产级鲁棒性的标准看，这套后端支付链路还没有达到“可以放心带真实资金流量”的程度。核心问题不是普通业务错误，而是几类会直接造成资金状态漂移、退款漏执行、分账错单的系统性缺陷。

**主要发现**
1. Critical: 已关闭或失败的支付单收到微信成功回调时，代码先把本地支付单改成已支付，再异步发起退款，而且退款任务入队失败不会阻断成功响应。
见 payment_callback.go、payment_callback.go、payment_callback.go。
问题在于，这条链路一旦在改状态之后、退款任务真正落队之前崩溃或入队失败，本地就会永久停留在“已支付但未退款”。而补偿任务并不会兜住这类单，因为退款恢复只扫描“订单已取消且支付单仍为已支付”的普通订单支付记录，见 payment_order.sql.go 和 refund_recovery_scheduler.go。这意味着“超时关单后用户又支付成功”的真实生产事故，当前实现会漏退款。

2. Critical: 异步退款任务把外部退款单号固定成 payment_order_id + order_id，和系统本身支持的“多次部分退款”模型冲突。
业务服务明确支持累计退款金额校验，并为每次退款生成新的退款单号，见 refund_service.go、refund_service.go、refund_service.go。
但异步退款执行路径却把退款单号写死，并在命中同一单号时直接复用旧退款记录，见 task_process_payment.go 和 task_process_payment.go。
后果是，同一支付单如果发生第二次部分退款，请求可能被错误折叠到第一次退款记录上，最终出现退款金额不对、状态不对、外部微信退款单不一致的问题。这是直接的资金风险。

3. Critical: 合单支付创建是“本地事务先提交，远端微信后调用”，失败后没有补偿，且会把旧待支付单只在本地关闭、不关闭微信侧订单。
事务里会先关闭旧的待支付单，见 tx_create_combined_payment.go，再创建新的合单记录和子支付单，见 tx_create_combined_payment.go、tx_create_combined_payment.go、tx_create_combined_payment.go。
之后才调用微信合单下单，见 combined_payment_service.go。如果这里失败，或者后续写 prepay_id 失败，代码直接返回错误，见 combined_payment_service.go 和 combined_payment_service.go。
这会留下两类生产事故：
一类是本地已经有新的待支付合单和子单，但用户拿不到可支付参数，订单进入悬空状态。
另一类是旧单只在数据库里被标记关闭，但微信侧原订单并未关闭，用户仍可能支付旧单，造成双轨支付。

4. High: 微信结算事件先精确找到子单，但随后又退化成“按订单取最新支付单”，分账目标可能错单。
结算回调先按子单 out_trade_no 找到合单子单，见 payment_callback.go，这是对的。
但后面却不是继续沿着这个子单对应的支付单处理，而是重新按 order_id 取“最新一条支付单”，见 payment_callback.go，其 SQL 只是按 created_at 倒序取最新，见 payment_order.sql.go。
只要一个订单存在多次支付尝试，结算事件就可能对应错支付单，导致该分账本该触发却被跳过，或者落到错误的 payment_order 上。

5. High: 单笔支付创建也有明显的半成功窗口，微信下单成功后本地写 prepay_id 失败，没有补偿。
支付单创建流程先在本地建支付单，再调微信创建 JSAPI 订单，见 payment_order_service.go。
如果微信返回成功，但本地更新 prepay_id 失败，代码直接报错返回，见 payment_order_service.go。
此时远端微信订单已经存在，本地却没有完整支付参数，也没有显式关单或恢复逻辑。用户端会看到“创建支付失败”，但真实世界里这个单可能仍可支付，属于典型的状态分叉。

6. High: 回调幂等不是原子事务，多个关键分支都存在“先改业务状态，后记录通知”的窗口。
以支付回调为例，顺序是先查通知是否存在，见 payment_callback.go，再更新支付状态，见 payment_callback.go，再记录通知 ID，见 payment_callback.go，最后投递后续任务，见 payment_callback.go。
这不是数据库原子幂等。只要在中间崩溃，就可能出现“业务状态已经变化，但通知记录缺失”的半处理状态。普通支付还有一层 paid + processed_at 的恢复机制，但关闭单自动退款、合单、结算等分支并没有同等强度的兜底。

**测试与覆盖**
现有支付域测试并不稳定，且覆盖重点不对。

我跑了聚焦支付域的现有测试，结果是 78 通过，2 失败。失败都在 payment_callback_test.go 这组支付回调测试里，实际失败点落在 payment_callback.go。
这说明至少两个事实：
1. 支付回调行为已经和测试预期发生漂移。
2. 测试更多覆盖的是幂等 happy path，而不是最危险的半成功、补偿失败、重入重放场景。

我没有看到足够扎实的自动化覆盖来证明下面这些事故路径已被验证：
1. 关闭单晚到支付后的自动退款补偿闭环
2. 合单创建成功一半失败一半时的清理
3. 同一支付单多次部分退款
4. 结算事件对应历史多支付单时的精确归属
5. 微信成功、本地落库失败后的恢复

**上线判断**
如果你问的是“现在直接上线是否可以”，我的判断是：不可以。

不是“建议再优化一下”的程度，而是当前实现里已经存在几条明确的资金风险路径。只要流量一上来，碰到网络抖动、微信回调重试、任务队列短暂不可用、用户反复发起支付/退款，这些问题就会从代码层面的隐患变成实际账务事故。

**建议的最小上线门槛**
1. 把“关闭单收到支付”改成可恢复的本地事务状态机，不允许出现“已收款但退款任务丢失”。
2. 重做退款幂等键设计，区分“同一次退款的重试”和“同一支付单的多次退款”。
3. 给合单支付补上失败补偿，至少要清理本地悬空记录，并显式关闭旧的微信订单。
4. 让结算事件绑定到精确 payment_order，而不是按订单取最新支付单。
5. 为回调、退款、合单创建补齐故障注入测试，重点测“中途崩溃后恢复”。

如果你要，我下一步可以直接继续做两件事中的一个：
1. 按优先级给你出一份支付链路整改清单，分成必须上线前修复和可延期项
2. 直接开始修最危险的前三个问题，并补对应测试