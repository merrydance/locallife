# 普通服务商微信支付开户流程实施跟踪

## 背景

本项目当前只接入微信支付【普通服务商】身份，不是渠道商、从业机构或平台商户。商户小程序中的微信支付开户流程必须以普通服务商特约商户进件 `applyment4sub` 为准，不能把渠道商的“商户开户意愿确认”作为入口、状态或完成门槛。

## 官方依据

- 特约商户进件-提交申请单：https://pay.weixin.qq.com/doc/v3/partner/4012719997.md
- 特约商户进件-申请单号查询申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012697052.md
- 特约商户进件-业务申请编号查询申请状态：https://pay.weixin.qq.com/doc/v3/partner/4012697168.md

关键结论：

- `contact_info.contact_email` 为必填，必须由商户填写超级管理员邮箱，不能使用平台管理员邮箱兜底。
- `APPLYMENT_STATE_TO_BE_CONFIRMED` 在普通服务商进件中是待账户验证。
- `APPLYMENT_STATE_TO_BE_SIGNED` 是待签约。
- `APPLYMENT_STATE_SIGNING` 是开通权限中。
- `APPLYMENT_STATE_FINISHED` 是商户入驻申请已完成；本项目开户完成点为 `APPLYMENT_STATE_FINISHED + sub_mchid`，再加本地支付配置激活成功。
- `商户开户意愿确认`、`AUTHORIZE_STATE_AUTHORIZED`、渠道商 `apply4subject` 接口不得作为普通服务商开户完成门槛。

## 执行原则

- 风险等级：G3，涉及支付开户、状态机、worker 恢复、小程序异步流程和终态展示。
- 每阶段必须先写或调整测试，再改生产代码。
- 每阶段完成后运行最小验证，记录结果，再勾选。
- 不回退工作区中与本任务无关的既有改动。

## 阶段清单

- [x] 阶段 1：后端状态语义修正
  - `finish` 文案改为微信支付已开通。
  - `to_be_confirmed` 文案改为账户验证。
  - 测试覆盖普通服务商状态映射和文案不得出现开户意愿/授权。
  - 验证：`cd locallife && go test ./logic -run 'Applyment|OrdinaryApplyment|NormalizeResolved'`

- [x] 阶段 2：后端 API 完成态门槛修正
  - `/v1/merchant/applyment/status` 查询到 `finish + sub_mchid` 后走本地激活。
  - 不再调用 `QueryAccountAuthorizeState` 作为普通服务商完成门槛。
  - 验证：`cd locallife && go test ./api -run 'MerchantApplyment|Applyment'`

- [x] 阶段 3：worker 与 ordinaryserviceprovider 非普通服务商能力清理
  - 恢复任务不再查询开户意愿授权状态。
  - ordinaryserviceprovider 主接口移除或隔离非普通服务商授权查询能力。
  - 验证：`cd locallife && go test ./worker -run 'ApplymentRecovery|Applyment' && go test ./wechat/ordinaryserviceprovider/...`

- [x] 阶段 4：超级管理员邮箱必填链路
  - 小程序提交页要求商户填写超级管理员邮箱。
  - 后端拒绝缺失或非法邮箱，不使用平台管理员邮箱兜底。
  - 验证：`cd locallife && go test ./logic -run 'ApplymentSubmission|ContactEmail|Ordinary' && go test ./api -run 'MerchantApplyment|ContactEmail' && cd ../weapp && npm run lint`

- [x] 阶段 5：小程序 ViewModel 去开户意愿确认
  - 删除 `needsOpenConfirmation`、`account_confirmation` 等渠道商语义。
  - `finish + sub_mch_id` 展示已开通。
  - `to_be_confirmed` 展示账户验证。
  - 验证：`cd weapp && npm run lint && npm run compile`

- [x] 阶段 6：提交后自动等待与状态同步
  - 提交成功后自动轮询微信申请单状态 20-30 秒。
  - 有 `sign_url` 时自动进入微信待办页。
  - 超时后展示系统继续同步，而不是要求用户手动刷新。
  - 验证：`cd weapp && npm run lint && npm run compile`

- [x] 阶段 7：微信待办页 UI 调整
  - `sign_url` 页面统一承接绑定微信、账户验证、签约等普通服务商微信侧待办。
  - 不出现开户意愿确认入口或文案。
  - 验证：`cd weapp && npm run lint && npm run compile`

