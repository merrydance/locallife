# TASK-PAY-006B 预订支付成功 handler cutover 任务卡

日期：2026-04-26

## 1. 目标

把预订支付成功从总支付处理器迁到 `reservation_domain` owner handler。不同 `payment_mode` 的预订 paid callback/query 都必须走同一 paid fact/application 入口，再由预订域单源推进预订状态。

本段只覆盖 `business_type=reservation` 及 reservation addon/payment mode 相关 paid 主链，不处理退款终态。

## 2. 范围

- reservation partner JSAPI paid callback/query 统一记录 payment fact。
- 预订押金、全款、补差等 `payment_mode` 在 owner handler 内部收口，不再散落在总支付处理器。
- `reservation_domain` application 负责 paid 后的预订状态推进，保证重复投递幂等。
- 旧 `ProcessTaskPaymentSuccess` 中 reservation 分支降为显式跳过或保护。

## 3. 不在本段处理

- 不迁移 order payment paid。
- 不迁移 reservation refund terminal fact。
- 不改 reservation 退款策略或取消流程。
- 不统一 timeout 前主动查单节奏。

## 4. 验收

- reservation paid callback 重复投递不重复推进预订状态。
- query replay 命中 paid 时与 callback 命中 paid 走同一 application 入口。
- `payment_mode` 差异只存在于 `reservation_domain` owner，不再依赖总支付处理器分支。
- 生产代码中 reservation payment success 终态推进只剩 `reservation_domain` owner。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./api ./worker -run 'TestHandle(Ecommerce|Partner)Payment.*Reservation|TestProcessTaskPaymentSuccess.*Reservation|TestPaymentFactServiceApplyExternalPaymentFactApplication.*Reservation' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及预订 paid 幂等、押金/全款/补差状态机，以及 paid 后资源占用与业务可见状态。

完成标志不是“预订也能写 paid fact”，而是预订 paid 的生产推进权已经离开总支付处理器。