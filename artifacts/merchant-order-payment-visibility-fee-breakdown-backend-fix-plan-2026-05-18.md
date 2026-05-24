# 商户订单支付可见性与费用清单后端修复文档

**日期**：2026-05-18
**范围**：仅 `locallife/` 后端。`merchant_app/` 由其他团队修复，本计划不修改 Flutter 商户 App。
**风险等级**：G3。原因：涉及订单支付状态、商户可见性、支付后通知、分账/结算金额、支付通道费、商户财务 API 和异步 worker。
**目标**：商户只能在用户支付成功后看到和接收订单；商户收到的订单与财务数据必须按计算后的金额展示餐费、运费、平台服务费、支付通道费和商户实收；所有异常必须有日志，HTTP 调用方必须拿到语义明确的稳定提示。

**状态**：已完成（6/6 任务卡）
**完成时间**：2026-05-18
**验收摘要**：已完成商户未支付订单不可见、支付后通知 paid-only 防线、商户视角费用清单、订单 API/通知费用清单、商户财务费用口径和错误日志收口。后端生成物已同步；`make lint-filesize` 仍受仓库既有超大文件影响，作为存量 guardrail 债务记录，不在本次修复边界内。

---

## 当前审查结论

后端支付成功通知链路本身目前是支付成功后触发：

- `locallife/db/sqlc/tx_create_order.go` 创建普通订单时状态为 `pending`，余额全额支付才会在事务内进入已支付处理。
- `locallife/db/sqlc/tx_payment_success.go` 支付成功事务调用 `processOrderPaymentWithQueries`。
- `locallife/logic/payment_fact_application_service.go` 在支付事实处理成功后写 `order_payment_succeeded` outbox。
- `locallife/worker/task_payment_domain_outbox.go` 消费支付成功 outbox 后调用 `sendOrderPaidNotifications`。
- `locallife/worker/task_process_payment.go` 中 `sendOrderPaidNotifications` 再调用 `notifyMerchantNewOrder`。

“用户未支付时商户已收到/看到订单”的后端根因主要是商户查询面暴露了 `pending`：

- `locallife/logic/order_query_service.go` 的 `ListMerchantOrders` 在未传 `status` 时传空过滤条件。
- `locallife/db/query/order.sql` 的 `ListOrdersByMerchantWithFilters` / `CountOrdersByMerchantWithFilters` 在 `status` 为空时返回该商户全部状态，包括 `pending`。
- `locallife/logic/order_query_service.go` 的 `GetMerchantOrder` 只校验归属，不校验 `pending` 对商户不可见。

因此后端修复必须同时做两件事：关闭商户 API 的未支付订单可见性，并在支付后通知 worker 加防御性状态校验，防止未来任何旁路调用把未支付订单推给商户。

---

## 不可降级的不变量

1. 商户运营端订单列表、详情、接单、拒单、完成、打印状态等商户订单接口不得暴露 `pending` 订单。
2. `new_order` 通知只能在订单真实进入支付成功后的业务状态之后发送。
3. `notifyMerchantNewOrder` 收到非 `paid` 或非可商户处理状态的订单必须记录结构化错误日志并返回错误，让 outbox/task 进入失败或重试路径，不允许静默跳过。
4. JSON marshal、费用来源缺失、分账记录缺失、金额校验不一致、DB 查询失败、配置缺失等异常都必须落结构化日志；HTTP request 路径通过 `internalError` / `loggedServerError` 记录，worker 路径在做出失败/重试/终止决定处记录。
5. 前端可见错误必须稳定、语义明确，不透出 SQL、provider 原文、堆栈、商户号、合同号、身份证、银行卡等内部或敏感信息。
6. 商户费用字段只展示金额，单位为分；不展示比例、bps、平台/运营商拆分、宝付内部 30bps provider 成本。
7. 商户侧平台服务费展示为一个金额，即当前平台 2% + 运营商 3% 的合计 5% 结果；字段名用 `platform_service_fee_amount`。
8. 商户侧支付通道费展示为商户承担的 60bps 计算金额；字段名用 `payment_channel_fee_amount`。不得暴露 `provider_payment_fee`、`provider_payment_fee_rate_bps`、30bps、`platform_net_payment_fee_margin`。
9. 商户侧订单金额必须拆出餐费和运费：餐费、餐费优惠、运费、运费优惠、用户实付、平台服务费、支付通道费、商户实收。
10. 金额不一致不得用 0、空对象、隐藏字段或旧字段兜底。必须返回错误、记录日志，并让调用方看到明确提示，例如“订单费用明细暂不可用，请稍后重试”。

