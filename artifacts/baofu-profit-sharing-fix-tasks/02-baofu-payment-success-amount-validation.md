# Task 02: 宝付支付成功金额精确校验

## 背景

宝付聚合支付查询/回调 `succAmt` 为整数分。支付成功金额必须等于本地 `payment_orders.amount`，否则不能把本地支付单标记为 paid，也不能继续生成宝付分账账单。

当前代码中查询路径已经在 `PaymentOrderService.recordAndApplyBaofuAggregatePaymentQueryFact` 拦截了金额不一致，但 callback 会直接记录事实并入队，事实应用中的 `markBaofuPaymentOrderPaid` 没有读取并比较本地金额。

## 目标

宝付主业务支付 success fact 在应用层统一校验：

- `fact.Amount.Valid == true`
- `fact.Amount.Int64 == paymentOrder.Amount`

通过后才允许 `UpdatePaymentOrderToPaid` 和后续分账账单生成。

## 不允许的处理

- 不允许 callback 事实绕过金额校验。
- 不允许缺金额时回填本地金额。
- 不允许金额不一致时仅记录日志后继续成功。
- 不允许把宝付 raw payload 或 raw error 直接给前端。

## 前端/调用方语义

金额缺失或不一致时，事实应用失败并保留稳定错误语义：`宝付支付结果金额与本地订单金额不一致，请等待系统对账或联系平台处理`。调用方不应收到 raw provider text。

## 修改范围

- `locallife/logic/baofu_payment_fact_application.go`
- `locallife/logic/payment_fact_application_service.go`
- `locallife/logic/payment_fact_application_service_test.go`
- 视需要补充 `locallife/api/baofu_callback_test.go`

## 实现步骤

1. 增加 order 支付 fact application 测试：宝付 success fact 缺 `Amount` 时失败，不调用 `UpdatePaymentOrderToPaid` 和 `ProcessPaymentSuccessTx`。
2. 增加 order 支付 fact application 测试：宝付 success fact `Amount != paymentOrder.Amount` 时失败。
3. 增加 reservation 支付 fact application 测试：宝付 success fact 金额不一致时失败。
4. 修改 `markBaofuPaymentOrderPaid`：先 `GetPaymentOrder`，校验金额和业务类型，再执行 `UpdatePaymentOrderToPaid`。
5. 保持 terminal failure 路径不要求金额，因为失败/关闭不应依赖 success amount。
6. 确认 `createOrderPaymentOutbox -> ensureBaofuOrderPaymentBill` 只会在金额校验通过后触发。

## 验收标准

- 宝付 callback/query 产生的 success fact 应用层都受金额校验保护。
- 金额缺失/不一致不会把支付单置为 paid。
- 金额缺失/不一致不会创建宝付分账账单。
- 查询路径原有金额不一致拦截仍有效。
- 错误消息稳定，日志可观测，不泄漏 raw provider payload。

## Review 检查点

- 金额校验是否在最低可防御层 `PaymentFactService` 内完成。
- 是否避免多个写路径绕过校验。
- 是否保留 failure/closed 事实的正常处理。
- 是否影响非宝付或宝付验资费路径。
- 是否符合支付域 G3 鲁棒性要求。

## 执行与 Review 结论

状态：已修复并完成 review。

- `markBaofuPaymentOrderPaid` 在 paid 更新前读取本地支付单并校验 success fact 金额。
- 宝付主业务支付 success 金额缺失或不等于 `payment_orders.amount` 时，拒绝置 paid，也不会继续创建分账账单。
- terminal failure / closed 路径不要求 success amount，避免误伤失败事实处理。
- 错误会进入 application failure 与结构化日志；错误语义为稳定中文，不暴露 raw provider payload。
- 已验证：`go test -count=1 ./logic`。
