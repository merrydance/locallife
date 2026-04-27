# TASK-PAY-006D 预订退款 terminal handler 任务卡

日期：2026-04-26

## 1. 目标

把预订类 ecommerce refund terminal callback/query 收口到 `reservation_domain` owner handler。退款申请 accepted 仍只表示 processing，terminal fact/application 才能推进预订退款完成、异常副作用与预付金额调整。

本段覆盖 `business_type=reservation` 与 reservation addon 相关 refund terminal 主链。

## 2. 范围

- reservation refund callback/query 统一写 ecommerce refund fact。
- `reservation_domain` application 消费 terminal fact，单源推进退款完成、关闭、异常副作用。
- 预订退款成功后的 prepaid amount 调整只允许在 owner handler 内发生。
- `ProcessTaskRefundResult` 中 reservation 分支降为显式跳过或保护。

## 3. 不在本段处理

- 不迁移 order refund terminal。
- 不改变预订取消发起退款的 create command 行为。
- 不处理 rider deposit refund、profit sharing return、补差退款。
- 不统一手工 reconciliation。

## 4. 验收

- reservation refund accepted 不提前关闭预订侧业务状态。
- callback/query 命中 terminal 时走同一 `reservation_domain` application 入口。
- prepaid amount 调整只在 terminal success owner handler 内发生，重复 application 不重复扣减。
- 生产代码中 reservation refund terminal 推进不再由 legacy refund worker 直接拥有。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./api ./worker -run 'TestHandleEcommerceRefund.*Reservation|TestProcessTaskRefundResult.*Reservation|TestPaymentFactServiceApplyExternalPaymentFactApplication.*ReservationRefund|TestProcessTaskPaymentDomainOutbox.*ReservationRefund' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及预订退款终态、预付金额变更、异常退款告警与用户可见状态。

完成标志不是“reservation refund fact 已记录”，而是预订退款 terminal 的生产推进权已经切到 `reservation_domain` owner。