---

## 商户费用清单契约

后端新增商户视角费用清单，订单列表、订单详情、商户新订单通知和商户财务明细复用同一套字段语义。

示例结构如下，金额单位均为分，字段值必须是已计算金额：

```json
{
  "food_amount": 10000,
  "merchant_discount_amount": 300,
  "voucher_discount_amount": 200,
  "food_payable_amount": 9500,
  "delivery_fee_amount": 800,
  "delivery_fee_discount_amount": 0,
  "delivery_payable_amount": 800,
  "customer_payable_amount": 10300,
  "platform_service_fee_amount": 475,
  "payment_channel_fee_amount": 57,
  "merchant_receivable_amount": 8968
}
```

字段口径：

- `food_amount`：商品餐费原价，对应 `orders.subtotal`。
- `merchant_discount_amount`：商户优惠金额，对应现有 `orders.discount_amount`。
- `voucher_discount_amount`：用户券/平台券金额，对应 `orders.voucher_amount`，按现有订单金额语义确认是否计入商户结算基数。
- `food_payable_amount`：餐费应付金额，默认 `subtotal - discount_amount - voucher_amount`，不得小于 0；如现有结算已有更权威基数，以持久化分账记录为准。
- `delivery_fee_amount`：代取费原价，对应 `orders.delivery_fee`。
- `delivery_fee_discount_amount`：代取费优惠，对应 `orders.delivery_fee_discount`。
- `delivery_payable_amount`：用户实际承担代取费，默认 `delivery_fee - delivery_fee_discount`，不得小于 0。
- `customer_payable_amount`：用户订单实付，必须与 `orders.total_amount` 或支付单金额一致。
- `platform_service_fee_amount`：商户可见平台服务费总额，等于 `profit_sharing_orders.platform_commission + profit_sharing_orders.operator_commission`。不拆平台 2% 和运营商 3%。
- `payment_channel_fee_amount`：商户承担支付通道费金额。宝付 v2 使用 `profit_sharing_orders.merchant_payment_fee`；旧微信/收付通路径使用商户侧等价字段或补齐后的持久化字段。只展示金额，不展示 60bps。
- `merchant_receivable_amount`：商户实收金额，优先使用 `profit_sharing_orders.merchant_amount`；缺失时不得静默按旧字段猜测。

---

## 任务卡 1：关闭商户未支付订单可见性

**问题**：商户列表和详情能看到 `pending` 订单，导致商户 App 轮询时把未支付订单当作新订单处理。
**目标**：商户订单查询面默认和显式都不返回未支付订单；直接访问未支付订单详情时返回语义明确的状态冲突。
**边界**：只改 `locallife/` 后端。用户端订单列表仍可显示自己的 `pending` 支付单；后台内部、支付恢复、超时关闭任务仍可读取 `pending`，不改变内部查询。

**涉及文件**：

- 修改：`locallife/logic/order_query_service.go`
- 修改：`locallife/db/query/order.sql`
- 生成：`locallife/db/sqlc/order.sql.go`、`locallife/db/sqlc/querier.go`
- 测试：`locallife/logic/order_query_service_test.go`
- 测试：`locallife/db/sqlc/order_test.go`
- 测试：`locallife/api/order_test.go`

**改动步骤**：

1. 在 `logic.ListMerchantOrders` 入口校验 `input.Status`：
   - `status=pending` 返回 `logic.NewRequestError(http.StatusConflict, errors.New("订单尚未支付，暂不可处理"))`。
   - 空 status 继续允许，但后续 SQL 必须排除 `pending`。
2. 修改 `ListOrdersByMerchantWithFilters`：
   - 未传 status 时增加 `status <> 'pending'`。
   - 传非 pending status 时按显式状态过滤。
   - 保持 `order_type`、排序、分页不变。
