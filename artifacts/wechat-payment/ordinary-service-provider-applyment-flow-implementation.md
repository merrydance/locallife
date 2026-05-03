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
