# TASK-PAY-006C 订单退款 terminal handler 任务卡

日期：2026-04-26

## 1. 目标

把订单类 ecommerce refund terminal callback/query 收口到 `order_domain` owner handler。退款申请 accepted 仍只表示 processing，只有 terminal fact/application 才能推进订单退款完成或异常副作用。

本段覆盖 `business_type=order` 的 refund terminal 主链，包括 callback、query recovery 和异常退款告警/通知副作用。

## 2. 范围

- order refund callback/query 统一写 ecommerce refund fact。
- `order_domain` application 消费 terminal fact，单源推进退款完成、关闭、异常副作用。
- `ProcessTaskRefundResult` 中 order 分支降为显式跳过或保护。
- accepted/processing 阶段不得提前关闭订单或提前完成售后业务状态。

## 3. 不在本段处理

- 不迁移 reservation refund terminal。
- 不改变退款申请 create command 语义。
- 不处理 rider deposit refund、profit sharing return 或补差退款。
- 不统一人工 reconciliation 入口。

## 4. 验收

- order refund accepted 只把本地 `refund_order` 置为 processing，不提前完成订单侧业务状态。
- callback/query 命中 terminal 时走同一 `order_domain` application 入口。
- 订单退款成功通知、异常告警等副作用从 owner/outbox 单源执行。
- 生产代码中 order refund terminal 推进不再由 `ProcessTaskRefundResult` 直接拥有。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./api ./worker -run 'TestHandleEcommerceRefund.*Order|TestProcessTaskRefundResult.*Order|TestPaymentFactServiceApplyExternalPaymentFactApplication.*OrderRefund|TestProcessTaskPaymentDomainOutbox.*OrderRefund' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及商户订单退款终态、用户通知、异常退款告警与售后状态机。

完成标志不是“refund fact 已记录”，而是 order refund terminal 的业务推进和副作用都已离开 legacy refund worker。