# TASK-PAY-003F-5a 收付通分账命令记录任务卡

日期：2026-04-25

## 1. 目标

把收付通分账 `CreateProfitSharing` 和无接收方场景下的 `FinishProfitSharing` 同步返回记录为 command 结果，避免微信返回 `order_id` 或 `PROCESSING` 被误读为分账已完成。

本任务只覆盖分账发起/完结命令记录，不覆盖分账查询、分账回调、分账回退、退款、fact 写入、fact application 消费或商户收入到账通知。

## 2. 范围

本段落地范围：

- `ProcessTaskProfitSharing` 中调用 `CreateProfitSharing` 的路径。
- `finishProfitSharingOrder` 中调用 `FinishProfitSharing` 的路径。
- 微信同步调用成功且本地 `profit_sharing_orders` 进入 `processing` 后记录 `external_payment_commands.accepted`。

不在本段处理：

- `QueryProfitSharing` reconciliation。
- `ProcessTaskProfitSharingResult` 通知。
- `CreateProfitSharingReturn` 分账回退。
- 分账 callback/fact/application。
- 同步失败时的终态分类或业务失败推进。

## 3. 关键边界

- 不创建 `external_payment_facts`。
- 不创建 `external_payment_fact_applications`。
- 不把 `CreateProfitSharing` / `FinishProfitSharing` 同步返回解释为分账完成。
- command 记录失败只影响审计日志，不影响 worker 主流程。
- response snapshot 只保存 `out_order_no`、`order_id`、`status` 等稳定非敏字段；不得保存接收方账号、姓名、加密姓名、证件号、银行卡号、密钥或完整微信原始 payload。

## 4. 同步失败处理约束

分账任务会使用同一个 `out_order_no` 重试同一笔分账命令。当前失败分支没有把本地 `profit_sharing_order` 持久标记为 failed 的安全锚点，因此本段不同步写 `rejected`，避免 command 表首条状态锁死后续成功重试。

后续如果要记录 rejected，必须先有明确的不可重试分类和本地失败状态持久化锚点，并由测试覆盖重复任务语义。

## 5. 验收

- `CreateProfitSharing` 成功且本地分账单进入 `processing` 后记录 provider `wechat`、channel `ecommerce`、capability `profit_sharing`、command type `create_profit_sharing`、business owner `profit_sharing`、external object type `profit_sharing`、external object key `out_order_no`。
- `FinishProfitSharing` 成功且本地分账单进入 `processing` 后记录 command type `finish_profit_sharing`。
- 成功记录使用微信 `order_id` 作为 secondary key。
- snapshot 不包含 receiver account、receiver name、encrypted name、pay sign 或完整微信原始 payload。
- 本段不新增 fact/application 写入，也不新增分账完成业务推进。

## 6. 验证

- `go test ./worker -run 'TestProcessTaskProfitSharing' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把分账 create/finish 返回当成终态，并且失败分支没有新增错误的 rejected command。