3. 修改 `CountOrdersByMerchantWithFilters`，与列表查询保持完全相同的可见性条件。
4. 在 `logic.GetMerchantOrder` 完成归属校验后增加状态校验：
   - `order.Status == db.OrderStatusPending` 时返回 `409 Conflict`。
   - 前端提示文案固定为“订单尚未支付，暂不可处理”。
   - 不返回订单内容。
5. Handler 继续走 `writeLogicRequestError`，不要把 409 误转为 500。
6. 对 merchant-owned pending 详情访问记录结构化 `Warn` 或审计事件，字段至少包含 `merchant_id`、`order_id`、`status`、`user_id`。这是业务可判定状态冲突，不需要 5xx。

**错误与日志契约**：

- DB 查询失败：logic 返回 plain wrapped error，例如 `list merchant orders: %w`，handler 通过 `internalError` 记录 500。
- 显式请求 `status=pending`：409，message “订单尚未支付，暂不可处理”，记录一条业务拒绝日志。
- 详情命中 pending：409，message “订单尚未支付，暂不可处理”，记录一条业务拒绝日志。

**测试要求**：

- `ListMerchantOrders` 不传 status 时，mock 断言传入 SQL 参数或 DB 测试断言结果不包含 `pending`。
- `ListMerchantOrders` 传 `status=pending` 返回 409 request error。
- `GetMerchantOrder` 对 merchant-owned pending 返回 409，且不加载 items。
- API 测试覆盖 `/v1/merchant/orders?status=pending` 和 `/v1/merchant/orders/{id}` pending 场景，响应 message 稳定。

**验收命令**：

```bash
cd locallife
make sqlc
go test ./logic -run 'TestOrderService.*MerchantOrder'
go test ./db/sqlc -run 'Test.*Merchant.*Order'
go test ./api -run 'Test(ListMerchantOrdersAPI|GetMerchantOrderAPI)'
make check-generated
```

---

## 任务卡 2：支付后新订单通知增加 paid-only 防线

**问题**：当前通知入口由支付成功 outbox 调用，但 `notifyMerchantNewOrder` 自身没有状态防线；未来若被旁路调用，仍可能把非已支付订单推给商户。该函数还存在 JSON marshal 错误被 `_` 忽略的降级风险。
**目标**：通知入口自身保证只推送支付后订单；任何序列化、快照构建、发送失败都记录或传播，不允许静默降级。
**边界**：不修改商户 App 的消息去重、轮询、语音播报逻辑；只保证后端发送契约正确。

**涉及文件**：

- 修改：`locallife/worker/task_process_payment.go`
- 可选修改：`locallife/logic/merchant_app_notification.go`
- 测试：`locallife/worker/task_process_payment_notify_rider_test.go`
- 测试：`locallife/worker/task_payment_domain_outbox_test.go`

**改动步骤**：

1. 在 `notifyMerchantNewOrder` 函数第一行增加状态校验：
   - 允许状态以商户可处理状态为准，最低要求包含 `db.OrderStatusPaid`。
   - `pending`、`cancelled` 或空状态必须 `log.Error()` 并返回错误。
   - 日志字段：`order_id`、`merchant_id`、`order_no`、`status`、`payment_method`。
2. 保持 `sendOrderPaidNotifications` 返回错误，让 `dispatchOrderPaymentSucceededOutbox` 感知失败；不要吞掉错误。
3. 替换 `payload, _ := json.Marshal(orderSnapshot)`：
   - marshal 失败时 `log.Error()` 并返回 `fmt.Errorf("marshal merchant new order websocket payload: %w", err)`。
4. 替换 `wsMessageJSON, _ := json.Marshal(pushMsg)`：
   - marshal 失败时同样记录并返回错误。
5. WebSocket 发布如果当前 `publishWSMessage` 没有返回错误，需要在任务卡实现时确认其签名：
   - 如果能改为返回 error，则发布失败返回错误。
   - 如果底层确实是 fire-and-forget 且没有错误返回，至少在 `publishWSMessage` 内部保证发布失败落结构化日志。
6. `merchantPayload.Amount` 仍可保留为用户实付总额，但 ExtraData 和 WebSocket snapshot 必须在任务卡 4 接入 `fee_breakdown` 后同步带上详细费用。

