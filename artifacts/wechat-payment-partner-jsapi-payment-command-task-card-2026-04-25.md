# TASK-PAY-003F-4a 收付通单笔 JSAPI 支付下单命令记录任务卡

日期：2026-04-25

## 1. 目标

把普通订单和预订通过收付通单笔 JSAPI 支付下单的同步返回记录为 command 结果，避免 `prepay_id` 或调起支付参数被误读为业务已支付。

本任务是 TASK-PAY-003F 的第八个独立域切片，只覆盖 `PaymentOrderService` 中 `CreatePartnerJSAPIOrder` 的单笔支付 create，不覆盖合单支付、预订换菜补差合单、分账、支付 callback/query fact、timeout 查单或业务终态推进。

## 2. 范围

本段落地范围：

- `createOrderEcommercePayment` 普通订单收付通单笔支付 create。
- `createReservationEcommercePayment` 预订收付通单笔支付 create。
- 微信同步下单成功且本地 `prepay_id` 持久化成功后记录 `external_payment_commands.accepted`。
- 微信同步下单失败且本地 payment order 已关闭后记录 `external_payment_commands.rejected`。

不在本段处理：

- 复用已有 pending payment order 的签名分支。
- 合单 `CreateCombineOrder`。
- `ReplaceReservationOrder` 等其他直接调用 `CreatePartnerJSAPIOrder` 的路径。
- 支付 callback/query fact 写入和 fact application 消费。
- payment success 业务状态推进。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 `prepay_id`、`pay_params` 或微信 create 成功解释为支付成功。
- command 记录失败只影响审计日志，不影响支付下单主流程。
- response snapshot 只允许保存 `out_trade_no`、`prepay_id`、稳定错误码和错误摘要；不得保存 `paySign`、密钥、证件号、银行卡号或完整微信原始 payload。

## 4. 同步失败处理约束

本路径在微信 create API 返回错误时会关闭本地 payment order，然后返回错误给调用方；后续重试会创建新的 payment order 和新的 `out_trade_no`。因此只有本地关闭成功后才写 `rejected` command。

如果关闭本地 payment order 失败，则不写 `rejected`，避免同一外部键后续仍可能处于不明确状态时被 command 表首次状态锁死。

`prepay_id` 本地持久化失败时，本段不写 `accepted`；该分支会尝试关闭微信订单并把本地 payment order 标记 failed，属于本地接管失败，后续需另按恢复设计处理。

## 5. 验收

- 普通订单和预订收付通单笔下单成功后记录 provider `wechat`、channel `ecommerce`、capability `partner_jsapi_payment`、command type `create_payment`、external object type `payment`、external object key `out_trade_no`。
- 成功记录使用 `prepay_id` 作为 secondary key，状态为 `accepted`。
- command 的 business object 指向本地 `payment_order`。
- 普通订单 business owner 为 `order`，预订为 `reservation`。
- 微信 create API 失败且本地关闭成功后记录 `rejected`；关闭失败时不新增 command。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder_.*Command|TestPaymentOrderServiceCreatePaymentOrder_WechatOrderClosedReturnsConflict' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把支付 create 返回当成业务终态，并且失败记录只发生在本地关闭成功之后。