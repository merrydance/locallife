# TASK-PAY-006E 商户交易 legacy cleanup 任务卡

日期：2026-04-26

## 1. 目标

在 006A 至 006D 切换完成后，清理 order/reservation 在总支付处理器与 legacy refund worker 中残留的生产推进权，确保商户交易 paid/refund terminal 只有业务域 owner handler 能落最终状态。

## 2. 范围

- 删除或降级 `ProcessTaskPaymentSuccess` 中 order/reservation 生产推进分支。
- 删除或降级 `ProcessTaskRefundResult` 中 order/reservation 生产推进分支。
- 清理与 006A 至 006D 相冲突的 recovery/replay 分支，保留显式保护与日志。
- 同步总计划、callsite matrix 和相关任务卡状态。

## 3. 不在本段处理

- 不新增新的支付/退款业务能力。
- 不处理 rider deposit、claim recovery、profit sharing、applyment、withdraw。
- 不处理 009 恢复调度统一化之外的跨域收敛。

## 4. 验收

- order/reservation paid 与 refund terminal 的生产终态推进只剩各自业务域 owner handler。
- legacy worker 即使被误触发也只做显式 skip/protect，不再改业务终态。
- 文档、调用矩阵与实际代码归属一致。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./api ./worker -run 'TestProcessTaskPaymentSuccess|TestProcessTaskRefundResult|TestPaymentRecoveryScheduler|TestRefundRecoveryScheduler' -count=1`
- `rg -n 'ProcessTaskPaymentSuccess|ProcessTaskRefundResult' locallife/worker locallife/api locallife/logic`

## 6. Review 结论

风险等级：G3。原因是本段是 owner cutover 的真正关口，若清理不完整会形成双写、重复副作用或恢复链路漂移。

完成标志不是“老分支删掉了”，而是代码搜索与测试都能证明 order/reservation paid/refund terminal 的生产推进权已经单源化。

### Closeout（2026-04-26）

- 已完成：`ProcessTaskPaymentSuccess` 对 `order` / `reservation` / `reservation_addon` 只保留显式 `SkipRetry` 保护，不再进入 `ProcessPaymentSuccessTx` 的商户交易终态推进。
- 已完成：`ProcessTaskRefundResult` 对 order / reservation terminal 只保留显式 `SkipRetry` 保护；order/reservation ecommerce refund callback 与 query recovery 已统一走 fact/application。
- 已完成：`PaymentRecoveryScheduler`、`RefundRecoveryScheduler` 与 callback/combine replay 路径只在非 owner 覆盖场景下才回退 legacy task。
- 已同步：总计划 [artifacts/wechat-payment-decoupling-refactor-plan-2026-04-25.md](/home/sam/locallife/artifacts/wechat-payment-decoupling-refactor-plan-2026-04-25.md) 与调用矩阵 [artifacts/wechat-payment-callsite-matrix-2026-04-25.md](/home/sam/locallife/artifacts/wechat-payment-callsite-matrix-2026-04-25.md)。
- 验证：`go test ./logic ./api ./worker -run 'TestProcessTaskPaymentSuccess|TestProcessTaskRefundResult|TestPaymentRecoveryScheduler|TestRefundRecoveryScheduler' -count=1` 通过；代码搜索确认商户交易 legacy handler 仅剩显式保护与非 owner 业务类型入口。