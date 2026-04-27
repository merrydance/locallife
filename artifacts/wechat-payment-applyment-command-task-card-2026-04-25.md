# TASK-PAY-003F-8 收付通商户进件命令记录任务卡

日期：2026-04-25

## 1. 目标

把收付通商户进件 `CreateEcommerceApplyment` 同步提交结果记录为 command，避免“申请单被微信受理”和“进件完成 / 可收款 / 已开户”混成同一语义。

本任务只覆盖进件 create command 审计，不重构进件素材上传、敏感字段加密、callback、recovery、状态页 query、fact 写入、fact application 消费或商户/运营商激活流程。

## 2. 范围

本段落地范围：

- `SubmitEcommerceApplyment` 中调用 `CreateEcommerceApplyment` 的同步提交路径。
- 微信 create 成功，并且本地 `ecommerce_applyments` 已同步为 submitted 后记录 `external_payment_commands.accepted`。
- command 记录失败只写日志，不影响进件主流程。

不在本段处理：

- 微信 create 失败时写 `rejected` 或 `unknown`。
- create 失败后按 `out_request_no` 查询补偿。
- 初次 query、callback、recovery 的状态事实化。
- `external_payment_facts` 或 `external_payment_fact_applications`。
- API 响应状态码、用户文案、Mini Program 页面或 Swagger 调整。

## 3. 关键边界

- command accepted 只表达申请创建命令已被微信受理，不表达审核通过、签约完成、账户验证完成或商户可收款。
- 只有本地 submitted 更新成功后才记录 accepted，避免外部 accepted 和本地 durable anchor 脱节。
- 当前失败路径没有本地 `sync_failed` / `submit_unknown` 安全锚点，本段不写 rejected/unknown，避免 command dedupe 锁死后续同一 `out_request_no` 的成功重试。
- response snapshot 只保存 `out_request_no`、`applyment_id` 和稳定提交状态；不得保存身份证号、银行卡号、联系人证件号、素材 URL、签名、密钥或完整微信原始 payload。

## 4. 验收

- command provider 为 `wechat`，channel 为 `ecommerce`，capability 为 `applyment`，command type 为 `create_applyment`。
- business owner 为 `applyment`，business object 指向本地 `ecommerce_applyment`。
- external object type 为 `applyment`，external object key 为 `out_request_no`。
- `applyment_id` 作为 secondary key。
- 本段不新增 fact/application 写入，也不改变进件终态处理。

## 5. 验证

- `go test ./logic -run 'TestSubmitEcommerceApplyment.*Command|TestSubmitEcommerceApplymentReturnsInitialQueryStatus|TestSubmitEcommerceApplymentFallsBackToOutRequestNoForInitialQuery|TestSubmitEcommerceApplymentSubmittedSyncFailure' -count=1`
- Review 确认本段没有新增 fact/application 写入，没有把进件 create 返回当成开户完成终态。