**错误与日志契约**：

- 非 paid 订单进入通知：`Error` 日志 + 返回错误；outbox/task 不标记成功。
- items 查询失败、items 视图构建失败、通知任务入队失败、JSON marshal 失败：返回 wrapped error，由 worker/outbox 失败路径记录并重试。
- 不允许出现 `_ = json.Marshal(...)` 或 `payload, _ := ...` 这类影响对外通知的忽略错误。

**测试要求**：

- `notifyMerchantNewOrder` 输入 `pending` 订单时不调用通知分发、不发布 WebSocket，并返回错误。
- WebSocket payload marshal 错误可通过注入不可序列化字段或拆出 builder 测试覆盖；如果无法自然构造，需要拆小纯函数。
- outbox 测试覆盖通知失败时 outbox 不被标记 dispatched。

**验收命令**：

```bash
cd locallife
go test ./worker -run 'TestNotifyMerchantNewOrder|TestProcessTaskPaymentDomainOutbox'
make test-safety
```

---

## 任务卡 3：建立商户视角费用清单计算边界

**问题**：订单响应只有 `subtotal`、`delivery_fee`、`discount_amount`、`delivery_fee_discount`、`total_amount` 等基础字段，没有商户可理解的费用清单；平台服务费、支付通道费、商户实收没有统一商户视角口径。
**目标**：新增后端统一计算/组装函数，所有商户订单和财务响应复用，字段只含金额。
**边界**：不改变真实分账计算规则；不改变 provider/internal 对账字段；不把 30bps provider 成本暴露给商户。

**涉及文件**：

- 新增：`locallife/logic/merchant_order_fee_breakdown.go`
- 测试：`locallife/logic/merchant_order_fee_breakdown_test.go`
- 可能修改：`locallife/logic/interfaces.go`
- 可能修改：`locallife/db/query/profit_sharing_order.sql`
- 生成：`locallife/db/sqlc/profit_sharing_order.sql.go`、`locallife/db/sqlc/querier.go`

**改动步骤**：

1. 在 logic 层新增商户视角类型，例如：

```go
type MerchantOrderFeeBreakdown struct {
    FoodAmount                 int64 `json:"food_amount"`
    MerchantDiscountAmount     int64 `json:"merchant_discount_amount"`
    VoucherDiscountAmount      int64 `json:"voucher_discount_amount"`
    FoodPayableAmount          int64 `json:"food_payable_amount"`
    DeliveryFeeAmount          int64 `json:"delivery_fee_amount"`
    DeliveryFeeDiscountAmount  int64 `json:"delivery_fee_discount_amount"`
    DeliveryPayableAmount      int64 `json:"delivery_payable_amount"`
    CustomerPayableAmount      int64 `json:"customer_payable_amount"`
    PlatformServiceFeeAmount   int64 `json:"platform_service_fee_amount"`
    PaymentChannelFeeAmount    int64 `json:"payment_channel_fee_amount"`
    MerchantReceivableAmount   int64 `json:"merchant_receivable_amount"`
}
```

2. 新增构建输入，显式接收 `db.Order` 和可选 `db.ProfitSharingOrder`：

```go
type BuildMerchantOrderFeeBreakdownInput struct {
    Order              db.Order
    ProfitSharingOrder *db.ProfitSharingOrder
}
```

3. 计算规则：
   - 商品/代取/优惠从 `orders` 表取。
   - `food_payable_amount + delivery_payable_amount` 必须等于 `customer_payable_amount`，除非既有业务明确有余额抵扣等字段需要单列；若不一致，返回错误。
   - `platform_service_fee_amount = platform_commission + operator_commission`。
   - `payment_channel_fee_amount` 对 Baofoo v2 使用 `merchant_payment_fee`；旧路径使用商户承担的 payment fee 字段。不得使用 `provider_payment_fee`。
   - `merchant_receivable_amount = merchant_amount`。