- [x] 阶段 8：开户完成终态页
  - 开户首页终态展示微信支付已开通、`sub_mch_id`、结算账户入口和后续说明。
  - 不展示授权未完成或开户意愿确认。
  - 验证：`cd weapp && npm run lint && npm run compile`

- [x] 阶段 9：文档与全量验证
  - 更新微信支付域 README 的完成态规则和小程序用户流程摘要。
  - 运行后端、小程序和 prompt governance 验证。

## 阶段记录

### 阶段 1

- 状态：已完成
- 修改文件：
  - `locallife/logic/ecommerce_applyment_submission.go`
  - `locallife/logic/ecommerce_applyment_submission_test.go`
- 验证结果：
  - RED：`PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'OrdinaryApplymentFinishedFrontendMessage|OrdinaryApplymentToBeConfirmedFrontendMessage|MapOrdinaryApplymentStateToStatus'` 先失败，确认旧文案仍包含“开户意愿/授权”和“商户确认”。
  - GREEN：`PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'OrdinaryApplymentFinishedFrontendMessage|OrdinaryApplymentToBeConfirmedFrontendMessage|MapOrdinaryApplymentStateToStatus'` 通过。
  - 阶段验证：`PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Applyment|OrdinaryApplyment|NormalizeResolved'` 通过。
- Review 结果：通过。`finish` 已改为“微信支付已开通”，`to_be_confirmed` 已改为“账户验证”，新增测试防止普通服务商完成态再次被开户意愿授权语义污染。


### 阶段 2

- 状态：已完成
- 修改文件：
  - `locallife/api/ecommerce_applyment.go`
  - `locallife/api/ecommerce_applyment_test.go`
  - `locallife/db/sqlc/tx_notification.go`
  - `locallife/db/sqlc/tx_applyment_activation_test.go`
- 验证结果：
  - RED：`PATH="/usr/local/go/bin:$PATH" go test ./api -run 'GetMerchantApplymentStatusOrdinaryFinishActivatesWithSubMchID|GetMerchantApplymentStatusOrdinaryFinishActivationFailureReturnsSyncingGuidance'` 先失败，确认 API 仍调用 `QueryAccountAuthorizeState`。
  - RED：`PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run TestApplymentSubMchActivationTxActivatesOrdinaryMerchantWithoutAccountAuthorizeState` 先失败，确认激活事务仍写入 `AUTHORIZE_STATE_UNAUTHORIZED` 且不直接激活交易能力。
  - GREEN：上述 API 聚焦测试通过。
  - GREEN：上述 DB 事务测试通过。
  - 阶段验证：`PATH="/usr/local/go/bin:$PATH" go test ./api -run 'MerchantApplyment|Applyment'` 通过。
- Review 结果：通过。`finish + sub_mchid` 不再查询开户意愿授权状态；API 完成态文案不含“开户意愿”；本地激活失败时展示“平台正在同步收款配置”；事务层以普通服务商完成态直接激活支付配置和商户状态。


### 阶段 3

- 状态：已完成
- 修改文件：
  - `locallife/worker/applyment_recovery_scheduler.go`
  - `locallife/worker/applyment_recovery_scheduler_test.go`
  - `locallife/wechat/ordinaryserviceprovider/interface.go`
  - `locallife/wechat/ordinaryserviceprovider/endpoints.go`
  - `locallife/wechat/ordinaryserviceprovider/client_capability_alignment_test.go`
  - `locallife/wechat/ordinaryserviceprovider/client_endpoints_test.go`
  - `locallife/wechat/ordinaryserviceprovider/contracts/capabilities.go`
  - `locallife/wechat/ordinaryserviceprovider/contracts/capabilities_test.go`
  - `locallife/wechat/ordinaryserviceprovider/contracts/types.go`
  - `locallife/wechat/ordinaryserviceprovider/contracts/official_alignment_test.go`
  - `locallife/wechat/ordinaryserviceprovider/contracts/official_contract_bindings_test.go`
  - `locallife/wechat/ordinaryserviceprovider/contracts/official_endpoints_test.go`
  - `locallife/wechat/ordinaryserviceprovider/errorcodes/capabilities.go`
  - `locallife/wechat/ordinaryserviceprovider/errorcodes/capabilities_test.go`
  - `locallife/wechat/ordinaryserviceprovider/errorcodes/official_codes.go`
  - `locallife/wechat/ordinaryserviceprovider/errorcodes/official_codes_test.go`
  - `locallife/wechat/ordinaryserviceprovider/mock/client.go`
