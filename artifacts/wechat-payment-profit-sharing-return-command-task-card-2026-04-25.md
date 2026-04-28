# TASK-PAY-003F-5b 收付通分账回退命令记录任务卡

日期：2026-04-25

## 1. 目标

把收付通分账回退 `CreateProfitSharingReturn` 同步返回记录为 command 结果，避免回退提交、处理中、同步失败或同步终态字段继续和业务退款终态混在一起。

本任务只覆盖分账回退 create command 审计，不重构当前回退结果处理语义，不覆盖 `QueryProfitSharingReturn`、回调、fact 写入、fact application 消费、退款终态推进或分账回退 recovery 调度策略。

## 2. 范围

本段落地范围：

- `RefundService.processProfitSharingRefund` 中调用 `CreateProfitSharingReturn` 的同步路径。
- `ProcessTaskInitiateRefund` 中调用 `CreateProfitSharingReturn` 的同步路径。
- 同步返回 `PROCESSING` 或 `SUCCESS` 时记录 `external_payment_commands.accepted`。
- 微信返回 `NOT_ENOUGH` / `PAYER_ACCOUNT_ABNORMAL` 等已知模糊错误并且本地回退单进入 `processing` 后记录 `external_payment_commands.unknown`。
- 同步返回明确失败且本地回退单标记 failed 成功后记录 `external_payment_commands.rejected`。

不在本段处理：

- 当前 `SUCCESS` 分支直接把本地回退单标记 success 的终态语义。
- `QueryProfitSharingReturn` 或 recovery 结果处理。
- 分账回退事实模型与业务域 terminal fact 消费。
- 个人接收方回退阻断规则。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 command accepted/unknown 解释为退款已完成。
- command 记录失败只影响审计日志，不影响退款或 worker 主流程。
- response snapshot 只保存 `out_return_no`、`out_order_no`、`return_id`、`result`、稳定错误码和错误摘要；不得保存回退账户、接收方姓名、加密姓名、银行卡号、密钥或完整微信原始 payload。

## 4. 同步失败处理约束

分账回退会按 `out_return_no` 幂等重试。只有当错误被本地明确落到 failed 状态后才写 `rejected`。对于微信已知模糊错误，本地会进入 `processing` 并进入查询/调度跟踪，因此写 `unknown` 而不是 `rejected`。

## 5. 验收

- `CreateProfitSharingReturn` 同步受理后记录 provider `wechat`、channel `ecommerce`、capability `profit_sharing`、command type `create_profit_sharing_return`、business owner `profit_sharing`、external object type `profit_sharing_return`、external object key `out_return_no`。
- command 的 business object 指向本地 `profit_sharing_return`。
- `return_id` 作为 secondary key；没有 `return_id` 时允许为空。
- 模糊错误记录 `unknown`，明确失败记录 `rejected`，成功/处理中记录 `accepted`。
- 本段不新增 fact/application 写入，也不新增退款终态语义变更。

## 6. 验证

- `go test ./logic -run 'TestCreateRefundOrder_ProfitSharingReturn' -count=1`
- `go test ./worker -run 'TestProcessTaskInitiateRefund_.*ProfitSharingReturn' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把分账回退 create 返回当成退款完成终态，并且 rejected 只发生在本地 failed 锚点之后。