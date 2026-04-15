# 微信支付商户进件能力组传播矩阵

## 1. 作用与优先级

本文件是 LocalLife 当前商户进件能力组的仓库内传播真相，用来回答“这组接口在仓库里到底接到了哪些调用面、持久化面、回调面、恢复面和后续异步面”。

它不替代官方文档，不重写官方字段契约，也不替代专项审查清单。

- 官方外部真相：`.github/standards/domains/wechat-payment/WECHAT_PAYMENT_OFFICIAL_API_BASELINE_2026-04-14.md`
- 本仓库传播真相：本矩阵
- 审查与回归补充：`.github/standards/domains/wechat-payment/WECHAT_PAYMENT_APPLYMENT_REVIEW_CHECKLIST_2026-04-14.md`

若这三者冲突，按以下顺序处理：

1. 先确认是否是官方接口契约变化
2. 再确认是否是仓库调用链已变化但本矩阵未更新
3. 最后才调整审查清单或 prompt 文案

## 2. 当前能力组边界

本矩阵覆盖的商户进件能力组，指以下 7 个官方接口在 LocalLife 当前仓库里的完整传播链：

1. `CreateEcommerceApplyment`
2. `QueryEcommerceApplymentByOutRequestNo`
3. `QueryEcommerceApplymentByID`
4. `ModifySubMerchantSettlement`
5. `QuerySubMerchantSettlement`
6. `QuerySubMerchantSettlementApplication`
7. `UploadImage`

只要变更触达以上任一接口，就必须按能力组处理，而不是只看单个 client 方法。

## 3. 当前产品范围与排除项

以下内容是当前仓库活跃产品面，不允许在实现、审查、文档或 prompt 中混写为“官方全集已经全部对外开放”：

- 当前活跃主体面只看 `merchant` 进件链路；`operator` 进件已下线，历史残留只允许以兼容测试或忽略逻辑存在。
- 当前商户自助进件只支持主体类型 `4` 个体工商户和 `2` 企业。
- 当前商户进件不是前端全量采集；现行模式是“前端补录结算账户 + 后端复用既有商户资料”。
- 经营场景二维码使用 `store_qr_code`，由后端生成并上传，不是前端可写字段。
- `wechat.EcommerceApplymentRequest` 模型能力大于当前产品开放能力，不能把模型字段全集误当成当前业务已覆盖范围。

## 4. 传播矩阵