- 验证结果：
  - RED：`PATH="/usr/local/go/bin:$PATH" go test ./worker -run TestApplymentRecoverySchedulerRunOnceReconcilesFinishedSubMchIDWithoutAccountAuthorization` 先失败，确认 scheduler 仍调用 `QueryAccountAuthorizeState`。
  - RED：`PATH="/usr/local/go/bin:$PATH" go test ./wechat/ordinaryserviceprovider/...` 先失败，确认 active contracts/client 仍暴露 `account_willingness` 与 `/v3/apply4subject/*`。
  - GREEN：`PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'ApplymentRecovery|Applyment'` 通过。
  - GREEN：`PATH="/usr/local/go/bin:$PATH" go test ./wechat/ordinaryserviceprovider/...` 通过。
  - 阶段回归：`PATH="/usr/local/go/bin:$PATH" go test ./logic -run 'Applyment|OrdinaryApplyment|NormalizeResolved' && PATH="/usr/local/go/bin:$PATH" go test ./api -run 'MerchantApplyment|Applyment' && PATH="/usr/local/go/bin:$PATH" go test ./db/sqlc -run TestApplymentSubMchActivationTxActivatesOrdinaryMerchantWithoutAccountAuthorizeState && PATH="/usr/local/go/bin:$PATH" go test ./worker -run 'ApplymentRecovery|Applyment' && PATH="/usr/local/go/bin:$PATH" go test ./wechat/ordinaryserviceprovider/...` 通过。
- Review 结果：通过。worker 恢复分支不再查询开户意愿授权状态；普通服务商 active endpoint contract、client runtime coverage、errorcode capability map 和 mock 已移除 `AccountWillingness*` / `AccountAuthorizeState*` / `/v3/apply4subject/*`；新增契约测试防止开户意愿确认重新进入 ordinaryserviceprovider runtime contracts。


### 阶段 4

- 状态：已完成
- 修改文件：
  - `locallife/api/ecommerce_applyment.go`
  - `locallife/api/ecommerce_applyment_test.go`
  - `weapp/miniprogram/api/applyment-bank.ts`
  - `weapp/miniprogram/api/merchant-applyment.ts`
  - `weapp/miniprogram/components/applyment-bank-form/index.ts`
  - `weapp/miniprogram/components/applyment-bank-form/index.wxml`
  - `weapp/miniprogram/components/applyment-bank-form/index.wxss`
  - `weapp/miniprogram/pages/merchant/settings/applyment/submit/index.ts`
  - `weapp/miniprogram/pages/merchant/settings/applyment/submit/index.wxml`
