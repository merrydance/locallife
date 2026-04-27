# TASK-PAY-003F-7 收付通商户注销提现命令记录任务卡

日期：2026-04-25

## 1. 目标

把收付通商户注销提现 `CreateEcommerceCancelWithdraw` 同步提交结果记录为 command，避免“申请提交被微信受理”和“注销/提现长流程已完成”混在同一语义里。

本任务只覆盖注销提现 create command 审计，不重构资格校验、素材上传、callback、recovery、query result 处理、fact 写入、fact application 消费或注销提现终态推进。

## 2. 范围

本段落地范围：

- `createMerchantCancelWithdrawApplication` 中调用 `CreateEcommerceCancelWithdraw` 的同步提交路径。
- 微信 create 成功，并且本地 application 已提交同步状态后记录 `external_payment_commands.accepted`。
- 微信 create 失败后，query by `out_request_no` 能查到申请单时记录 `external_payment_commands.accepted`。
- 微信 create 和 query 都失败，且错误属于 `ALREADY_EXISTS` / `BIZ_ERR_NEED_RETRY` / `SYSTEM_ERROR` 等模糊提交结果时，本地进入 `submit_unknown` 后记录 `external_payment_commands.unknown`。
- 微信 create 和 query 都失败，且错误明确不是模糊提交结果时，本地进入 `sync_failed` 后记录 `external_payment_commands.rejected`。

不在本段处理：

- `ValidateEcommerceCancelWithdraw` 资格校验 command 记录。
- 商户注销提现 callback / recovery / detail query 的状态同步。
- `merchant_cancel_withdraw_applications` 当前由 query/callback/recovery 更新的长流程状态语义。
- `external_payment_facts` 或 `external_payment_fact_applications`。
- API 响应状态码、用户文案或 Mini Program 页面调整。

## 3. 关键边界

- 不把 command accepted 翻译为注销完成或提现到账。
- command unknown 只表达微信同步提交结果不确定，后续仍依赖 query/recovery/callback。
- command rejected 必须发生在本地 application 已进入 `sync_failed` 后。
- command 记录失败只写日志，不影响注销提现主流程。
- response snapshot 只保存 `out_request_no`、`applyment_id`、`sub_mchid`、`cancel_state`、稳定错误码和错误摘要；不得保存 payee_info、身份证号、银行卡号、加密姓名、素材 URL、密钥或完整微信原始 payload。

## 4. 同步失败处理约束

商户注销提现以 `out_request_no` 作为微信幂等键。create 失败后当前 handler 会立即 query 同一 `out_request_no`：

- query 找到记录，说明微信侧已接受或已有同一申请，写 `accepted`。
- query 也失败，且 create 错误被判定为模糊提交结果，保留本地 `submit_unknown` 并写 `unknown`。
- query 也失败，且 create 错误明确可判定为同步拒绝，本地 `sync_failed` 后写 `rejected`。

## 5. 验收

- command provider 为 `wechat`，channel 为 `ecommerce`，capability 为 `cancel_withdraw`，command type 为 `create_cancel_withdraw`。
- business owner 为 `merchant_finance`，business object 指向本地 `merchant_cancel_withdraw_application`。
- external object type 为 `cancel_withdraw`，external object key 为 `out_request_no`。
- `applyment_id` 作为 secondary key；没有 `applyment_id` 时允许为空。
- create+query 双失败模糊路径记录 `unknown`，明确拒绝路径记录 `rejected`。
- 本段不新增 fact/application 写入，也不改变商户注销提现终态处理。

## 6. 验证

- `go test ./api -run 'TestCreateMerchantCancelWithdrawApplication.*Command' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把注销提现 create 返回当成长流程完成终态。