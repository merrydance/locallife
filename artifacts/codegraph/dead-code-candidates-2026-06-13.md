# 死代码候选核对清单

日期：2026-06-13

这份清单只做定位和复核，不删除任何代码。

## 核对方法

- CodeGraph：读取 `/home/sam/locallife/.codegraph/codegraph.db`，索引为最新
- 文本核对：用 `rg` 核对生产 Go 源码、测试源码和 Swagger 生成物中的引用
- 静态检查：`staticcheck -checks=U1000,SA4006,SA4011 ./...`
- 基础验证：`/usr/local/go/bin/go vet ./...`
- 范围：`locallife/**/*.go`，重点看生产源码里的私有符号

## 验证结果

- 早前基线 `go vet ./...`：通过。
- 本轮 fresh `go vet ./...`：未通过，原因是工作区已有的 `locallife/db/sqlc/tx_rider_refund_test.go` 修改尚未与当前接口对齐，`PrepareRiderDepositRefundTxParams` / `PrepareRiderDepositRefundTxResult` 字段不匹配；这不是本次报告更新引入的生产代码变化。
- `staticcheck -checks=U1000,SA4006,SA4011 ./...`：继续确认未使用代码和 `SA4006` 信号；本轮 fresh 运行也先报同一个 `tx_rider_refund_test.go` 编译错误。
- `/usr/local/go/bin/go test ./...`：本次为了确认现状跑过，但失败在既有 DB / integration 测试状态：
  - `db/sqlc`: `TestDeactivateStaleMerchantAppDevices` 期望 `1`，实际 `3`
  - `integration`: `TestClaimJourneyD39RiderRecoveryDisputeListPaginationIntegration` 返回 `404`，期望 `201`
  - `integration`: `TestTakeoutJourneyB3Integration` 触发 `baofu_account_bindings_opening_mode_account_type_check`
- 本次没有改生产代码，所以这些验证失败不来自本次文档更新。

## 分类说明

- `可优先清理候选`：生产源码里没有调用方；用途明确但当前未接入。删除前仍要做一次小范围编译/测试。
- `已清理`：已按本报告逐项复核、删除，并记录对应验证。
- `整组遗留候选`：一组同属一个未接入功能的类型/函数，适合按功能成组确认。
- `测试仍在引用`：生产路径没用，但测试还在用；若删除，需要同步删/改测试。
- `Swagger-only`：Go 编译视角未使用，但被 Swagger 注释或生成文档引用；不应按普通死代码直接删。
- `暂不作为死代码`：静态工具有噪声，或它是被同文件内部私有函数调用的符号，不能单独删除。

## 已清理