- 验证结果：
  - RED：`PATH="/usr/local/go/bin:$PATH" go test ./api -run 'TestMerchantBindBankAPI/BadRequest_(MissingContactEmailDoesNotFallbackToPlatformEmail|InvalidContactEmailReturnsUserFacingMessage)'` 先失败，确认后端仍会用平台邮箱兜底，且非法邮箱会暴露 validator 原始字段。
  - GREEN：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'TestMerchantBindBankAPI/(OK_WithOrdinaryServiceProviderClient|BadRequest_MissingContactEmailDoesNotFallbackToPlatformEmail|BadRequest_InvalidContactEmailReturnsUserFacingMessage)'` 通过。
  - 阶段后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./logic -run 'ApplymentSubmission|ContactEmail|Ordinary'` 通过。
  - 阶段后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'MerchantBindBank|MerchantApplyment|ContactEmail|Applyment'` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过。
- Review 结果：通过。商户开户提交页已要求填写“管理员邮箱”，表单在提交前校验必填和基础邮箱格式；提交 payload 透传 `contact_email`；后端拒绝缺失或非法邮箱并返回中文业务文案；普通服务商进件不再使用 `WECHAT_ORDINARY_APPLYMENT_CONTACT_EMAIL` 或平台管理员邮箱兜底。


### 阶段 5

- 状态：已完成
- 修改文件：
  - `weapp/miniprogram/api/merchant-applyment.ts`
  - `weapp/miniprogram/services/merchant-applyment-workflow.ts`
- 验证结果：
  - RED：`! rg -n "needsOpenConfirmation|account_confirmation|isAccountAuthorized|accountAuthorizeState|needsConfirmation|开户意愿|微信确认|待确认" weapp/miniprogram/api/merchant-applyment.ts weapp/miniprogram/services/merchant-applyment-workflow.ts` 先失败，确认小程序 ViewModel 仍存在开户意愿授权/微信确认语义。
  - GREEN：上述静态不变量检查通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过。
- Review 结果：通过。`finish` 状态不再依赖 `account_authorize_state` 或 `AUTHORIZE_STATE_AUTHORIZED` 才展示开通；`to_be_confirmed` 被解释为账户验证；workflow 中移除 `awaiting_confirmation` 阶段和 `account_confirmation` 任务，不再出现普通服务商开户意愿确认入口或微信确认文案。


### 阶段 6

- 状态：已完成
- 修改文件：
  - `weapp/miniprogram/pages/merchant/settings/applyment/submit/index.ts`
- 验证结果：
  - RED：`rg -n "APPLYMENT_SUBMIT_STATUS_POLL|pollSubmittedApplymentStatus|resolveSubmittedWorkflowViewAfterPolling" weapp/miniprogram/pages/merchant/settings/applyment/submit/index.ts` 先失败，确认提交页没有提交后短时自动等待/轮询。
  - GREEN：`rg -n "APPLYMENT_SUBMIT_STATUS_POLL|pollSubmittedApplymentStatus|showSubmitSyncingToast" weapp/miniprogram/pages/merchant/settings/applyment/submit/index.ts` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过。
- Review 结果：通过。提交申请单成功后，小程序维持长 loading 并以 3 秒间隔轮询开户状态，最多等待 27 秒；一旦状态进入微信待办阶段，自动跳转待办页；如果仍在审核/处理中，则提示“平台正在同步微信待办”并回到开户首页，不再把用户直接丢回页面要求手动刷新。


### 阶段 7

- 状态：已完成
- 修改文件：
  - `weapp/miniprogram/pages/merchant/settings/applyment/action/index.ts`
  - `weapp/miniprogram/pages/merchant/settings/applyment/action/index.wxml`
  - `weapp/miniprogram/pages/merchant/settings/applyment/action/index.wxss`
- 验证结果：
  - RED：`rg -n "actionTaskView|buildApplymentActionTaskView|微信侧待办|完成后刷新状态" weapp/miniprogram/pages/merchant/settings/applyment/action/index.ts weapp/miniprogram/pages/merchant/settings/applyment/action/index.wxml` 先失败，确认待办页缺少统一普通服务商微信侧待办 ViewModel 和收口动作。
  - GREEN：上述静态检查通过。
  - 防回归检查：`! rg -n "开户意愿|微信确认|待确认|account_confirmation|needsConfirmation|isAccountAuthorized|accountAuthorizeState" weapp/miniprogram/pages/merchant/settings/applyment/action weapp/miniprogram/services/merchant-applyment-workflow.ts weapp/miniprogram/api/merchant-applyment.ts` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过。
- Review 结果：通过。微信待办页增加普通服务商“微信侧待办” ViewModel，按签约、法人扫码验证、账户验证给出不同标题、说明、二维码标题、保存按钮和处理步骤；账户验证信息继续在同页承接复制动作；页底保留“完成后刷新状态”和“返回开户首页”；页面和 workflow/API 防回归检查均未出现开户意愿确认或渠道商授权语义。


### 阶段 8

- 状态：已完成
- 修改文件：
  - `weapp/miniprogram/pages/merchant/settings/applyment/index.ts`
  - `weapp/miniprogram/pages/merchant/settings/applyment/index.wxml`
- 验证结果：
  - RED：`rg -n "openedSummary|微信支付商户号|后续说明|资金管理|onOpenSettlementAccountPage" weapp/miniprogram/pages/merchant/settings/applyment/index.ts weapp/miniprogram/pages/merchant/settings/applyment/index.wxml` 先失败，确认开户首页终态缺少完成摘要、微信支付商户号、后续说明和稳定的结算账户入口。
  - GREEN：上述静态检查通过。
  - 防回归检查：`! rg -n "开户意愿|微信确认|待确认|account_confirmation|needsConfirmation|isAccountAuthorized|accountAuthorizeState|授权未完成" weapp/miniprogram/pages/merchant/settings/applyment/index.ts weapp/miniprogram/pages/merchant/settings/applyment/index.wxml weapp/miniprogram/api/merchant-applyment.ts weapp/miniprogram/services/merchant-applyment-workflow.ts` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过。
  - 阶段小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过。
- Review 结果：通过。开户首页在 `opened` 终态展示“微信支付已开通”、微信支付商户号、后续说明和资金管理边界；结算账户卡片和页面级按钮均可进入结算账户页；页面不展示授权未完成、开户意愿确认或渠道商授权语义。


### 阶段 9

- 状态：已完成
- 修改文件：
  - `.github/standards/domains/wechat-payment/README.md`
  - `locallife/docs/docs.go`
  - `locallife/docs/swagger.json`
  - `locallife/docs/swagger.yaml`
  - `artifacts/wechat-payment/ordinary-service-provider-applyment-flow-implementation.md`
- 验证结果：
  - 文档 GREEN：`rg -n "普通服务商开户流程的项目内完成态规则|商户小程序开户用户流程摘要|APPLYMENT_STATE_FINISHED|AUTHORIZE_STATE_AUTHORIZED" .github/standards/domains/wechat-payment/README.md artifacts/wechat-payment/ordinary-service-provider-applyment-flow-implementation.md` 通过。
  - 防回归检查：`! rg -n "开户意愿确认.*(完成门槛|入口)|AUTHORIZE_STATE_AUTHORIZED.*(完成门槛|等待)" locallife/api locallife/logic locallife/worker locallife/wechat/ordinaryserviceprovider weapp/miniprogram/api/merchant-applyment.ts weapp/miniprogram/services/merchant-applyment-workflow.ts weapp/miniprogram/pages/merchant/settings/applyment` 通过。
  - 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./logic -run 'ApplymentSubmission|ContactEmail|Ordinary|PaymentFact|ProfitSharingReceiver|ReplaceOrder'` 通过。
  - 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'MerchantBindBank|MerchantApplyment|ContactEmail|Applyment'` 通过。
  - 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./db/sqlc -run 'ApplymentSubMchActivation|Notification|Applyment'` 通过。
  - 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./worker -run 'ApplymentRecovery|Applyment'` 通过。
  - 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./wechat/ordinaryserviceprovider/...` 通过。
  - 小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run lint` 通过。
  - 小程序验证：`PATH="$HOME/.local/bin:$PATH" npm run compile` 通过。
  - 小程序全量门禁：`PATH="$HOME/.local/bin:$PATH" npm run quality:check` 通过。
  - Prompt governance：`PATH="$HOME/.local/bin:$PATH" node .github/scripts/prompt_governance_lint.js` 通过。
  - Prompt routing：`PATH="$HOME/.local/bin:$PATH" node .github/scripts/prompt_routing_test.js` 通过。
  - 生成物检查：`PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-generated` 首次发现 Swagger 生成物未同步；保留生成后的 `docs.go`、`swagger.json`、`swagger.yaml` 后重跑通过。
- Review 结果：通过。微信支付域 README 已沉淀普通服务商开户完成态规则、邮箱必填、小程序开户流程、微信待办和终态页边界；Swagger 生成物同步了 `account_authorize_state` 兼容字段说明；后端、小程序、prompt governance 和生成物检查均已验证。


## 后续修复计划：开户完成激活与可营业状态残留

### 线上报错根因

报错：

```text
activate applyment: update merchant payment config: no rows in result set
```

真实路径：

1. `payment:process_fact_application` 任务调用 `PaymentFactService.ApplyExternalPaymentFactApplication`。
2. 进件事实进入 `applyApplymentFact`，在 `finish + sub_mch_id` 时调用 `ApplymentSubMchActivationTx`。
3. `ApplymentSubMchActivationTx` 先写 `ecommerce_applyments.status=finish` 和 `ecommerce_applyments.sub_mch_id`。
4. 事务随后调用 `UpdateMerchantPaymentConfig`，要求 `merchant_payment_configs` 已存在该 `merchant_id` 的行。
5. 当前生产提交开户链路没有创建 `merchant_payment_configs` 初始行；代码库中 `CreateMerchantPaymentConfig` 除测试外没有生产调用。
6. 因此新开户商户在完成态激活时，`UPDATE merchant_payment_configs WHERE merchant_id=$1 RETURNING *` 找不到行，返回 `no rows in result set`，整个激活事务回滚。

根因：激活事务把 `merchant_payment_configs` 当成必然已存在的前置记录，但普通服务商提交链路没有创建该记录；`finish + sub_mch_id` 的激活 owner 应该能幂等创建或更新商户支付配置。

### 修复目标

- 普通服务商 `finish + sub_mch_id` 必须幂等完成本地激活：写进件终态、写支付配置子商户号、激活支付配置、更新商户状态为 `active`。
- 不再依赖 `AUTHORIZE_STATE_AUTHORIZED` 创建开户成功 outbox 或判断激活完成。
- 商户手动开店失败文案不得再出现“开户意愿授权”。
- 激活事务可安全重试，适配已有配置行和缺失配置行两种生产状态。

### 任务 1：支付配置激活改为幂等 upsert

**文件：**

- 修改：`locallife/db/query/merchant_payment_config.sql`
- 修改：`locallife/db/sqlc/tx_notification.go`
- 测试：`locallife/db/sqlc/tx_applyment_activation_test.go`
- 生成：`locallife/db/sqlc/merchant_payment_config.sql.go`
- 生成：`locallife/db/mock/store.go`

步骤：

1. 在 `tx_applyment_activation_test.go` 增加 RED：无 `merchant_payment_configs` 行时，`ApplymentSubMchActivationTx` 仍应创建配置、写入 `sub_mch_id`、设置 `status=active`、商户 `status=active`。
2. 运行：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./db/sqlc -run 'ApplymentSubMchActivation'`，期望新增测试先失败并复现 `no rows in result set`。
3. 在 `merchant_payment_config.sql` 增加 `UpsertMerchantPaymentConfig`，使用 `INSERT ... ON CONFLICT (merchant_id) DO UPDATE SET sub_mch_id=EXCLUDED.sub_mch_id, status=EXCLUDED.status, updated_at=now() RETURNING *`。
4. 运行：`PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make sqlc`。
5. 在 `ApplymentSubMchActivationTx` 中用 `UpsertMerchantPaymentConfig` 替代 `UpdateMerchantPaymentConfig`。
6. 重跑 db 聚焦测试。

