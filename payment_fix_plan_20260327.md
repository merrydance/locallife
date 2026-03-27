# 支付问题修复计划与影响面拆分

基于 [payment_review_report_20260327.md](payment_review_report_20260327.md) 的 findings，本文给出后续修复的执行顺序、影响面拆分、验证要求和回滚边界。

## 当前进度

- [x] 阶段 1A：新增用户支付账单聚合接口，钱包页切换到真实支付/退款流水
- [x] 阶段 1B：统一小程序 payment.ts 实现，payment-refund.ts 收敛为兼容层
- [x] 阶段 2A：提现失败告警改为独立的 WITHDRAW_FAILED 类型
- [x] 阶段 2B：运营商账户余额接口改为 `200 + account_status/status_desc` 空态语义，Web 财务页区分未配置与接口失败
- [x] 验证与回归：已完成 Swagger 生成、Go 定向测试、Web lint、改动文件级小程序 ESLint

### 验证结果

- 已执行：`cd locallife && make swagger`
- 已执行：`cd locallife && go test ./logic ./api ./worker -run 'TestPaymentLedgerServiceListPaymentLedger|TestListPaymentLedgerAPI|TestGetOperatorAccountBalanceAPI|TestGetOperatorAccountBalanceAPI_NotConfigured|TestProcessTaskMerchantWithdrawResult_FailedPublishesAlert'`
- 已执行：`cd web && npm run lint`
- 已执行：`cd weapp && npx eslint miniprogram/api/payment.ts miniprogram/api/payment-refund.ts miniprogram/pages/user_center/payment-detail/index.ts miniprogram/pages/user_center/refund-detail/index.ts miniprogram/pages/user_center/wallet/index.ts`
- 未通过但与本轮改动无关：`cd weapp && npm run lint` 仍被仓库内已有语法问题阻塞，涉及 `miniprogram/api/group-application.ts`、`miniprogram/pages/merchant/dishes/edit/index.ts`、`miniprogram/pages/register/operator/index.ts`、`miniprogram/utils/media.ts`

## 目标

1. 先修复用户可感知的资金展示错误，避免继续输出错误账单语义。
2. 收敛小程序支付 API 抽象，消除错误路由和旧契约模型的继续扩散。
3. 最后补齐提现告警与运营端状态表达，提高运维可观测性。

## 设计原则

1. 不改变后端现有支付状态机语义，尤其不把 partial refund 强行映射成 payment order `refunded`。
2. 优先建立单一前端支付契约源，避免继续并存两套 API 模块。
3. 用户侧账单展示要基于真实资金事件，而不是从 payment order 状态反推。
4. 如果修改 db/query 或 sqlc 接口，必须同步执行 `make sqlc`。
5. 涉及“未开通/未配置”空态的接口，尽量对齐 API 契约规范中的 `200 + status` 语义，而不是继续扩大 4xx 分支。

## 建议分阶段执行

### 阶段 1：修复用户侧账单与小程序支付契约

目标：解决用户可见错误和最容易再次踩坑的前端契约漂移。

#### 1A. 修复钱包退款流水建模

推荐方案：新增“用户支付资金流水”聚合读取能力，由后端显式返回 payment event 和 refund event，再由钱包页直接消费。

不推荐方案：钱包页继续只拉 payment list，然后逐笔 N+1 查询 refund list 拼装。

不推荐原因：

- 性能差，列表越长请求数越多。
- 无法自然做统一分页。
- 容易把 payment 与 refund 的排序、时间字段、部分退款金额继续拼错。

推荐落地方式：

1. 后端新增一个面向当前用户的聚合列表接口。
2. 返回统一的账单项结构，至少包含：`entry_type`、`payment_order_id`、`refund_order_id`、`business_type`、`amount`、`status`、`occurred_at`、`title`。
3. payment entry 和 refund entry 分开建模，refund 一律使用真实 `refund_amount` 和 `refunded_at`。
4. partial refund 不再依赖 payment order 的 `status=refunded` 才展示。

推荐接口方向：