| 位置 | 符号 | 原作用 | 复核与验证 |
| --- | --- | --- | --- |
| `locallife/api/server.go:1815` | `attachedServerError` | 将错误挂到 Gin context，同时返回公开错误文案 | 生产和测试均无调用；常用路径使用 `internalError` / `loggedServerError`。已删除；`go build ./api` 通过。`go test ./api` 被当前工作区已有的 `api/rider_test.go` WIP 编译错误挡住，未作为本项通过依据 |
| `locallife/api/baofu_settlement_account_profile_defaults_merge.go:410` | `pgInt8Value` | 把 `pgtype.Int8` 转成 `int64`，原本用于结算账户默认值响应或合并 | 生产和测试均无调用。已删除，同时移除随之未使用的 `pgtype` import；`go build ./api` 通过。`go test ./api` 仍被当前工作区已有的 `api/rider_test.go` WIP 编译错误挡住，未作为本项通过依据 |
| `locallife/api/merchant_application.go:1957` | `parseFlexibleDate` | 解析中文、点分隔、纯数字、横线分隔的营业执照日期字符串 | 生产和测试均无调用；OCR 日期解析逻辑已转移到 worker/helper。已删除；`go build ./api` 通过。`go test ./api` 仍被当前工作区已有的 `api/rider_test.go` WIP 编译错误挡住，未作为本项通过依据 |
| `locallife/logic/payment_order_service.go:427` | `subMchIDFromPaymentAttach` | 从支付单 attach 里解析 `sub_mchid` | 生产和测试均无调用。已删除；`parsePaymentAttach` 仍被预订待支付单复用判断使用，保留。`go build ./logic` 通过。`go test ./logic -run 'TestPaymentOrderService|TestCreateBaofuPaymentOrder|TestCreatePaymentOrder|TestClosePaymentOrder' -count=1` 被当前工作区已有的 `logic/rider_deposit_refund_service_test.go` WIP 编译错误挡住，未作为本项通过依据 |
| `locallife/logic/payment_order_service.go:466` | `shouldEnableOrderProfitSharing` | 按订单类型判断是否启用订单分账 | 生产和测试均无调用；worker 里有另一套 `shouldDispatchOrderProfitSharing` 测试覆盖。已删除；`orderTypeDineIn` / `orderTypeTakeaway` 仍被同包其他文件和测试使用，保留。`go build ./logic` 通过。`go test ./logic -run 'TestPaymentOrderService|TestCreateBaofuPaymentOrder|TestCreatePaymentOrder|TestClosePaymentOrder' -count=1` 被当前工作区已有的 `logic/rider_deposit_refund_service_test.go` WIP 编译错误挡住，未作为本项通过依据 |
| `locallife/worker/task_process_payment.go:1544` | `workerStringValue` | 把 `*string` 转成空字符串兜底 | 生产和测试均无调用。已删除；相邻 `workerStringPtrIfNotEmpty` 仍被错误字段映射使用，保留。`go build ./worker` 通过；`go test ./worker -run 'TestWorkerPaymentCommandErrorFields|TestShouldDispatchOrderProfitSharing' -count=1` 通过 |
| `locallife/worker/task_process_payment.go:1619` | `workerProfitSharingCommandSnapshot` | 过滤空字段后序列化分账 command snapshot | 生产和测试均无调用。已删除；相邻 `workerBaofuRefundCommandSnapshot` 仍在退款命令记录路径使用，保留。`go build ./worker` 通过；`go test ./worker -run 'TestWorkerPaymentCommandErrorFields|TestShouldDispatchOrderProfitSharing' -count=1` 通过 |
| `locallife/api/merchant_application.go:35` | `loggingReader` / `Read` / `Close` | 包装上传 body，按读取进度记录商户 OCR 上传日志 | 类型和方法生产、测试均无调用；上传链路已不使用这个 wrapper。已删除，同时移除随之未使用的 `io` import；`go build ./api` 通过；`go test ./api -run '^$' -count=1` 通过 |
| `locallife/api/applyment_contact_info.go:21` | `pgTextValue` | 把 `pgtype.Text` 转成普通字符串，用于取用户手机号兜底 | 只被同文件未使用的 `resolveApplymentContactPhone` 调用。已随上层 helper 一起删除；`firstNonEmptyTrimmed` 仍被 `baofu_callback.go` 使用，保留。`go build ./api` 通过；`go test ./api -run '^$' -count=1` 通过 |
| `locallife/api/applyment_contact_info.go:28` | `Server.resolveApplymentContactPhone` | 商户进件联系人手机号解析：候选值优先，否则从用户手机号兜底 | 生产和测试均无调用；当前商户申请提交路径在 `merchant_application.go` 自行从用户手机号兜底。已删除；`go build ./api` 通过；`go test ./api -run '^$' -count=1` 通过 |
| `locallife/api/payment_order.go:1127` | `applyAbnormalRefundURIRequest` | 异常退款申请 URI 参数 DTO | 未见 handler 绑定、路由注册、Swagger 注释或 `docs` 生成物引用。已删除；`go build ./api` 通过；`go test ./api -run '^$' -count=1` 通过 |
| `locallife/api/payment_order.go:1131` | `applyAbnormalRefundBodyRequest` | 异常退款申请 body DTO | 未见 handler 绑定、路由注册、Swagger 注释或 `docs` 生成物引用。已删除；`go build ./api` 通过；`go test ./api -run '^$' -count=1` 通过 |
| `locallife/api/payment_callback.go:78` | `Server.enqueueProfitSharingPaymentFactApplication` | 从支付回调触发分账 payment fact application task | 生产和测试均无调用；当前宝付分账 callback 路径在 `baofu_callback.go` 记录并派发 application。已删除；相邻的 direct payment、骑手押金退款、预订退款、订单退款 enqueue helper 仍在用，保留。`go build ./api` 通过；`go test ./api -run 'TestBaofuShareCallback|TestHandlePaymentNotify|TestHandleRefundNotify|TestHandleMerchantTransferNotify' -count=1` 通过 |
| `locallife/api/payment_callback.go:564` | `Server.requireTaskDistributorForNotification` | 回调处理前检查 task distributor，缺失时释放通知 claim 并返回失败 | 生产和测试均无调用；当前回调路径已在各自 handler 内直接处理缺失分支。已删除；`go build ./api` 通过；`go test ./api -run 'TestBaofuShareCallback|TestHandlePaymentNotify|TestHandleRefundNotify|TestHandleMerchantTransferNotify' -count=1` 通过 |
| `locallife/logic/combined_payment_service.go:15` | `combinedOutTradePrefix` | 原合单支付 out_trade_no 前缀 | 当前合单支付服务 fail-closed，没有创建路径使用。已删除；`go build ./logic` 通过；`go test ./api -run 'TestCreateCombinedPaymentOrderAPI_BaofuMainBusinessFailsClosed' -count=1` 通过 |
| `locallife/logic/combined_payment_service.go:16` | `combinedOrderMaxCount` | 原合单支付子订单数量上限 | 当前合单支付服务 fail-closed，没有创建路径使用。已删除；`go build ./logic` 通过；`go test ./api -run 'TestCreateCombinedPaymentOrderAPI_BaofuMainBusinessFailsClosed' -count=1` 通过 |
| `locallife/logic/baofu_payment_readiness.go:47` | `ensureMerchantBaofuReadyForPayment` | 单商户宝付支付 readiness 校验 wrapper | 只被同文件未接入的 `ensureCombinedPaymentMerchantsBaofuReady` 调用；生产支付创建路径直接使用 `merchantBaofuReadinessForPayment`，该函数仍在 `baofu_payment_order_route.go` 使用并保留。已删除；`go build ./logic` 通过；`go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder_RequiresMerchantBaofuReadiness|TestPaymentOrderServiceCreatePaymentOrder_BaofuWechatChannelNotReadyFailsBeforeClientCall|TestBaofuPaymentReadiness' -count=1` 通过 |
| `locallife/logic/baofu_payment_readiness.go:65` | `ensureCombinedPaymentMerchantsBaofuReady` | 创建合单支付前逐商户检查宝付账户和微信通道 readiness | 生产和测试均无调用；当前合单支付服务已 fail-closed，不进入合单创建 readiness 校验。已删除；底层 `merchantBaofuReadinessForPayment` 仍服务单商户宝付支付创建路径。`go build ./logic` 通过；`go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder_RequiresMerchantBaofuReadiness|TestPaymentOrderServiceCreatePaymentOrder_BaofuWechatChannelNotReadyFailsBeforeClientCall|TestBaofuPaymentReadiness' -count=1` 通过 |
| `locallife/logic/rider_onboarding_review_service.go:571` | `onboardingReviewRunID` | 把 rider onboarding review run 转为 `*int64` | 逻辑层生产和测试均无调用；API 层同名 helper 仍在商户/骑手申请提交入口使用并保留。已删除；`go build ./logic` 通过；`go test ./logic -run 'TestEvaluateRiderApplication|TestRiderOnboardingReviewServiceProcessSubmittedApplication_UsesDurableApprovalTx' -count=1` 通过 |
| `locallife/worker/task_payment_timeout.go:264` | `paymentTimeoutSubMchIDFromAttach` | 超时关闭支付单时从 attach 解析 `sub_mchid` | 生产和测试均无调用；当前宝付支付超时查询/关闭使用 collect merchant/terminal 配置与支付单号，不通过 attach 解析子商户号。已删除；`go build ./worker` 通过；`go test ./worker -run 'TestProcessTaskPaymentOrderTimeout|TestProcessTaskOrderPaymentTimeout_DelegatesPendingBaofuPaymentOrder' -count=1` 通过 |
| `locallife/worker/task_process_payment.go:97` | `withProfitSharingEnqueueDedup` / `profitSharingEnqueueDedupWindow` | 给分账 enqueue 追加 asynq unique 去重窗口 | 生产和测试均无调用；当前分账任务调度由 API/logic 调用方直接传入去重选项，分账结果通知仍保留 `profitSharingResultNotificationDedupWindow`。已删除；`go build ./worker` 通过；`go test ./worker -run 'TestProcessTaskBaofuProfitSharing|TestProcessTaskPaymentDomainOutbox_PublishesProfitSharingResultReady|TestProcessTaskPaymentDomainOutbox_PublishesRiderProfitSharingResultReady|TestWorkerPaymentCommandErrorFields|TestShouldDispatchOrderProfitSharing' -count=1` 通过 |
| `locallife/worker/order_payment_fact.go:36` | `recoveredOrderPaymentFactResource` / `orderPaymentInt8Value` | 为“已支付但未处理”恢复扫描构造泛用 payment fact 资源快照 JSON，并把 `pgtype.Int8` 转成 JSON 值 | 生产和测试均无调用；支付恢复调度器当前通过 `recordRecoveredDirectPaymentFact` 使用 direct-payment 专用 `recoveredDirectPaymentFactResource`，宝付支付恢复由专用宝付 scheduler 处理。已删除，同时移除只服务该 helper 的 `encoding/json` import；`go build ./worker` 通过；`go test ./worker -run 'TestPaymentRecoverySchedulerRunOnceCreatesRiderDepositPaymentFactApplication|TestPaymentRecoverySchedulerRunOnceCreatesClaimRecoveryPaymentFactApplication|TestProcessTaskPaymentOrderTimeout_DirectRemotePaidRecordsFactInsteadOfClosing' -count=1` 通过 |
| `locallife/logic/replace_order.go:332` | `markReplaceReservationPaymentOrderFailedForCleanup` | 替换预订支付失败后把支付单置为 failed | 生产和测试均无调用；当前替换预订正向支付创建失败直接返回错误，宝付预订支付终态失败由 payment fact 应用路径处理。已删除；`go build ./logic` 通过；`go test ./logic -run 'TestProcessReplaceOrderRefundWithBaofu|TestCreateReplaceOrderBaofuPayment|TestReplaceReservationOrderWithBaofu|TestReplaceReservationRefundCommandInputUsesBaofuProvider' -count=1` 通过 |
| `locallife/logic/baofu_account_onboarding_profile.go:168` | `BaofuAccountOnboardingService.getOrCreateFlow` | 为宝付开户 profile 获取或创建开户 flow 的旧 wrapper | 生产和测试均无调用；当前 `Start` 流程先解析 active flow，再直接调用 `getOrCreateFlowWithExisting`，该函数和 `createBaofuAccountOpeningFlow` 仍承载开户草稿复用、模式切换作废和新 flow 创建。已删除；`go build ./logic` 通过；`go test ./logic -run 'TestBaofuAccountOnboardingServiceStart_MerchantEmptyModeContinuesPersonalDraft|TestBaofuAccountOnboardingServiceStart_MerchantOpeningModeChangeVoidsDraftFlow|TestBaofuAccountOnboardingServiceStart_MerchantOpeningModeChangeRejectsProcessingFlow|TestBaofuAccountOnboardingServiceStart_ReplacementFlowReusesPaidVerifyFeeWithoutChargingAgain|TestBaofuAccountOnboardingServiceStartRecoversProviderProgressFlow' -count=1` 通过 |
| `locallife/logic/payment_order_service.go:273` | `PaymentOrderService.resolveConcurrentOrderPayment` | 并发创建订单支付单冲突时，轮询并复用或关闭已有待支付单 | 生产和测试均无调用；当前创建支付单路径已在创建前读取最新待支付单并按金额/渠道决定复用或关闭。已删除；`go build ./logic` 通过；`go test ./logic -run 'TestPaymentOrderServiceCreatePaymentOrder|TestPaymentOrderServiceCreateReservationPaymentRejectsOfflineOperatorAsCustomerOwner|TestPaymentOrderServiceClosePaymentOrder|TestPaymentOrderServiceGetPaymentOrder|TestPaymentOrderServiceListPaymentOrders' -count=1` 通过 |
| `locallife/logic/payment_order_service.go:303` | `PaymentOrderService.resolveConcurrentReservationPayment` | 并发创建预订支付单冲突时，按 attach 判断是否复用已有待支付单 | 生产和测试均无调用；当前预订支付复用判断由创建前的 `shouldReuseReservationPendingPayment` 承担，`parsePaymentAttach` 因此仍在用并保留。已删除；同上 build/test 通过 |
| `locallife/logic/payment_order_service.go:536` | `sleepWithContext` / 支撑重试常量 | 并发支付冲突轮询时的 context-aware sleep 和重试参数 | 只被已删除的并发解析 helper 调用；API 层仍有自己的 `sleepWithContext` 用于骑手押金 legacy out_trade_no 重试，不属于本项。已删除；同上 build/test 通过 |
| `locallife/logic/payment_order_service.go:547` | `PaymentOrderService.markPaymentOrderFailedForCleanup` | 预支付失败后把支付单标记为 failed 并记录日志 | 生产和测试均无调用；当前宝付创建失败路径会关闭本地 pending 支付单，宝付/预订支付终态失败由 payment fact 应用路径处理。已删除；同上 build/test 通过 |
| `locallife/logic/refund_service.go:58` | `RefundService.maybeMarkPaymentOrderRefunded` | 累计退款额达到支付金额后，把支付单置为 refunded | 生产和测试均无调用；当前退款结果终态由 worker legacy 路径和 `PaymentFactService.maybeMarkPaymentOrderRefunded` 处理，商户发起退款的同步阶段不直接终结支付单状态。已删除；`go build ./logic` 通过；`go test ./logic -run 'TestCreateRefundOrder|TestPaymentFactServiceApplyExternalPaymentFactApplication_.*Refund' -count=1` 通过 |
| `locallife/internal/wechatdoc/alignment_helpers.go` | `alignment_helpers.go` 整组旧对齐审计 helper | 微信官方文档 endpoint/字段/枚举/约束对齐审计的未接入实现 | 生产和测试均无调用；`cmd/wechat_doc_extract` 只使用 `ExtractMarkdownFile`，`cmd/doc_audit` 走 `internal/docaudit`，未接入这套 alignment helper。已删除整个文件；`go test ./internal/wechatdoc ./internal/docaudit ./cmd/wechat_doc_extract ./cmd/doc_audit -count=1` 通过 |
| `locallife/api/rider.go` | `listRidersRequest` / `listRidersResponse` / `Server.listRiders` | 旧版 `/v1/admin/riders` 管理员骑手列表 handler、查询参数和响应 DTO | 真实路由在 `server.go` 注册到 `listPlatformRiders`，旧 handler 生产和测试均无调用。已删除旧 handler，并把 `/v1/admin/riders` Swagger 注释迁移到真实 handler，使文档响应从旧 `listRidersResponse` 对齐为 `platformRiderListResponse`；`PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" make swagger` 通过 |
| `locallife/api/scan.go` | `buildMerchantStorefrontQRCodeObjectKey` / `buildMerchantStorefrontQRCodeScene` / `wxaCodeEnvVersion` / `Server.ensureMerchantStorefrontQRCode` | 旧版商户店铺小程序码生成和媒体资产保存 helper | 生产、测试、路由和 Swagger 均无调用；真实扫码/二维码入口只有 `/v1/scan/table` 和 `/v1/tables/{id}/qrcode`，桌台二维码生成路径独立保留。已删除店铺码整组 helper 和专属常量；不需要重新生成 Swagger |
| `locallife/worker/refund_recovery_scheduler.go` | `errRefundRecoverySubMchIDMissing` / `errRefundRecoveryMerchantUnresolved` / `RefundRecoveryScheduler.resolveSubMchID` | 旧版 refund recovery 根据订单或预订反查商户微信/宝付子商户号 | 生产和测试均无调用；当前 stuck refund status recovery 已按 payment channel 分流，直连微信用 `out_refund_no` 调 `DirectPaymentClientInterface.QueryRefund`，宝付用 collect merchant/terminal 配置调 `aggregatepay.QueryRefund`，不再需要按订单/预订解析商户 `sub_mchid`。已删除整组 helper 和专属错误；高风险退款恢复路径需跑 focused worker tests |
| `locallife/worker/order_profit_sharing_snapshot.go` | `wechatProfitSharingPaymentFeeRateBps` / `estimatedWechatProfitSharingPaymentFee` / `RedisTaskProcessor.ensureOrderProfitSharingSnapshot` | 旧版微信分账快照创建逻辑，按订单、商户、运营商和骑手估算分账并写入 `profit_sharing_orders` | 生产和测试均无调用；当前主业务分账由 `BaofuPaymentRecoveryScheduler.createReadyProfitSharingOrders` 调 `BaofuProfitSharingService.CreatePendingOrder` 创建宝付分账单，并由 `ProcessTaskBaofuProfitSharing` 派发命令。旧 helper 会创建 `Provider=wechat` 的历史快照，已退役。已删除整个文件；高风险分账路径需跑 focused worker tests |

