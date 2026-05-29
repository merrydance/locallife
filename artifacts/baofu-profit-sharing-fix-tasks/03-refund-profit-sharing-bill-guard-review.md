# Task 03: 退款污染宝付待分账账单保护复核

## 背景

原风险是：支付成功后自动创建宝付分账账单，如果后续出现退款，已存在的 pending 分账账单可能继续按旧金额发起分账，污染资金分配。

当前代码已经把保护下沉到事务层：

- `CreateBaofuProfitSharingOrderTx`
- `EnsureBaofuProfitSharingBillTx`
- `PrepareBaofuProfitSharingCommandTx`

三者都会锁定支付单，并检查未完成退款、成功退款净额、业务范围和账单金额陈旧。

## 目标

复核当前修复是否满足生产要求，并补齐必要测试或文档说明。若复核确认已满足，不做重复实现。

## 不允许的处理

- 不允许只在 worker 或 API 层做事务外检查。
- 不允许存在退款时静默继续按旧账单分账。
- 不允许把成功退款净额口径推广到未确认的订单类型。

## 前端/调用方语义

事务层返回稳定业务错误，调用方语义应为：

- `订单已有未完成退款申请，不能继续发起宝付分账`
- `宝付分账账单金额与退款后净额不一致`
- `宝付分账成功退款净额口径仅适用于预订支付单`

这些语义不能被上层吞掉成成功。

## 修改范围

预计只读复核：

- `locallife/db/sqlc/tx_baofu_profit_sharing.go`
- `locallife/db/sqlc/tx_baofu_profit_sharing_test.go`
- `locallife/logic/payment_fact_application_service.go`
- `locallife/worker/task_baofu_profit_sharing.go`

如发现覆盖缺口，再补测试。

## 复核步骤

1. 确认 `EnsureBaofuProfitSharingBillTx` 在事务内 `GetPaymentOrderForUpdate`。
2. 确认 `EnsureBaofuProfitSharingBillTx` 检查 `GetTotalActiveRefundedByPaymentOrder`。
3. 确认 `EnsureBaofuProfitSharingBillTx` 检查 `GetTotalSuccessfulRefundedByPaymentOrder` 和净额。
4. 确认 `PrepareBaofuProfitSharingCommandTx` 在真正调用宝付前重复检查。
5. 确认测试覆盖 active refund、成功退款净额、非 reservation 成功退款拒绝、陈旧账单拒绝。
6. 如有缺口，按 TDD 补齐。

## 验收标准

- 退款占用和成功退款净额保护在事务层。
- worker 发起宝付前不会绕过事务保护。
- 自动生成账单与手动/恢复发起账单共用保护。
- 本任务若无代码改动，应在最终报告中明确“复核关闭，无需修复”。

## Review 检查点

- 是否存在另一个分账发起入口没走 `PrepareBaofuProfitSharingCommandTx`。
- 是否把成功退款净额限制在已确认业务类型。
- 是否有清晰的 400/409 业务错误语义。

## 执行与 Review 结论

状态：复核关闭，无需新增修复。

- `CreateBaofuProfitSharingOrderTx`、`EnsureBaofuProfitSharingBillTx`、`PrepareBaofuProfitSharingCommandTx` 均在事务层锁定支付单并检查退款状态。
- 未完成退款会阻断分账；成功退款后的净额口径仅允许 `reservation` / `reservation_addon`。
- 发起宝付前会重复检查 stale bill 金额，避免退款后继续按旧账单发起分账。
- 上层 worker 不绕过 `PrepareBaofuProfitSharingCommandTx`。
- 已验证：`go test -count=1 ./db/sqlc`。