4. 对已支付且应分账的订单，缺少 `ProfitSharingOrder` 时返回语义错误，不允许返回空费用清单。
5. 对尚未完成分账创建但已支付的边界，要在支付后 worker 中先保证费用/分账记录创建，再发送商户通知；如果现有链路无法保证顺序，需要任务卡实现时调整 outbox 或通知时机，而不是前端兜底。
6. 金额校验错误返回 typed/domain error，例如 `ErrMerchantFeeBreakdownUnavailable` 或 `ErrMerchantFeeBreakdownInconsistent`，由 API/worker 分别映射为稳定提示和结构化日志。

**错误与日志契约**：

- logic 纯计算函数不直接打日志，返回明确错误。
- API 调用方遇到费用缺失或金额不一致：记录结构化错误，返回 500，message “订单费用明细暂不可用，请稍后重试”。
- worker 通知遇到费用缺失或金额不一致：`Error` 日志 + 返回错误，触发任务失败/重试。

**测试要求**：

- 外卖订单拆分餐费、运费、优惠、平台服务费、支付通道费、商户实收。
- 平台服务费聚合 platform + operator，不输出拆分字段。
- Baofoo v2 使用 `merchant_payment_fee`，不使用 `provider_payment_fee`。
- 金额不一致返回错误，不返回部分结果。
- 缺少分账记录返回错误，不降级为空清单。

**验收命令**：

```bash
cd locallife
go test ./logic -run 'TestBuildMerchantOrderFeeBreakdown'
```

---

## 任务卡 4：商户订单 API 和新订单通知返回费用清单

**问题**：商户订单列表、详情、通知和 WebSocket snapshot 没有详细费用清单；通知 ExtraData 只有 `total_amount`，商户无法区分餐费、运费、平台服务费、支付通道费、实收。
**目标**：所有商户订单消费面都带 `fee_breakdown`，字段与任务卡 3 完全一致。
**边界**：不要求用户端订单 API 展示商户实收；不修改 `merchant_app/` 解析逻辑，但后端契约必须准备好。

**涉及文件**：

- 修改：`locallife/api/order.go`
- 修改：`locallife/worker/task_process_payment.go`
- 修改：`locallife/logic/merchant_app_notification.go`
- 修改或新增 SQL：`locallife/db/query/profit_sharing_order.sql`
- 生成：`locallife/db/sqlc/*.sql.go`
- 测试：`locallife/api/order_test.go`
- 测试：`locallife/worker/task_process_payment_notify_rider_test.go`

**改动步骤**：

1. 在 API response 新增商户视角 DTO：

```go
type merchantOrderFeeBreakdownResponse struct {
    FoodAmount                int64 `json:"food_amount"`
    MerchantDiscountAmount    int64 `json:"merchant_discount_amount"`
    VoucherDiscountAmount     int64 `json:"voucher_discount_amount"`
    FoodPayableAmount         int64 `json:"food_payable_amount"`
    DeliveryFeeAmount         int64 `json:"delivery_fee_amount"`
    DeliveryFeeDiscountAmount int64 `json:"delivery_fee_discount_amount"`
    DeliveryPayableAmount     int64 `json:"delivery_payable_amount"`
    CustomerPayableAmount     int64 `json:"customer_payable_amount"`
    PlatformServiceFeeAmount  int64 `json:"platform_service_fee_amount"`
    PaymentChannelFeeAmount   int64 `json:"payment_channel_fee_amount"`
    MerchantReceivableAmount  int64 `json:"merchant_receivable_amount"`
}
```

2. 在 `orderResponse` 增加 `FeeBreakdown *merchantOrderFeeBreakdownResponse json:"fee_breakdown,omitempty"`。
3. 不把费用加载塞进通用 `newOrderResponse(o db.Order)`，避免影响用户端和内部调用；新增商户专用 builder，例如：
   - `newMerchantOrderResponse(ctx, order, items, feeBreakdown)`，或
   - handler 先调用 `newOrderResponse`，再显式挂 `FeeBreakdown`。
4. `listMerchantOrders` 批量加载订单对应的支付单/分账记录，避免 N+1。若当前没有批量 query，新增 `ListProfitSharingOrdersByOrderIDsForMerchant`：
   - JOIN `payment_orders`，通过 `payment_orders.order_id = ANY($1)` 查分账记录。
   - 限定 `profit_sharing_orders.merchant_id = $2`。