## 可优先清理候选

| 位置 | 符号 | 作用 | 核对结论 |
| --- | --- | --- | --- |

## 整组遗留候选

暂无。已处理的整组项见“已清理”。

## 未注册 handler / 未接入入口候选

| 位置 | 符号 | 作用 | 核对结论 |
| --- | --- | --- | --- |
暂无。已处理的未接入口项见“已清理”。

## Swagger-only / 文档契约用途

| 位置 | 符号 | 作用 | 核对结论 |
| --- | --- | --- | --- |
| `locallife/api/merchant_application_ocr_correction.go:39` | `patchMerchantDocumentOCRFieldsRequest` | Swagger 文档里的 OCR 更正统一请求体 | 已复核并保留：真实路由 `/v1/merchant/application/documents/{document_type}/ocr-fields` 进入 `patchMerchantApplicationDocumentOCRFields` 后按 `business_license` / `food_permit` 分别绑定具体 DTO；该结构只作为多态请求体的 Swagger schema，被注释和生成文档引用，删除会丢失 API 文档契约 |

## 测试仍在引用，先别直接删

| 位置 | 符号 | 作用 | 核对结论 |
| --- | --- | --- | --- |
| `locallife/api/recovery_dispute.go:1805` | `deriveAutomaticRecoveryDisputeResolution` | 从追偿争议和行为决策推导自动处理结果 | 生产无调用，但 `recovery_dispute_test.go` 仍直接测它 |
| `locallife/baofu/client.go:454` | `publicBusinessFailure` | 解析宝付 public envelope 业务失败码/文案 | 生产无调用，但 `baofu/client_test.go` 仍直接测它；也可能是原本 public 接口失败分类残留 |
| `locallife/db/sqlc/tx_claim_behavior.go:135` | `behaviorDecisionScoreBreakdown` | 索赔行为决策评分详情 JSON 结构 | 生产无直接引用，但 `tx_claim_behavior_test.go` 的 `decodeScoreBreakdown` 直接用它校验评分 JSON 结构；若后续要清理，应先把测试解码结构移到 `_test.go` |
| `locallife/db/sqlc/tx_claim_behavior.go:143` | `behaviorDecisionScoreDetail` | 单项评分与命中信号结构 | 被上面的测试解码结构嵌套引用，随 score breakdown 成组保留 |
| `locallife/db/sqlc/tx_claim_behavior.go:148` | `behaviorDecisionSignal` | 评分信号 code/weight/count/active | 被上面的测试解码结构嵌套引用，随 score breakdown 成组保留 |
| `locallife/logic/promotion_engine.go:269` | `suggestBestVoucher` | 从 DB 查询用户可用券并挑选最优券 | 生产无调用；`suggestBestVoucherFromList` 才是生产使用核心，测试仍覆盖该 wrapper |
| `locallife/worker/baofu_alert_payloads.go:17` | `newBaofuPaymentCallbackMissingAlert` | 构造宝付支付回调缺失告警 payload | 生产无调用；`alert_payloads_test.go` 仍覆盖 |
| `locallife/worker/baofu_alert_payloads.go:34` | `newBaofuProfitSharingProcessingSLAAlert` | 构造宝付分账处理超时告警 payload | 同上 |
| `locallife/worker/baofu_alert_payloads.go:52` | `newBaofuWithdrawalProcessingSLAAlert` | 构造宝付提现处理超时告警 payload | 同上 |
| `locallife/worker/baofu_alert_payloads.go:72` | `newBaofuFailedFactAlert` | 构造宝付 payment fact 失败告警 payload | 同上 |
| `locallife/worker/baofu_alert_payloads.go:90` | `newBaofuFeeLedgerMismatchAlert` | 构造宝付手续费台账不一致告警 payload | 同上 |
| `locallife/worker/payment_channel_boundary.go:11` | `paymentOrderRequiresProfitSharing` | 包装 `db.PaymentOrderRequiresProfitSharing`，用于判断支付单是否需要分账 | 生产无调用；测试仍覆盖边界 |
| `locallife/worker/payment_channel_boundary.go:25` | `paymentOrderUsesMainBusinessRefundChannel` | 判断支付单是否应走主业务宝付退款通道 | 生产无调用；测试仍覆盖边界 |
| `locallife/worker/task_merchant_application_ocr.go:505` | `parseFoodPermitOCRText` | 食品经营许可证 OCR 主解析入口，调用 internal parser 并记录失败日志 | 生产改用 `parseFoodPermitOCRTextFallback` / internal parser；测试仍直接测旧入口 |
| `locallife/worker/task_process_payment.go:84` | `shouldDispatchOrderProfitSharing` | 判断支付成功后是否派发订单分账 | 生产无调用；`task_process_payment_internal_test.go` 仍覆盖 |
| `locallife/worker/task_process_payment.go:1551` | `workerPaymentCommandErrorFields` | 将微信/宝付错误转成 command 记录的 code/message | 生产无调用；测试仍覆盖错误映射 |

## 暂不作为死代码

| 位置 | 符号 | 作用 | 核对结论 |
| --- | --- | --- | --- |
暂无。已处理的高风险 worker 遗留项见“已清理”。

## 代码块级信号

| 位置 | 信号 | 作用 | 核对结论 |
| --- | --- | --- | --- |
| `locallife/api/merchant_finance.go:877` | `SA4006` | 统计带状态筛选的商户结算数量 | `err` 在分支中赋值后统一检查；不是死代码块，但 staticcheck 会提示可读性/风险 |
| `locallife/api/merchant_finance.go:898` | `SA4006` | 统计不带状态筛选的商户结算数量 | 同上 |

## 建议顺序

1. 纯孤立 helper、未接入口候选和已确认退役的高风险 worker 遗留项已清理。
2. 对 Swagger-only 的 OCR 更正请求体，保持为文档契约用途，不作为删除项。
3. 对“测试仍在引用”的项，先决定测试是否还代表有效业务规则；若规则已经迁移，应同步删除或改写测试。
4. 对 `merchant_finance.go` 的 `SA4006`，如果进入修复阶段，建议只做局部可读性调整，不当作死代码删除。
