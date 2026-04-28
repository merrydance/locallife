# TASK-PAY-003F-9 直连商家转账赔付命令记录任务卡

日期：2026-04-25

## 1. 目标

把平台赔付链路中的直连商家转账 `CreateTransfer` 同步提交结果记录为 command，避免“转账单创建/已存在”和“赔付已到账/赔付动作完成”混成同一语义。

本任务只覆盖赔付 worker 中的 create transfer command 审计，不重构商家转账 client、回调、query recovery、claim payout 终态推进、fact 写入、fact application 消费或赔付 durable anchor 模型。

## 2. 范围

本段落地范围：

- `ExecuteClaimPayoutAction` 中调用 `CreateTransfer` 的同步提交路径。
- 本地 `behavior_action` 已进入 `running` 且持久化 `out_bill_no` 后，微信 create 成功则记录 `external_payment_commands.accepted`。
- 微信 create 返回 `ALREADY_EXISTS` / duplicate 语义时，记录 `external_payment_commands.accepted`，后续仍由 query/callback/recovery 判定转账终态。
- command 记录失败只写日志，不影响赔付主流程。

不在本段处理：

- create 非 duplicate 错误时写 `rejected` 或 `unknown`。
- `QueryTransferByOutBillNo` 的 terminal fact 化。
- `HandleClaimPayoutTransferNotification` 回调事实化。
- `external_payment_facts` 或 `external_payment_fact_applications`。
- `WAIT_USER_CONFIRM` 的用户确认前端流程或赔付终态处理。

## 3. 关键边界

- command accepted 只表达商家转账创建命令被微信受理或同一 `out_bill_no` 已存在，不表达转账成功、用户已确认、微信零钱到账或 claim payout 完成。
- `WAIT_USER_CONFIRM`、`PROCESSING`、`TRANSFERING` 都不是赔付成功终态。
- 当前非 duplicate create 错误会让本地 action 进入可重试 failed 状态，没有安全的 rejected/unknown 本地锚点，本段不写 rejected/unknown，避免 command dedupe 锁死后续成功重试。
- response snapshot 只保存 `out_bill_no`、`transfer_bill_no`、`state`、duplicate 标记和 `has_package_info`；不得保存 openid、user_name、真实姓名、转账备注、package_info、完整微信原始 payload 或签名数据。

## 4. 验收

- command provider 为 `wechat`，channel 为 `direct`，capability 为 `merchant_transfer`，command type 为 `create_transfer`。
- business owner 为 `claim_recovery`，business object 指向本地 `behavior_action`。
- external object type 为 `merchant_transfer`，external object key 为 `out_bill_no`。
- `transfer_bill_no` 作为 secondary key；duplicate accepted 没有 `transfer_bill_no` 时允许为空。
- 本段不新增 fact/application 写入，也不改变赔付终态处理。

## 5. 验证

- `go test ./worker -run 'TestProcessTaskClaimPayout_.*Command|TestProcessTaskClaimPayout_Success|TestProcessTaskClaimPayout_DuplicateTransferStillMarksClaimPaid|TestProcessTaskClaimPayout_WechatCreateTransferErrorsPersistRetryableDetail' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把转账 create 返回当成赔付完成终态。