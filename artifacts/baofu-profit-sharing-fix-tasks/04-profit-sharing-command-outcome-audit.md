# Task 04: 宝付分账外部命令 outcome 审计落库

## 背景

外部支付命令审计表用于记录本地向支付提供方发出的命令及其受理结果。宝付分账 worker 当前只在调用宝付前写入 `submitted`，调用成功后只更新分账单的 `sharing_order_id` 并打日志，审计表没有记录 `accepted/rejected/unknown` outcome。

此外，`CreateExternalPaymentCommand` 的冲突语义只返回已有记录，不更新状态；因此不能靠再次 `Create...` 覆盖 submitted。

## 目标

宝付分账命令在 provider call 之后必须记录 outcome：

- 受理/处理中：`accepted`
- 明确业务拒绝：`rejected`
- 超时、网络异常、无法确认 provider 是否受理：`unknown`

命令 outcome 必须可审计、幂等，并且不泄漏 raw provider message。

## 不允许的处理

- 不允许只停留在 `submitted`。
- 不允许用再次 `CreateExternalPaymentCommand` 覆盖状态。
- 不允许 provider 超时被当成 rejected。
- 不允许把 raw provider message 直接存入前端可见字段或不安全日志。
- 不允许失败时静默吞掉审计写入错误。

## 前端/调用方语义

审计 outcome 的错误字段使用稳定本地语义，例如：

- `baofu_profit_sharing_accepted`
- `baofu_profit_sharing_rejected`
- `baofu_profit_sharing_unknown`

面向前端/运营的语义应为：

- accepted：`宝付分账已受理，等待结果回调或系统查询`
- rejected：`宝付分账请求被拒绝，请检查分账订单状态或联系平台处理`
- unknown：`宝付分账请求结果暂不确定，系统将通过查询恢复`

## 修改范围

- `locallife/db/query/external_payment_fact.sql`
- 生成：`locallife/db/sqlc/external_payment_fact.sql.go`
- 生成：`locallife/db/sqlc/querier.go`
- 生成/可能变更：`locallife/db/mock/store.go`
- `locallife/logic/payment_command_service.go`
- `locallife/logic/payment_command_service_test.go`
- `locallife/worker/task_baofu_profit_sharing.go`
- `locallife/worker/task_baofu_profit_sharing_test.go`

## 实现步骤

1. SQL 增加 `UpdateExternalPaymentCommandOutcome`，按 `id` 条件更新 `command_status`、`accepted_at`、`rejected_at`、`last_error_code`、`last_error_message`、`response_snapshot`。
2. 为 command service 增加 `RecordExternalPaymentCommandOutcome` 或同等窄接口。
3. 测试 command service：submitted -> accepted 写 `accepted_at`，不写 raw message。
4. 测试 command service：submitted -> rejected 写 `rejected_at` 和稳定错误码。
5. 测试 command service：submitted -> unknown 不写 accepted/rejected 时间，但记录稳定错误码。
6. worker 调用宝付前保存 command id。
7. `CreateProfitSharing` 返回受理结果后，更新 outcome 为 accepted，并把安全 response snapshot 写入审计。
8. provider 明确业务拒绝时，更新 outcome 为 rejected。
9. provider 超时/不确定时，更新 outcome 为 unknown，并返回可重试错误让恢复查询接管。
10. `make sqlc`、必要时 `make mock`。

## 验收标准

- 分账命令不再长期只停留在 submitted。
- outcome 更新是幂等、可重试的。
- unknown 不被错误归类为 rejected。
- response/error snapshot 不包含 raw provider message、敏感商户号、证书、签名、完整 raw payload。
- worker 现有分账流程、退款保护和 snapshot 金额校验不被破坏。

## Review 检查点

- SQL 是否只更新目标 command，不扩大写范围。
- 状态转换是否不会把 terminal outcome 回退成 submitted。
- provider error 分类是否安全，不泄漏 raw 文本。
- mock/sqlc 是否同步。
- 是否保留异步恢复和 callback/query 最终事实应用路径。

## 执行与 Review 结论

状态：已修复并完成 review。

- 新增 `UpdateExternalPaymentCommandOutcome`，按 command id 收敛 `submitted/unknown` 到 `accepted/rejected/unknown`。
- `accepted/rejected` 终态不会被 submitted 或另一终态覆盖，同终态重试也不会刷新原始终态时间和快照。
- 宝付分账 worker 在 provider call 后必须记录 outcome；审计写失败会返回错误，不静默继续更新本地 `sharing_order_id`。
- provider 业务拒绝映射为 `rejected`；超时、网络异常、空结果、缺上游引用映射为 `unknown` 并交由恢复查询收敛。
- response snapshot 只保存安全字段，不保存 raw provider message、完整上游单号、接收方商户号、签名或证书材料。
- 已验证：`go test -count=1 ./logic`、`go test -count=1 ./worker`、`go test -count=1 ./db/sqlc`、`make check-generated`、`make check-baofu-contract`。