| 官方接口 | 集成边界 | 当前主调用面 | 持久化/状态写入 | 异步/恢复/后续消费者 | 当前必须守住的约束 |
| --- | --- | --- | --- | --- | --- |
| `CreateEcommerceApplyment` | `locallife/wechat/ecommerce.go` | `locallife/logic/ecommerce_applyment_submission.go` 的 `SubmitEcommerceApplyment`，由 `locallife/api/ecommerce_applyment.go` 商户绑卡提交流程触发 | `UpdateEcommerceApplymentToSubmitted` 先写 `out_request_no`、`applyment_id`、初始状态；缺少关键字段不能继续落库 | 初次提交结果会进入状态快照、状态页同步、恢复任务、后续结果任务 | 创建响应必须校验 `applyment_id`；不能只反序列化成功就信任上游 |
| `QueryEcommerceApplymentByID` | `locallife/wechat/ecommerce.go` | `SubmitEcommerceApplyment` 初次查询；`locallife/api/ecommerce_applyment.go` 状态页同步；`locallife/worker/applyment_recovery_scheduler.go` 恢复查询 | `UpdateEcommerceApplymentStatus` 或 `ApplymentSubMchActivationTx` 写入 `status`、`sign_state`、`sign_url`、`legal_validation_url`、`account_validation`、`sub_mch_id` | `ProcessTaskApplymentResult`、状态页展示、结算验证调度前置判断 | 查询优先键默认是 `applyment_id`；只有在允许时才回退 `out_request_no` |
| `QueryEcommerceApplymentByOutRequestNo` | `locallife/wechat/ecommerce.go` | `SubmitEcommerceApplyment` 在 `applyment_id` 不可用时回退；`queryEcommerceApplymentStatus` 回退；`ApplymentRecoveryScheduler.queryApplymentStatus` 回退 | 与按 ID 查询使用同一组持久化字段，不允许出现另一套状态语义 | 与按 ID 查询共享状态页、恢复任务、结果任务消费者 | `sign_state=UNSIGNED` 且带 `sign_url` 必须被视为合法响应；不能因历史假设拒绝 |
| `ModifySubMerchantSettlement` | `locallife/wechat/ecommerce.go` | `locallife/api/settlement_account.go` 的 `modifyMerchantSettlementAccount` | 通过 `updateMerchantSettlementApplicationTracking` 记录最新 `application_no` 与提交时间 | 后续申请状态查询依赖该 `application_no`；运营排障也依赖它 | 修改结算账户是独立子流程，不等于进件创建，也不等于 0.01 校验结果 |
| `QuerySubMerchantSettlement` | `locallife/wechat/ecommerce.go` | `locallife/api/settlement_account.go` 的 `getMerchantSettlementAccount`；`locallife/worker/applyment_settlement_verification_scheduler.go` 的定时核验 | `UpdateEcommerceApplymentSettlementVerification` 写入首单后校验跟踪、检查次数、失败原因 | 结算账户页展示；首单后的 0.01 打款校验轮询；失败通知 | 账户当前信息查询与首单后验证轮询复用同一官方接口，但本地语义不同，不能混成一个状态 |
| `QuerySubMerchantSettlementApplication` | `locallife/wechat/ecommerce.go` | `locallife/api/settlement_account.go` 的 `getMerchantSettlementApplication` | 通过 `updateMerchantSettlementApplicationTracking` 刷新最近查询时间与申请跟踪 | 商户查询修改结算账户申请结果；运营排障依赖申请单号 | `application_no` 是本地追踪主键，创建修改申请后必须能回查到同一单号 |
| `UploadImage` | `locallife/wechat/ecommerce.go` | `locallife/logic/ecommerce_applyment_submission.go` 的 `UploadApplymentAsset` 负责营业执照、身份证、超级管理员证件等素材；`locallife/api/ecommerce_applyment.go` 直接上传后端生成的 `store_qr_code` | 不直接写进件状态，但其 `media_id` 进入创建请求，失败会中断提交 | 商户绑卡提交前置；二维码生成链路来自 `api/scan.go` 的后端生成能力 | 不能把图片上传当成纯 client 细节；素材来源、真图校验、二维码生成面都是进件能力组的一部分 |

## 5. 共享状态真相

商户进件能力组当前共享的本地状态真相，不允许在 handler、logic、worker、scheduler 之间各自维护一套私有解释：

- `locallife/logic/ecommerce_applyment_status.go` 是当前共享状态解析入口。
- `sign_state` 是状态解析的一等输入，不能只用 `applyment_state` 推断待签约语义。
- `sub_mch_id` 可以早于真正 `finish` 返回；拿到 `sub_mch_id` 不等于商户已开通。
- 只有在“状态解析为 `finish` 且 `sub_mch_id` 非空”时，才允许执行 `ApplymentSubMchActivationTx`。
- 状态页、回调写库、恢复调度器、结果处理任务，对同一组 `applyment_state + sign_state + sub_mch_id` 组合必须得到同一结论。

## 6. 当前仓库内关键传播链

### 6.1 提交链

- `locallife/api/ecommerce_applyment.go` 负责商户绑卡提交入口、资料装配、二维码生成与上传。
- `locallife/logic/ecommerce_applyment_submission.go` 负责调用创建接口、首轮查询、持久化提交结果。
- 创建后立即形成可被状态页、恢复任务、回调和异步结果任务消费的统一 applyment 记录。

### 6.2 状态页同步链