5. `getMerchantOrder` 加载该订单的分账记录并构建 `fee_breakdown`。
6. 任一 paid 商户订单构建费用失败：
   - API 返回 500，message “订单费用明细暂不可用，请稍后重试”。
   - 记录结构化错误字段：`order_id`、`merchant_id`、`payment_order_id`、`profit_sharing_order_id`、`status`。
7. `notifyMerchantNewOrder` 在构建 ExtraData 和 WebSocket snapshot 前加载费用清单。
8. 通知 ExtraData 增加：

```json
{
  "fee_breakdown": {
    "food_amount": 10000,
    "merchant_discount_amount": 300,
    "voucher_discount_amount": 200,
    "food_payable_amount": 9500,
    "delivery_fee_amount": 800,
    "delivery_fee_discount_amount": 0,
    "delivery_payable_amount": 800,
    "customer_payable_amount": 10300,
    "platform_service_fee_amount": 475,
    "payment_channel_fee_amount": 57,
    "merchant_receivable_amount": 8968
  }
}
```

9. WebSocket `new_order` snapshot 同步加入同名 `fee_breakdown`。
10. Swagger 注释更新商户订单列表/详情响应说明。

**错误与日志契约**：

- API 费用构建失败：`internalError` 或等价已记录 5xx 边界，公开 message 使用稳定中文提示。
- worker 费用构建失败：`log.Error()` + 返回错误，不发送缺费用的通知。
- JSON marshal 失败：必须返回错误，不允许 `_` 忽略。

**测试要求**：

- `/v1/merchant/orders` 返回的每个 paid 订单都带 `fee_breakdown`。
- `/v1/merchant/orders/{id}` 返回 `fee_breakdown`。
- 缺少分账记录或金额不一致时返回 500 且 message 稳定。
- `notifyMerchantNewOrder` 的 SendNotification ExtraData 和 WebSocket payload 都包含 `fee_breakdown`。
- 响应不包含 `provider_payment_fee`、`provider_payment_fee_rate_bps`、`operator_commission`、`platform_commission` 等商户不应看到的拆分字段。

**验收命令**：

```bash
cd locallife
make sqlc
make swagger
go test ./api -run 'Test(ListMerchantOrdersAPI|GetMerchantOrderAPI)'
go test ./worker -run 'TestNotifyMerchantNewOrder'
make check-generated
```

---

## 任务卡 5：商户财务 API 改为商户可见费用口径

**问题**：商户财务接口当前暴露 `platform_commission`、`operator_commission`、`total_platform_fee`、`total_operator_fee` 等拆分字段，不符合“平台 5% 服务费不拆分展示”的要求；支付费字段也容易混淆 provider 30bps 与商户 60bps。
**目标**：商户财务 API 只展示商户可理解金额：平台服务费金额、支付通道费金额、商户实收金额。内部平台/对账接口继续保留拆分和 provider 成本。
**边界**：不修改平台对账、运营商统计、内部 reconciliation API 的字段；只改 `/v1/merchant/finance/**` 商户视角契约。

**涉及文件**：

- 修改：`locallife/api/merchant_finance.go`
- 修改：`locallife/db/query/profit_sharing_order.sql`
- 修改：`locallife/db/query/merchant_settlement_adjustment.sql`
- 生成：`locallife/db/sqlc/profit_sharing_order.sql.go`
- 生成：`locallife/db/sqlc/merchant_settlement_adjustment.sql.go`
- 测试：`locallife/api/merchant_finance_test.go`
- 测试：`locallife/db/sqlc/profit_sharing_order_test.go`

**改动步骤**：

1. `financeOverviewResponse` 面向商户新增/改名：
   - `total_platform_service_fee_amount`
   - `total_payment_channel_fee_amount`
   - `total_deduction_fee_amount`
   - `total_merchant_receivable_amount`
2. 商户财务订单明细 `financeOrderItem` 改为：
   - `platform_service_fee_amount = platform_commission + operator_commission`
   - `payment_channel_fee_amount = merchant_payment_fee` 或旧路径商户承担支付费
   - `merchant_receivable_amount = merchant_amount`
   - 不输出 `platform_commission`、`operator_commission`。
3. 服务费明细 `serviceFeeItem` 改为：
   - `platform_service_fee_amount`
   - `payment_channel_fee_amount`
   - `total_fee_amount`
   - 不输出 `platform_fee`、`operator_fee`。
