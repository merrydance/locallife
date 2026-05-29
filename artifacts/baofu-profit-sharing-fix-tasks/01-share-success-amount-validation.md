# Task 01: 宝付分账成功金额精确校验

## 背景

宝付/宝财通分账请求 `sharingAmt`、分账查询/回调 `succAmt` 的金额单位都是整数分。LocalLife 的宝付分账计算也在分单位内完成，费率四舍五入到分，并把余数分配到接收方金额中。因此，宝付成功事实的金额必须与本地冻结的分账明细精确一致，不存在金额容忍区间。

当前风险点：

- `logic/baofu_profit_sharing_service.go` 在 `succAmt <= 0` 时用本地预期金额回填事实金额。
- `logic/payment_fact_application_service.go` 对宝付分账 success 事实只按状态把分账单置为 `finished`，没有校验 `fact.Amount`。

## 目标

宝付分账 success 事实只有在金额字段存在、为正、且精确等于冻结的分账接收方总额时，才能把本地分账单置为 `finished`。

## 不允许的处理

- 不允许缺失 `succAmt` 时回填本地金额并继续成功。
- 不允许把金额不一致降级为成功、忽略、跳过、或只写日志。
- 不允许用金额容忍区间。
- 不允许把宝付原始错误文本直接暴露到前端或不安全日志。

## 前端/调用方语义

金额缺失或不一致时，事实应用应失败并保留可观测错误。前端或运营侧应看到稳定的业务语义：`宝付分账结果金额与本地账单不一致，请等待系统对账或联系平台处理`。实现可以先通过 outbox / application failure / 结构化日志承载该语义，但不能静默成功。

## 修改范围

- `locallife/logic/baofu_profit_sharing_service.go`
- `locallife/logic/payment_fact_application_service.go`
- `locallife/logic/baofu_profit_sharing_service_test.go`
- `locallife/logic/payment_fact_application_service_test.go`

## 实现步骤

1. 在 `RecordShareFact` 增加测试：当 `TransactionState=SUCCESS` 且 `SuccessAmountFen=0` 时，返回错误，不创建 fact/application。
2. 在 `RecordShareFact` 增加测试：当 `TransactionState=PROCESSING` 且 `SuccessAmountFen=0` 时仍可记录非终态事实，但 `Amount.Valid=false`。
3. 修改 `RecordShareFact`：只对非成功终态或非终态允许金额为空；对 success 事实要求 `SuccessAmountFen > 0`。
4. 在 `ApplyExternalPaymentFactApplication` 的宝付分账 success 路径增加测试：`fact.Amount` 缺失时失败，不调用 `UpdateProfitSharingOrderToFinished`。
5. 增加测试：`fact.Amount` 与 `baofuProfitSharingOrderExpectedShareAmount(order)` 不一致时失败，不调用 `UpdateProfitSharingOrderToFinished`。
6. 增加测试：金额一致时仍成功置为 `finished`。
7. 修改 `applyProfitSharingSuccessFact` 或其前置校验：仅宝付分账 success fact 执行金额校验，非宝付旧路径保持现有语义。
8. 错误日志/错误文本使用稳定本地语义，避免 raw provider message 泄漏。

## 验收标准

- 宝付 success 分账事实缺金额会失败。
- 宝付 success 分账事实金额不一致会失败。
- 宝付 success 分账事实金额一致才会 finished。
- 非终态查询事实不会因为缺金额被误杀。
- 回归测试覆盖 callback/query 事实记录和 fact application 两层。

## Review 检查点

- 金额单位是否全程为分。
- 校验是否在事实应用层兜底，而不是只在 callback/parser 层。
- 是否没有任何降级成功分支。
- 错误是否结构化可观测，且不给前端/日志泄漏 raw provider message。
- 是否符合 `logic` 层职责，不把 transport DTO 泄漏进业务层。

## 执行与 Review 结论

状态：已修复并完成 review。

- `RecordShareFact` 不再在 success 缺 `succAmt` 时回填本地金额；success 缺金额会拒绝记录。
- fact application 层补了兜底校验：宝付分账 success 金额必须存在、为正，且等于冻结分账接收方总额。
- 非终态 fact 仍允许金额为空，不误杀恢复查询中的 processing 状态。
- 错误会进入 application failure 与结构化日志；错误语义为稳定中文，不暴露 raw provider payload。
- 已验证：`go test -count=1 ./logic`。