- `locallife/api/ecommerce_applyment.go` 的状态页同步逻辑使用 `queryEcommerceApplymentStatus`。
- 查询顺序是“先 `applyment_id`，后 `out_request_no` 回退”。
- 状态页会把远端返回的 `sign_url`、`sign_state`、`legal_validation_url`、`account_validation`、`sub_mch_id` 同步回数据库。
- 即使本地历史状态已经是 `finish`，也允许继续回查微信，以修复历史脏数据或过早写成终态的问题。

### 6.3 回调与结果任务链

- `locallife/api/payment_callback.go` 负责进件回调写库，并分发 `DistributeTaskProcessApplymentResult`。
- `locallife/worker/task_process_payment.go` 的 `ProcessTaskApplymentResult` 负责通知和结果已处理标记，不应把商户自身注册成当前主链的分账接收方。
- 结果任务并不重新定义状态，只消费共享状态解析后的结论。

### 6.4 恢复与补偿链

- `locallife/worker/applyment_recovery_scheduler.go` 负责跟进待恢复申请单。
- 恢复任务复用同一查询顺序与同一状态写回字段。
- 若状态仍需异步跟进，会继续分发进件结果任务，而不是只更新数据库就结束。

### 6.5 结算账户后续链

- `locallife/api/settlement_account.go` 负责结算账户查询、修改、申请状态查询。
- `locallife/worker/applyment_settlement_verification_scheduler.go` 负责首单后的 0.01 校验轮询。
- 结算账户修改申请状态与首单后结算验证是两个不同子阶段，不能混成一个“账户状态”。

## 7. 变更完成判定

只要本能力组有代码变更，至少要显式检查以下面是否被传播：

1. `locallife/wechat/` 集成 client 与错误映射
2. `locallife/logic/` 提交与共享状态解析
3. `locallife/api/ecommerce_applyment.go` 状态页和提交入口
4. `locallife/api/payment_callback.go` 回调写库与结果任务分发
5. `locallife/api/settlement_account.go` 结算账户三条接口
6. `locallife/worker/applyment_recovery_scheduler.go` 恢复查询与补偿
7. `locallife/worker/applyment_settlement_verification_scheduler.go` 首单后验证
8. `locallife/worker/task_process_payment.go` 进件结果任务
9. `locallife/db/sqlc/` 相关写入方法是否仍覆盖完整字段
10. 相关测试是否覆盖新增分支或契约变化

若以上任一面被明确排除，必须在任务说明或审查结论里写明“为什么本次可以不传播到该面”。

## 8. 最小验证集

能力组变更默认至少执行以下最小验证之一，并报告实际命令：

- `go test ./logic ./api ./worker ./wechat -run 'Applyment|Ecommerce'`

若本次变更明确触达结算账户子流程，再补充检查这些测试面是否需要一起跑：

- `locallife/api/settlement_account_test.go`
- `locallife/worker/applyment_settlement_verification_scheduler_test.go`

当前这组能力的现有关键测试面包括：

- `locallife/wechat/ecommerce_test.go`
- `locallife/logic/ecommerce_applyment_submission_test.go`
- `locallife/api/ecommerce_applyment_test.go`
- `locallife/api/ecommerce_applyment_status_contract_test.go`
- `locallife/api/payment_callback_test.go`
- `locallife/api/settlement_account_test.go`
- `locallife/worker/applyment_recovery_scheduler_test.go`
- `locallife/worker/applyment_support_test.go`
- `locallife/worker/task_process_applyment_result_status_test.go`
- `locallife/worker/task_process_payment_test.go`
- `locallife/worker/applyment_settlement_verification_scheduler_test.go`

## 9. 维护规则

出现以下任一情况时，必须同步更新本矩阵：

- 官方 7 个接口中的活跃边界变了
- 新增或删除了调用面、状态写回面、scheduler、worker 或回调消费者
- 当前产品开放范围变了，例如主体类型、前端采集范围、二维码生成方式、operator 是否重新开放
- 共享状态真相入口或激活条件变了

如果代码已经改了，但本矩阵没有更新，按能力组治理规则视为变更未完成。