4. 每日财务 `dailyFinanceItem` 改为同样口径：
   - `platform_service_fee_amount`
   - `payment_channel_fee_amount`
   - `merchant_receivable_amount`
5. 结算记录和结算时间线 `merchantSettlementItem` / `merchantSettlementTimelineItem` 改为：
   - `platform_service_fee_amount`
   - `merchant_receivable_amount`
   - 必要时加 `payment_channel_fee_amount`
   - 不输出平台/运营商拆分。
6. SQL 查询层直接输出商户视角字段，避免 handler 到处手写 `platform + operator`：
   - `platform_service_fee_amount = platform_commission + operator_commission`
   - `payment_channel_fee_amount = CASE WHEN calculation_version = 'baofu_fee_v2' THEN merchant_payment_fee ELSE payment_fee END`
7. 对旧字段兼容策略由产品/API 版本决定：
   - 推荐先新增新字段并停止新页面使用旧字段。
   - 若必须保留旧字段一个版本，旧字段标记 deprecated，但不得在商户 App 新逻辑中使用。
   - 最终商户接口不得长期暴露拆分字段。
8. Swagger 注释和测试期望同步更新。

**错误与日志契约**：

- 财务 SQL/DB 错误：handler 用 `internalError` 记录并返回稳定 500。
- 日期参数错误：400，message 保持明确，例如“invalid date range”或现有中文稳定文案。
- 费用字段为负或金额不一致：不要在 handler 修正为 0；返回 500 并记录 `merchant_id`、`payment_order_id`、`profit_sharing_order_id`、相关金额。

**测试要求**：

- 概览汇总只断言新商户字段，不再依赖平台/运营商拆分字段。
- 订单明细、服务费明细、每日汇总、结算记录、时间线都不输出 `platform_commission` / `operator_commission`。
- Baofoo v2 的支付通道费取 `merchant_payment_fee`，不是 `provider_payment_fee` 或旧 `payment_fee`。
- 平台内部 reconciliation 测试不受影响，仍可看到 provider 成本和平台净支付费差额。

**验收命令**：

```bash
cd locallife
make sqlc
make swagger
go test ./api -run 'TestMerchantFinance'
go test ./db/sqlc -run 'Test.*ProfitSharing'
make check-generated
```

---

## 任务卡 6：错误提示、日志和观测收口

**问题**：本次修复牵涉 API、logic、SQL、worker 和异步 outbox。如果某处把错误降级为空列表、空费用清单、缺字段通知或 vague 500，前端和运营无法判断真实状态。
**目标**：形成统一错误处理收口：业务冲突给 4xx 语义提示，系统/数据异常有结构化日志并返回稳定 5xx 提示，worker 异常进入失败/重试路径。
**边界**：不重构全站错误模型；只在本次触达路径补齐，不顺手改无关模块。

**涉及文件**：

- 修改：`locallife/api/order.go`
- 修改：`locallife/api/merchant_finance.go`
- 修改：`locallife/logic/order_query_service.go`
- 修改：`locallife/logic/merchant_order_fee_breakdown.go`
- 修改：`locallife/worker/task_process_payment.go`
- 测试：对应 API/logic/worker 测试文件

**改动步骤**：

1. 定义本次前端可见提示：
   - 未支付订单不可处理：409，“订单尚未支付，暂不可处理”。
   - 订单费用明细不可用：500，“订单费用明细暂不可用，请稍后重试”。
   - 商户订单不属于当前商户：403，保留现有语义，同时记录审计日志。
   - 商户不存在或当前用户非商户：保持现有 403/404 语义，不混入未支付提示。
2. API handler 中：
   - 4xx 业务错误走 `writeLogicRequestError` 或 `errorResponse`。
   - 5xx 走 `internalError`，不能用 `errorResponse(err)`。
   - 502/503 如果出现 provider/upstream 依赖，走 `loggedServerError` 并返回稳定公开文案。
3. logic 层：
   - 业务冲突使用 `logic.NewRequestError(409, errors.New("..."))`。
   - DB、序列化、依赖缺失等系统错误用 `fmt.Errorf("context: %w", err)` 返回，不在 logic 重复打 Error 日志。
