# 支付业务链代码审查报告

审查日期：2026-03-27

审查范围：

- 后端支付主链：locallife/api、locallife/logic、locallife/db/sqlc、locallife/worker、locallife/wechat
- 小程序支付入口：weapp/miniprogram/api、weapp/miniprogram/pages
- Web 财务与分账入口：web/src/app/operator、web/src/app/platform

本次为静态代码审查，未复用既有 review 结论，也未执行集成环境联调。

## Findings

### 1. 钱包页用 payment_order.status 推断退款，导致部分退款长期不可见，财务流水展示失真

严重级别：高

证据：

- weapp/miniprogram/pages/user_center/wallet/index.ts:71-105 只调用 getPayments 获取 payment_orders，并用 `p.status === 'refunded'` 推断这是不是退款流水。
- locallife/api/payment_order.go:813-820 明确支持 `refund_type = full | partial`。
- locallife/logic/refund_service.go:49-67 与 locallife/worker/task_process_payment.go:91-108 只会在“累计退款额 >= 支付金额”时把 payment order 标记成 `refunded`。

影响：

- 部分退款发生后，钱包页仍会把这笔支付显示为一笔负向消费，不会出现对应退款流水。
- 如果一笔支付经历多次部分退款，用户直到“退满”为止都看不到任何正向退款记录；真正退满后，页面又会把整笔支付金额一次性显示成退款，时间和金额粒度都不真实。
- 这是直接影响用户侧资金认知的展示错误，虽然不改变真实账务，但会显著增加投诉和客服核对成本。

建议：

- 钱包账单不要再从 payment order 状态反推退款事件，而应显式查询 refund order 或服务端提供合并后的资金流水视图。
- 至少要把 partial refund 单独建模并展示实际 refund_amount 与 refunded_at。

### 2. 小程序主支付模块仍暴露不存在的退款创建路由，调用即失败

严重级别：中

证据：

- weapp/miniprogram/api/payment.ts:205-223 将退款创建请求发送到 `POST /v1/payments/${paymentId}/refunds`。
- locallife/api/server.go:959-976 只注册了 `GET /v1/payments/:id/refunds`，退款创建入口实际是 `POST /v1/refunds`。
- locallife/api/payment_order.go:813-857 的 handler 也确认创建退款的正式入口是 `createRefundOrder`。

影响：

- 任何新页面如果复用 payment.ts 的 `createRefund`，都会直接打到不存在的路由，得到 404/405。
- 现在仓库内已经存在另一套正确实现的退款 API，这说明前端支付模块已经发生契约分叉，后续继续迭代时很容易把错误接口重新接回线上路径。

建议：

- 删除或修正 payment.ts 中的 `createRefund`，避免仓库继续同时保留两套相互冲突的退款契约。
- 在小程序侧收敛为单一支付 API 模块，禁止新页面继续二选一接入。

### 3. 小程序支付列表查询参数仍使用 page，而后端只绑定 page_id，分页能力实际失效

严重级别：中

证据：

- weapp/miniprogram/api/payment-refund.ts:154-158 定义的是 `page` / `page_size`。
- weapp/miniprogram/api/payment-refund.ts:500-517 的 `getUserPaymentHistory` 继续把 `page` 传给 `/v1/payments`。
- weapp/miniprogram/pages/user_center/wallet/index.ts:71 与 weapp/miniprogram/pages/user_center/payment-detail/index.ts:74 都在调用 `getPayments({ page: 1, page_size: ... })`。
- locallife/api/payment_order.go:613-667 的 query binding 只接受 `page_id` / `page_size`。

影响：

- 任何尝试请求第 2 页及之后的调用，都会被后端当作默认 `page_id = 1` 处理，客户端看起来像“翻页成功”，实际始终拿第一页。
- 当前钱包页虽然写死第一页，但 helper 层已经把错误参数固化，后续一旦接入下拉加载或分页，问题会直接变成线上行为。

建议：

- 小程序统一改为 `page_id`。
- 对 `/v1/payments` 增加端到端前端契约测试，防止 query 参数再次漂移。

### 4. payment-refund.ts 仍保留旧版创建支付参数模型，任何新调用都会被后端拒绝

严重级别：中

证据：