### 任务 2：修正开户成功 outbox 激活判定

**文件：**

- 修改：`locallife/logic/payment_fact_application_service.go`
- 测试：`locallife/logic/payment_fact_application_service_test.go`

步骤：

1. 将 `TestPaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinishWithoutAuthorizationDoesNotActivate` 改为普通服务商无授权状态也创建 activated outbox。
2. 运行：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./logic -run 'PaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinish'`，期望先失败。
3. 在 `applyApplymentFact` 中，`finish + sub_mch_id` 且 `ApplymentSubMchActivationTx` 成功后，`Activated` 直接置为 `true`。
4. 保留 `AccountAuthorizeState` 只作为兼容字段，不作为普通服务商激活/outbox 门槛。
5. 重跑 logic 聚焦测试。

### 任务 3：修正商户开店失败文案

**文件：**

- 修改：`locallife/api/merchant.go`
- 测试：`locallife/api/merchant_status_test.go`

步骤：

1. 更新 `TestUpdateMerchantOpenStatus_RequireApplymentWhenOpen_InactivePaymentConfig`，断言响应不包含“开户意愿授权”，并包含“微信支付进件”和“结算账户配置”。
2. 运行：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'UpdateMerchantOpenStatus_RequireApplymentWhenOpen'`，期望先失败。
3. 将失败文案改为“普通服务商特约商户未激活，请先完成微信支付进件和结算账户配置后再开业”。
4. 重跑 API 聚焦测试。