4. worker 层：
   - 在决定中止/重试处 `log.Error()`，字段包含业务 ID。
   - 返回错误给 asynq/outbox，不吞错。
5. 增加一次 targeted grep 审查：

```bash
cd locallife
rg -n "json\\.Marshal\\(.*\\)|_, _ :=|payload, _ :=|errorResponse\\(err\\)" api logic worker
```

6. 对本次触达代码逐项确认没有：
   - 费用缺失返回空对象。
   - marshal 错误被忽略。
   - pending 订单被静默过滤但详情不给明确提示。
   - worker 失败仍标记成功。

**测试要求**：

- API failure-path 测试断言 status code 和 message。
- worker failure-path 测试断言错误返回，且不会发送不完整通知。
- 费用构建 failure-path 测试断言不会生成部分结果。

**验收命令**：

```bash
cd locallife
go test ./api -run 'Test(ListMerchantOrdersAPI|GetMerchantOrderAPI|MerchantFinance)'
go test ./logic -run 'Test(OrderService.*Merchant|BuildMerchantOrderFeeBreakdown)'
go test ./worker -run 'TestNotifyMerchantNewOrder|TestProcessTaskPaymentDomainOutbox'
make test-safety
make check-generated
```

---

## 实施顺序

1. 先做任务卡 1，立即关闭 merchant API 未支付订单可见性。
2. 再做任务卡 2，给异步通知入口加 paid-only 防线和 marshal 错误处理。
3. 再做任务卡 3，建立费用清单统一计算边界。
4. 再做任务卡 4，把费用清单接入商户订单 API、通知和 WebSocket。
5. 再做任务卡 5，统一商户财务 API 的商户可见费用口径。
6. 最后做任务卡 6，扫错误处理、日志、Swagger、生成物和高风险验证。

这个顺序避免先改展示字段但继续暴露未支付订单，也避免通知先带费用但费用来源还不稳定。

---

## 回归与发布检查清单

- [x] `pending` 订单不会出现在 `/v1/merchant/orders` 默认列表。
- [x] `/v1/merchant/orders?status=pending` 返回 409 和“订单尚未支付，暂不可处理”。
- [x] `/v1/merchant/orders/{id}` 命中本商户 pending 订单返回 409 和同一提示。
- [x] `notifyMerchantNewOrder` 收到 pending 订单会记录错误并返回错误，不发送通知。
- [x] 商户订单列表、详情、通知 ExtraData、WebSocket snapshot 都包含 `fee_breakdown`。
- [x] `fee_breakdown` 只含金额字段，不含比例、bps、provider 30bps、平台/运营商拆分。
- [x] 平台服务费按商户可见总额展示，即 `platform_commission + operator_commission`。
- [x] 支付通道费按商户承担金额展示，Baofoo v2 使用 `merchant_payment_fee`。
- [x] 商户财务 API 不再面向商户暴露 `platform_commission` / `operator_commission`。
- [x] 所有费用缺失、金额不一致、JSON marshal、DB 查询失败都有错误返回和结构化日志。
- [x] `make sqlc`、`make swagger`、`make check-generated` 已按源文件变更执行。
- [x] 支付/订单高风险路径已运行 targeted tests 和 `make test-safety`。

## 完成记录

- `make sqlc` 已执行，SQLC 与 mock 生成物已同步。
- `make swagger` 已执行，Swagger 生成物已同步。
- `make check-generated` 已通过。
- `make test-safety` 已通过。
- `go test ./logic ./worker ./api ./db/sqlc -count=1` 已通过。
- `make lint-filesize` 已执行但未通过，失败原因是仓库既有超大文件清单；本次未扩大该问题边界。

---

## 不在本计划内

- 不修改 `merchant_app/` 的轮询、语音播报、状态映射、打印模板或 UI 展示。
- 不改变用户端下单、支付、取消订单的用户视角能力。
- 不改变平台内部 reconciliation 对 provider 30bps、平台净支付费差额、分账接收方金额的内部展示。
- 不重构全站订单 DTO；只在商户视角挂载费用清单。
- 不用前端隐藏 pending 作为主要修复手段；后端必须先保证契约正确。