- weapp/miniprogram/api/payment-refund.ts:67-72 的 `CreatePaymentParams` 仍是 `payment_method/amount/description` 结构。
- weapp/miniprogram/api/payment-refund.ts:337-352 的 `PaymentAdapter.buildPaymentParams` 继续生成这套旧字段。
- weapp/miniprogram/api/payment-refund.ts:462-489 的 `PaymentUtils.createWechatPayment/createAlipayPayment/createBalancePayment` 仍通过旧字段调用 `POST /v1/payments`。
- weapp/miniprogram/api/payment-refund.ts:122-127 与 locallife/api/payment_order.go 中当前后端契约都要求 `order_id/business_type/payment_type`。

影响：

- 这些 helper 目前虽然没有被实际页面调用，但它们仍是可导出的公共 API，未来任何开发者只要按名称直觉复用，就会稳定触发 400。
- 这说明小程序支付层已经同时维护了“现行契约”和“旧契约”两套模型，维护成本高，而且极易产生新的支付死链。

建议：

- 删除旧 helper，或全部改造成当前后端契约。
- 对 payment.ts 与 payment-refund.ts 做模块收敛，避免继续双轨并存。

### 5. 提现结果轮询把提现失败上报成 REFUND_FAILED，告警语义已漂移

严重级别：低

证据：

- locallife/worker/task_process_payment.go:23-31 的 AlertType 枚举没有提现专用告警类型。
- locallife/worker/task_merchant_withdraw_result.go:116-126 与 161-170 在“提现查询重试耗尽”和“提现失败”两种场景都发布 `AlertTypeRefundFailed`。

影响：

- 监控面板、告警路由和人工处置 runbook 都无法区分“退款故障”和“提现故障”。
- 当支付链路同时处理退款与提现时，值班人员会被错误标签误导，增加排障时间，也不利于后续按类型做稳定性统计。

建议：

- 增加独立的提现告警类型，例如 `WITHDRAW_FAILED`。
- 对现有告警消费侧同步更新分类、聚合与 runbook。

### 6. 运营商财务页吞掉了 payment config 专用错误，并把多种失败场景都伪装成“未开通”

严重级别：低

证据：

- web/src/app/operator/finance/page.tsx:74-88 识别出 `operator payment config is not active` 后，仅赋值给 `blockedByPaymentConfig`，随后又被 `const _ = blockedByPaymentConfig` 丢弃。
- 同文件 94 行把 `financeLocked` 直接设置成 `!balanceRes?.sub_mch_id`。
- locallife/api/operator_finance.go:139-167 表明后端确实会专门返回 `operator payment config is not active`。

影响：

- 前端没有把“未开通收付通”和“账户接口请求失败/权限异常/网关异常”区分开。
- 一旦余额接口因为临时故障返回失败，页面也会退化成“未开通，禁用提现”，真实故障被吞掉，运营同学无法判断是配置问题还是系统事故。

建议：

- 明确区分 `payment config inactive`、网关失败、权限失败三种状态。
- 不要用 `!balanceRes?.sub_mch_id` 作为通用兜底锁定条件。

## 正向结论

- 后端支付主链的分层基本清晰，支付创建、退款编排、回调处理、worker 补偿、scheduler 恢复路径已经形成完整闭环。
- 微信回调侧能看到较完整的幂等认领、商户归属校验、金额异常退款与失败释放认领逻辑，主链安全性明显好于前端契约层的现状。
- 收付通分账、退款回退、提现轮询均已有异步任务与恢复机制，说明后端核心支付状态机不是“只靠回调一次成功”的脆弱实现。

## 残余风险

- 本次没有跑集成测试，也没有在真实微信沙箱环境验证回调与补偿联动，因此报告重点是契约一致性、展示正确性与告警可运维性。
- 小程序目前同时存在 payment.ts 与 payment-refund.ts 两套支付抽象，这是当前支付前端层最主要的持续性风险源。

## 优先级建议

1. 先修钱包退款流水建模，避免继续向用户展示错误账单。
2. 再收敛小程序两套支付 API，删掉错误退款路由和旧版创建支付 helper。
3. 最后补齐提现告警分类和运营商财务页的错误态区分，提升值班与运维可观测性。