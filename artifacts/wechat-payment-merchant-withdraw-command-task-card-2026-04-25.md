# TASK-PAY-003F-6 收付通商户提现命令记录任务卡

日期：2026-04-25

## 1. 目标

把收付通商户提现 `CreateEcommerceWithdraw` 同步提交结果记录为 command，避免“提现提交被微信受理”和“提现已到账/失败”继续混在同一语义里。

本任务只覆盖商户提现 create command 审计，不重构当前商户提现 API handler、不改 callback、recovery、query result 处理、fact 写入、fact application 消费或提现终态推进。

## 2. 范围

本段落地范围：

- `createMerchantAccountWithdraw` 中调用 `CreateEcommerceWithdraw` 的同步提交路径。
- 微信 create 成功，并且本地 `withdrawal_record` 已创建后记录 `external_payment_commands.accepted`。
- 微信 create 超时或失败后，query by `out_request_no` 能查到微信提现单时记录 `external_payment_commands.accepted`。
- 微信 create 和 query 都失败，本地记录保持 `pending` 并进入轮询时记录 `external_payment_commands.unknown`。

不在本段处理：

- 商户提现 callback / recovery / detail query 的状态同步。
- `withdrawal_record` 当前直接被 query/callback/recovery 更新的终态语义。
- `external_payment_facts` 或 `external_payment_fact_applications`。
- API 响应状态码、用户文案或 Mini Program 页面调整。

## 3. 关键边界

- 不把 command accepted 翻译为提现成功或到账。
- command unknown 只表达微信同步提交结果不确定，后续仍依赖 query/recovery/callback。
- command 记录失败只写日志，不影响提现主流程。
- response snapshot 只保存 `out_request_no`、`withdraw_id`、`sub_mchid`、`wechat_status`、稳定错误码和错误摘要；不得保存银行卡、账户名、证件号、密钥或完整微信原始 payload。

## 4. 同步失败处理约束

商户提现以 `out_request_no` 作为微信幂等键。create 失败后当前 handler 会立即 query 同一 `out_request_no`：

- query 找到记录，说明微信侧已接受或已有同一提现单，写 `accepted`。
- query 也失败，说明状态不确定，保留本地 `pending` 并写 `unknown`。
- 本段不新增 `rejected` 分支，因为当前 handler 没有安全的本地 failed/closed 锚点；明确拒绝语义留到后续提现终态/fact 改造中处理。

## 5. 验收

- command provider 为 `wechat`，channel 为 `ecommerce`，capability 为 `withdraw`，command type 为 `create_withdraw`。
- business owner 为 `merchant_finance`，business object 指向本地 `withdrawal_record`。
- external object type 为 `withdraw`，external object key 为 `out_request_no`。
- `withdraw_id` 作为 secondary key；没有 `withdraw_id` 时允许为空。
- create+query 双失败路径记录 `unknown`，不记录 `rejected`。
- 本段不新增 fact/application 写入，也不改变商户提现终态处理。

## 6. 验证

- `go test ./api -run 'TestCreateMerchantAccountWithdrawAPI.*Command|TestCreateMerchantAccountWithdrawAPIReturnsPendingConfirmationWhenWechatCreateAndQueryFail' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把提现 create 返回当成到账终态。