### 任务 4：恢复链路验证

**文件：**

- 检查：`locallife/worker/applyment_recovery_scheduler.go`
- 检查：`locallife/worker/task_payment_domain_outbox.go`
- 测试：`locallife/worker/applyment_recovery_scheduler_test.go`
- 测试：`locallife/worker/task_payment_domain_outbox_test.go`

步骤：

1. 确认 `finish + sub_mch_id` 恢复查询会记录 applyment activated fact 并 enqueue `payment:process_fact_application`。
2. 确认 activated outbox 发送“微信支付开户成功”通知后会 `MarkEcommerceApplymentResultProcessed(... finish)`。
3. 运行：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./worker -run 'ApplymentRecovery|PaymentDomainOutbox|PaymentFactApplication'`。

### 收口验证

修复完成后运行：

```bash
cd locallife
PATH="/usr/local/go/bin:$PATH" go test -count=1 ./db/sqlc -run 'ApplymentSubMchActivation'
PATH="/usr/local/go/bin:$PATH" go test -count=1 ./logic -run 'PaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinish|ApplymentSubmission|ContactEmail|Ordinary'
PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'UpdateMerchantOpenStatus_RequireApplymentWhenOpen|MerchantBindBank|MerchantApplyment|ContactEmail|Applyment'
PATH="/usr/local/go/bin:$PATH" go test -count=1 ./worker -run 'ApplymentRecovery|PaymentDomainOutbox|PaymentFactApplication'
PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-generated