- 首选：新增 `GET /v1/payments/ledger`
- 备选：扩展现有 `GET /v1/payments` 返回 refund summaries

取舍说明：

- 首选方案更清晰，避免把“支付订单列表”和“资金流水列表”混成同一个响应结构。
- 备选方案改动面略小，但会把原本清晰的 payment resource 响应污染成半聚合结构，后续更难维护。

后端影响面：

- 可能新增或修改 [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql)
- 可能新增或修改 [locallife/db/query/refund_order.sql](locallife/db/query/refund_order.sql)
- 会触发生成代码更新：[locallife/db/sqlc](locallife/db/sqlc)
- 新增或扩展 logic 聚合服务，建议独立成 payment ledger service，而不是塞进 handler
- 需要在 [locallife/api/server.go](locallife/api/server.go#L959) 附近补路由
- 需要新增 API response struct，避免直接透出临时 map

小程序影响面：

- [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts#L71)
- 可能同步影响 [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts#L79)
- 需要新增或调整小程序账单数据类型，避免继续把 `PaymentOrder` 当流水项使用

测试与验证：

- 后端单测覆盖：全额退款、单次部分退款、多次部分退款、支付无退款、退款未完成
- 小程序验证：钱包首页首屏、分页加载、退款后刷新、部分退款金额展示
- 手工场景：一笔 100 元支付，先退 30，再退 20，再退 50，页面应展示 1 笔支付和 3 笔退款，而不是 1 笔反向支付

#### 1B. 收敛小程序支付 API 模块

推荐方案：以 [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts) 作为唯一主模块，把当前有效的退款与查询能力合并进去，然后逐步淘汰 [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts)。

这样选的原因：

- 下单确认、订单详情、预订支付等主支付路径已经主要依赖 `payment.ts`。
- 让“创建支付主链”继续留在现有主模块，迁移成本低于反向把所有页面切到 `payment-refund.ts`。

具体动作：

1. 修正 `payment.ts` 中错误的退款创建路由，统一使用 `POST /v1/refunds`。
2. 在 `payment.ts` 中补齐正确的 `getPaymentRefunds`、`getRefundById`、`getRefundReturns` 类型定义与调用形式。
3. 删除或废弃 `payment-refund.ts` 中的旧 `CreatePaymentParams`、`PaymentAdapter.buildPaymentParams`、`PaymentUtils.createWechatPayment/createAlipayPayment/createBalancePayment`。
4. 全量替换用户中心页面对 `payment-refund.ts` 的导入。
5. 统一把支付列表分页参数改成 `page_id`。

直接受影响文件：

- [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts#L205)
- [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts#L154)
- [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts#L337)
- [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts#L500)
- [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts#L1)
- [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts#L75)

风险控制：

- 第一版可以先保留 `payment-refund.ts`，但只做 re-export 或明显废弃注释，不再允许新增真实实现。
- 页面迁移完成后再删除旧模块，避免一步切换导致用户中心页面全量回归成本过高。

测试与验证：

- 小程序静态检查：`npm run lint`
- 用户中心页验证：支付详情、继续支付、退款列表、钱包账单
- 主支付页回归：堂食、外卖、预订支付调用不应受影响

### 阶段 2：修复运维与运营可观测性问题

目标：让提现故障可被正确归类，让运营端能区分“未配置”和“接口失败”。

#### 2A. 拆分提现失败告警类型

具体动作：

1. 在 [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go#L23) 的 `AlertType` 中新增提现专用类型，例如 `WITHDRAW_FAILED`。
2. 更新 [locallife/worker/task_merchant_withdraw_result.go](locallife/worker/task_merchant_withdraw_result.go#L116) 和 [locallife/worker/task_merchant_withdraw_result.go](locallife/worker/task_merchant_withdraw_result.go#L161) 的告警发出点。
3. 若平台有按 `alert_type` 聚合、筛选或展示的前端/消费侧，也要同步改。

验证：

- worker 单测覆盖“查询重试耗尽”和“微信提现失败”两类路径
- 告警 payload 中的 `alert_type` 应明确区分退款和提现

#### 2B. 修复运营商财务页的错误态表达

最小可行修复：

1. 前端显式区分 `payment config inactive`、账户接口失败、提现列表接口失败。
2. 只有在明确命中“未开通收付通”时，才显示锁定文案和禁用态。
3. 其余失败要展示重试或错误提示，不能直接伪装成“未开通”。

推荐增强：

1. 后端把“未配置收付通”改为 `200 + account_status/status_desc` 风格，和 API 契约规范保持一致。
2. 这样 Web 无需继续依赖 brittle 的 message 文本匹配。

影响面：

- 前端主文件：[web/src/app/operator/finance/page.tsx](web/src/app/operator/finance/page.tsx#L62)
- 后端可选增强点：[locallife/api/operator_finance.go](locallife/api/operator_finance.go#L139)

取舍说明：

- 如果只改前端，能快速止血，但仍依赖错误 message 字符串。
- 如果前后端一起改，契约更稳定，但需要同步改 Web 分支逻辑和接口响应结构。

验证：

- 未开通收付通时，页面显示明确引导，不误报系统故障
- 账户接口临时失败时，页面显示错误态，不把用户误导成“未配置”
- 提现记录接口失败时，账户区和记录区要能分开降级

## 文件级影响面拆分

### 后端高优先级改动候选

- [locallife/api/payment_order.go](locallife/api/payment_order.go#L59)
- [locallife/api/server.go](locallife/api/server.go#L959)
- [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql)
- [locallife/db/query/refund_order.sql](locallife/db/query/refund_order.sql)
- [locallife/db/sqlc/models.go](locallife/db/sqlc/models.go#L1305)
- [locallife/db/sqlc/models.go](locallife/db/sqlc/models.go#L1491)
- 新增或扩展 logic service，建议位于 locallife/logic 下 payment 相关文件

### 小程序高优先级改动候选

- [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts#L205)
- [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts#L154)
- [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts#L337)
- [weapp/miniprogram/api/payment-refund.ts](weapp/miniprogram/api/payment-refund.ts#L500)
- [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts#L71)
- [weapp/miniprogram/pages/user_center/payment-detail/index.ts](weapp/miniprogram/pages/user_center/payment-detail/index.ts#L79)

### 运维与运营侧改动候选

- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go#L23)
- [locallife/worker/task_merchant_withdraw_result.go](locallife/worker/task_merchant_withdraw_result.go#L116)
- [web/src/app/operator/finance/page.tsx](web/src/app/operator/finance/page.tsx#L62)
- [locallife/api/operator_finance.go](locallife/api/operator_finance.go#L139)

## 建议执行顺序

1. 先做阶段 1A 的接口设计确认，定下“账单聚合接口”还是“扩展现有支付列表”的最终方案。
2. 紧接着做阶段 1B，先统一小程序 API 模块，再迁移页面引用。
3. 阶段 1 合并完成后，再做钱包页与支付详情页的联调回归。
4. 最后处理阶段 2，把告警语义和运营端错误态一起补齐。

## 验证清单

### 后端

- 若改动 `db/query`：执行 `make sqlc`
- 运行最小相关单测，优先 payment/refund/worker 相关测试
- 新接口补 handler + logic + db 层单测，避免只在一层加覆盖

### 小程序

- 执行 `npm run lint`
- 验证继续支付、钱包账单、退款列表、按订单查支付详情

### Web

- 执行 `npm run lint`
- 验证运营商财务页在三种场景下的表现：未配置、接口失败、正常可提现

## 不建议的捷径

1. 不要把 partial refund 直接写回 payment order `status=refunded` 作为展示修复手段，这会破坏后端现有语义。
2. 不要继续增加第三套小程序 payment helper，应该先收敛现有两个模块。
3. 不要只靠前端字符串匹配补丁修掉所有运营商财务状态，至少要把错误态和未配置态分开。
4. 不要只修页面调用而保留错误 helper 原地导出，否则几周后会再次被误用。