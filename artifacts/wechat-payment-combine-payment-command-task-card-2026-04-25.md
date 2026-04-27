# TASK-PAY-003F-4c 收付通合单支付下单命令记录任务卡

日期：2026-04-25

## 1. 目标

把订单合单支付的收付通 `CreateCombineOrder` 同步返回记录为 command 结果，避免 `prepay_id` 或调起支付参数被误读为合单已支付。

本任务是 TASK-PAY-003F 的第十个独立域切片，只覆盖 `CombinedPaymentService.CreateCombinedPaymentOrder` 中的合单 create，不覆盖单笔支付、预订换菜补款、合单 query、合单 timeout、支付 callback/fact、支付成功业务推进或关闭合单命令记录。

## 2. 范围

本段落地范围：

- `CreateCombinedPaymentOrder` 新建本地合单和子支付单后的 `CreateCombineOrder` 调用。
- 微信同步下单成功且本地合单 `prepay_id` 持久化成功后记录 `external_payment_commands.accepted`。
- 微信同步下单失败且本地子支付单和主合单都关闭成功后记录 `external_payment_commands.rejected`。

不在本段处理：

- 并发 pending 合单复用签名分支。
- 合单查询 `QueryCombineOrder`。
- 合单关闭 `CloseCombineOrder`。
- 合单 timeout 查单、callback fact 写入、fact application 消费。
- payment success 业务状态推进。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 `prepay_id`、`pay_params` 或微信 create 成功解释为支付成功。
- command 记录失败只影响审计日志，不影响合单下单主流程。
- response snapshot 只允许保存 `combine_out_trade_no`、`prepay_id`、稳定错误码和错误摘要；不得保存 `paySign`、密钥、证件号、银行卡号或完整微信原始 payload。

## 4. 同步失败处理约束

本路径在微信 create API 返回错误时会尝试关闭本地所有子支付单和主合单，然后返回错误给调用方。后续重试会重新创建新的合单和新的 `combine_out_trade_no`。

只有所有本地关闭动作都成功后才写 `rejected` command。如果任一子支付单或主合单关闭失败，则不写 `rejected`，避免同一外部键仍处于不明确状态时被 command 表首次状态锁死。

`prepay_id` 本地持久化失败时，本段不写 `accepted`；该分支会尝试把本地子单和主合单标记 failed 并关闭微信合单，属于本地接管失败，后续需另按恢复设计处理。

## 5. 验收

- 合单下单成功后记录 provider `wechat`、channel `ecommerce`、capability `combine_payment`、command type `create_payment`、business owner `order`、external object type `combined_payment`、external object key `combine_out_trade_no`。
- 成功记录使用 `prepay_id` 作为 secondary key，状态为 `accepted`。
- command 的 business object 指向本地 `combined_payment_order`。
- 微信 create API 失败且本地子支付单和主合单都关闭成功后记录 `rejected`；任一关闭失败时不新增 command。
- 本段不新增 fact/application 写入，也不新增业务终态推进。

## 6. 验证

- `go test ./logic -run 'TestCreateCombinedPaymentOrder' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把合单 create 返回当成业务终态，并且失败记录只发生在本地关闭成功之后。