cd ../weapp
PATH="$HOME/.local/bin:$PATH" npm run quality:check
```

### 执行结果（2026-05-03）

- [x] 任务 1：已完成。新增无 `merchant_payment_configs` 行的事务回归测试；`ApplymentSubMchActivationTx` 改为 `UpsertMerchantPaymentConfig`，`finish + sub_mch_id` 可幂等创建或更新支付配置，并继续将 `merchants.status` 推进为 `active`。
- [x] 任务 2：已完成。普通服务商 `APPLYMENT_STATE_FINISHED + sub_mch_id` 的 activated outbox 不再依赖 `AUTHORIZE_STATE_AUTHORIZED`；`account_authorize_state` 只保留为兼容字段。
- [x] 任务 3：已完成。商户开店失败文案和结算账户未激活错误文案均移除“开户意愿授权”，只提示完成微信支付进件和结算账户配置。
- [x] 任务 4：已完成。恢复链路确认 `finish + sub_mch_id` 会记录 activated fact 并投递 `payment:process_fact_application`；activated outbox 发布成功后会将进件结果处理状态标记为 `finish`。

验证结果：

- RED：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./db/sqlc -run 'ApplymentSubMchActivation'` 先失败并复现 `update merchant payment config: no rows in result set`。
- RED：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./logic -run 'PaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinish'` 先失败，证明无授权状态未创建 activated outbox。
- RED：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'UpdateMerchantOpenStatus_RequireApplymentWhenOpen'` 先失败，证明响应仍包含“开户意愿授权”。
- 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./db/sqlc -run 'ApplymentSubMchActivation'` 通过。
- 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./logic -run 'PaymentFactServiceApplyExternalPaymentFactApplication_OrdinaryApplymentFinish|ApplymentSubmission|ContactEmail|Ordinary'` 通过。
- 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'UpdateMerchantOpenStatus_RequireApplymentWhenOpen|MerchantBindBank|MerchantApplyment|ContactEmail|Applyment'` 通过。
- 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./api -run 'UpdateMerchantOpenStatus_RequireApplymentWhenOpen|MerchantBindBank|MerchantApplyment|ContactEmail|Applyment|SettlementAccount'` 通过。
- 后端验证：`PATH="/usr/local/go/bin:$PATH" go test -count=1 ./worker -run 'ApplymentRecovery|PaymentDomainOutbox|PaymentFactApplication'` 通过。
- 生成物检查：`PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make check-generated` 通过。
- 小程序全量门禁：`PATH="$HOME/.local/bin:$PATH" npm run quality:check` 通过。

Review 结果：通过。线上报错根因已在激活事务 owner 层修复，不依赖提交链路预创建支付配置；开户完成事实、恢复任务、outbox 通知和商户开业前置校验均以普通服务商完成态 `finish + sub_mch_id` 为准，不再把渠道商开户意愿授权作为完成门槛。
