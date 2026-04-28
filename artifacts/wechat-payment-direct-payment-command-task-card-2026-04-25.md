# TASK-PAY-003F-1 直连支付下单命令记录任务卡

日期：2026-04-25

## 1. 目标

把直连 JSAPI 支付下单的同步 create 返回收口为 command 记录，而不是业务终态。

本任务是 TASK-PAY-003F 的第一个独立域切片，只覆盖直连支付下单，不覆盖平台收付通支付、退款、分账、进件、提现或转账。

## 2. 范围

首段落地范围：

- 骑手押金充值 `POST /v1/rider/deposit`。
- 同步下单成功只记录 `external_payment_commands.accepted`。
- 同步下单失败且本地 payment order 已关闭后记录 `external_payment_commands.rejected`。
- response snapshot 只允许保存脱敏下单结果，例如 `prepay_id`、错误码和稳定错误摘要；不得保存 `paySign`、密钥、证件号、银行卡号或完整微信原始 payload。

后续同域待迁移范围：

- 商户追偿 direct payment。
- 骑手追偿 direct payment。

## 3. 边界

- 不移动 handler 内既有编排到 logic 层；本段只补 command 审计，不做层级重构。
- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 prepay/create 返回展示为“支付成功”。
- 不改变支付回调、查询、关单、订单状态推进或押金入账逻辑。
- 不改 request-level 幂等语义；仍以既有 pending payment order 复用逻辑为准。

## 4. 验收

- 骑手押金充值下单成功记录 provider `wechat`、channel `direct`、capability `direct_jsapi_payment`、command type `create_payment`、business owner `rider_deposit`、external object type `payment`、external object key `out_trade_no`。
- 成功记录使用 `prepay_id` 作为 secondary key，状态为 `accepted`。
- 同步拒绝记录状态为 `rejected`，并保留稳定错误码和摘要。
- command 记录失败只影响审计日志，不改变充值接口主响应。

## 5. 验证

- `go test ./api -run 'TestDepositRiderAPI|TestWithdrawRiderAPI' -count=1`
- Review 确认本段没有新增 fact/application 写入，也没有新增业务终态推进。