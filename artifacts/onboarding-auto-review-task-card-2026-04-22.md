# 入驻自动审核与证照治理任务卡

## 目标

把商户与骑手的“上传 -> OCR -> 自动审核 -> 生效证照 -> 到期提醒 -> 过期暂停 -> 换证复审 -> 恢复”收敛成一条高内聚、低耦合、可审计、可恢复的后端闭环。

这份任务卡不是需求说明书，而是实现拆解卡。

## 审查结论

当前设计方向总体正确，符合“按职责分层”的架构目标：

- 上传层只负责资产归属
- OCR 层只负责识别与 readiness
- review 层只负责业务判定
- credential governance 负责通过后的持续有效治理

从高内聚低耦合角度看，主方向成立。

但按“链路完整、恢复安全、生产健壮”口径，当前设计还有 3 个必须先收紧的点；这些不构成推翻设计的 blocker，但必须作为任务卡前置项处理，不能跳过。

## 必须先收紧的点

### 1. 恢复逻辑必须按暂停归属释放，不能盲目 unsuspend

当前文档已经提出“清除 document_expired 导致的暂停”，但如果直接复用现有 [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql#L93-L99) 和 [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql#L167-L175) 的无条件 `UnsuspendMerchantTakeout` / `UnsuspendRider`，会把其他流程占有的暂停一起清掉。

这在仓库里已经有反例约束：食安结案只能释放仍由食安占有的暂停位，见 [locallife/integration/food_safety_case_integration_test.go](locallife/integration/food_safety_case_integration_test.go#L253-L290) 和 /memories/repo/food-safety-case-resolution-owned-suspension-only.md。

结论：

- 证照过期恢复不能直接调用现有通用 unsuspend。
- 必须新增“按 reason owner 条件释放”的 SQL 或 tx helper。

### 2. 生效证照台账不能只停留在 JSONB 摘要

文档里允许 `active_document_summary` 或 `credential_ledger`，甚至提到首阶段可用 JSONB 低成本落地。这对展示摘要可以，但对生产扫描、唯一性和替换切换不够稳。

原因：

- 7 天提醒需要可索引的到期扫描
- 过期暂停需要批量幂等扫描
- 替换时需要稳定唯一约束，避免一主体同类型多条 active 生效行并存

结论：

- 生产 source of truth 应该是结构化表，而不是仅靠 JSONB 摘要。
- JSONB 只能作为缓存或响应投影，不能作为 scheduler 的主扫描输入。

### 3. 恢复条件必须按“证照矩阵完整满足”判断，不能只看单张新证

商户至少同时依赖营业执照和食品经营许可证；骑手至少依赖健康证，身份证是否纳入持续治理需要明确定矩阵。

如果只要某一张新证审核通过就自动恢复，会出现：

- 商户食品经营许可证仍过期，但营业执照刚换证通过，交易被错误恢复
- 骑手还有其他受治理证照未满足，但接单被错误恢复

结论：

- 恢复必须由“当前 required credential matrix 是否全部满足”决定。
- 不能由单个 review run 的局部成功直接触发恢复。

## 风险等级

- 该改造按后端治理口径应视为 `G3`

理由：

- 影响 OCR 高风险链路
- 影响商户交易能力与骑手接单能力
- 涉及 scheduler、worker、暂停/恢复、提醒、重复执行和恢复语义
- 一旦恢复或暂停边界做错，会造成生产误停或误放开

## 当前实现状态（2026-04-22）

当前已落地到仓库、并经过聚焦验证的内容：

- `onboarding_review_runs` 与 `credential_ledgers` schema、query、sqlc 生成物已落地。
- OCR worker 已开始在现有 OCR JSON 内回写结构化 `readiness`，submit 前置校验会优先消费 `readiness`，不再完全依赖 handler 现场猜字段是否可用。
- `OnboardingReviewService` 已接入商户/骑手 submit 路径，能持久化 `review_run` 与 `review_summary`。
- rider submit 已改为优先创建 `review_run` 并分发 `TaskOnboardingReview`；当 review service 或 distributor 不可用、或入队失败时，会同步 fallback 到 shared rider review owner。
- merchant submit 已改为优先创建 `review_run` 并分发 `TaskOnboardingReview`；当 review service 或 distributor 不可用、或入队失败时，会同步 fallback 到 shared merchant review owner。
- rider review 执行已收口到 `RiderOnboardingReviewService`，worker 与 sync fallback 共享同一套判定、审批/退回、review run 完成与 credential activation 逻辑。
- merchant review 执行已收口到 `MerchantOnboardingReviewService`，worker 与 sync fallback 共享同一套审批事务、review run 完成与 credential activation 逻辑。
- merchant 主链目前已具备 API submit、worker 消费、logic owner 三层聚焦测试，至少覆盖了 async success、enqueue fallback、worker merchant 审核消费、existing run 完成后 `review_summary` 回填，以及审批事务输入构造。
- 商户自动审核阻塞分支、商户/骑手自动通过分支，都会形成结构化 `review_summary`。
- `CredentialGovernanceService` 已接入审核通过路径，能把通过后的证照原子切换到 active ledger。
- 7 天到期提醒 scheduler 已落地，会扫描 active credential、发送通知并回写 `last_reminded_at`。
- 过期暂停 scheduler 已落地，会扫描已过期 active credential、按 `document_expired` 归属抢占暂停位并回写 ledger `suspended_at`。
- `CredentialGovernanceService` 已接入恢复门禁：仅当 required credential matrix 满足时，才会尝试按 `document_expired` owner 条件释放暂停，并回写 active ledger 的 `resumed_at`。
- 换证复审通过后的恢复通知已落地，会带上 subject/document/outcome 等治理上下文。
- 申请响应已投影 `review_summary` 与 `active_credentials` 摘要；测试环境默认关闭 credential governance service，避免历史 API 单测被额外 store 查询干扰。
- api 层旧的 rider 审核 helper 已删除，避免 handler/logic 双实现漂移；规则行为测试已迁到 logic owner。
- api 层旧的 merchant 审批 helper 已删除，避免 handler/logic 双实现漂移；商户审批主路径已切到 logic owner。

当前仍明确未完成、不能误判为闭环已完结的内容：

- OCR readiness 已开始持久化到现有 OCR JSON，但还没有独立 read model，review 层仍会从现有 OCR JSON 构造 activation 输入，所以 `D3` 还未完成。
- merchant submit gate 的纯文档规则判定（readiness、证照有效期、主体一致性、身份证匹配）已下沉到 logic；当前仍留在 API 的主要是原始 JSON 解码、food permit repair、地图反查地址匹配与库查询去重，因此 `D3` 还处于收口中而非已完成。
- credential governance 目前已覆盖 activation、response projection、7 天 reminder scheduler、expiry enforcement、owned release、required credential matrix restore gate 与恢复通知。
- `active_credentials` 投影当前只在 credential governance service 启用时返回；现有 API 单测默认关闭该 service，因此聚焦 API 验证覆盖的是“接口兼容性”，不是完整治理闭环。

## 范围边界

- 只处理商户与骑手入驻自动审核和证照治理闭环
- 首版只纳入：
  - merchant: `business_license`、`food_permit`
  - rider: `health_cert`
- rider `id_card` 是否纳入持续治理，本轮先作为显式决策项，不默认实现
- 首版商户“暂停交易功能”先对齐外卖交易面暂停，不顺手重写全部商户状态机
- 不在本轮处理微信进件回调、运营商合同到期、UI 视觉改造

## 任务卡

### A. 契约与不变量

- [x] A1. 固定 subject/document 治理矩阵
  - 明确 merchant 必需证照、rider 必需证照
  - 明确哪些证照只参与首次审核，哪些证照进入持续治理
  - 产出：文档内矩阵 + 常量定义草案
  - 验证：见 [artifacts/onboarding-auto-review-contract-2026-04-22.md](artifacts/onboarding-auto-review-contract-2026-04-22.md)

- [x] A2. 固定 readiness / review outcome / suspension reason_code 枚举
  - readiness: `ready | partial | unreadable | invalid | provider_failed`
  - review outcome: `approved | rejected | needs_resubmit | needs_manual`
  - suspension reason_code 至少包含 `document_expired`
  - 验证：见 [artifacts/onboarding-auto-review-contract-2026-04-22.md](artifacts/onboarding-auto-review-contract-2026-04-22.md)

- [x] A3. 固定暂停归属释放规则
  - 新增规则：只有暂停原因仍由 `document_expired` 占有时，换证后才允许自动释放
  - 不允许清除食安、追偿、人工合规等其他原因占有的暂停位
  - 验证：见 [artifacts/onboarding-auto-review-contract-2026-04-22.md](artifacts/onboarding-auto-review-contract-2026-04-22.md)

- [x] A4. 固定恢复判定规则
  - 以“required credential matrix 全部满足”为唯一恢复条件
  - 不允许单个 review run 局部成功直接恢复
  - 验证：见 [artifacts/onboarding-auto-review-contract-2026-04-22.md](artifacts/onboarding-auto-review-contract-2026-04-22.md)

### B. 数据契约与结构

- [x] B1. 设计 `review_runs` 或等价持久化结构
  - 持久化 stage、outcome、reason_code、rule_hits、ocr_job_refs、snapshot version
  - 验证：已落地 `onboarding_review_runs`，见 [locallife/db/migration/000216_create_onboarding_review_and_credential_ledgers.up.sql](locallife/db/migration/000216_create_onboarding_review_and_credential_ledgers.up.sql#L10)

- [x] B2. 设计 `credential_ledgers` 结构化表
  - 至少包含 subject、document_type、source_application_id、media_asset_id、expires_at、active、last_reminded_at、suspended_at、resumed_at
  - 对 `(subject_type, subject_id, document_type, active=true)` 建唯一约束
  - 验证：已落地 `credential_ledgers` 与 active 唯一索引，见 [locallife/db/migration/000216_create_onboarding_review_and_credential_ledgers.up.sql](locallife/db/migration/000216_create_onboarding_review_and_credential_ledgers.up.sql#L62)

- [x] B3. 设计 expiring / expired / owned-release 查询契约
  - `ListCredentialsForReminderWindow`
  - `ListExpiredActiveCredentials`
  - `ReleaseMerchantTakeoutSuspensionIfOwned`
  - `ReleaseRiderSuspensionIfOwned`
  - 验证：已落地 query source，见 [locallife/db/query/credential_ledger.sql](locallife/db/query/credential_ledger.sql#L86) 和 [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql#L116)

- [x] B4. 设计 response projection 契约
  - 申请响应暴露 review_summary
  - 运营或商户/骑手侧可读取当前生效证照摘要
  - 验证：契约已冻结，申请查询已接出 `review_summary` 列，见 [artifacts/onboarding-auto-review-contract-2026-04-22.md](artifacts/onboarding-auto-review-contract-2026-04-22.md#L267)

### C. 数据库与生成物

- [x] C1. 新增 migration：review run 持久化
  - 验证：migrate up/down 可通过，约束完整

- [x] C2. 新增 migration：credential ledger + 索引 + 唯一约束
  - 验证：支持 expires_at 扫描和 active 唯一性

- [x] C3. 新增 db/query SQL
  - review run 写入/查询
  - credential ledger 创建/切换/扫描
  - owned release SQL
  - 验证：见 [locallife/db/query/onboarding_review.sql](locallife/db/query/onboarding_review.sql#L1)、[locallife/db/query/credential_ledger.sql](locallife/db/query/credential_ledger.sql#L1)、[locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql#L76)

- [x] C4. 运行 `make sqlc`
  - 验证：已通过 `make sqlc`，并同步更新 mock 生成物

### D. 领域结构与边界

- [x] D1. 新建 onboarding review 领域 owner
  - rider 已有 `RiderOnboardingReviewService`，merchant 已有 `MerchantOnboardingReviewService`；提交后的审批执行、review run 持久化与 credential activation 已收口到 logic owner
  - 不再把完整审核逻辑压在 API handler 中
  - 验证：handler 只负责请求校验和触发，不直接拍板

- [x] D2. 新建 credential governance 领域 owner
  - 统一承接生效证照投影、提醒、过期暂停、恢复
  - 不把 reminder / enforcement / activation 分散到多个无关包
  - 验证：activation、active credential summary、7 天 reminder、expiry enforcement、restore gate 已收口到治理主链

- [x] D3. 划清 OCR 与 review 边界
  - OCR worker 只产出 normalized fields + readiness
  - review worker 不直接解析 provider 原文
  - merchant submit gate 的原始 OCR JSON 解码与 food permit repair 已从 API handler 收口到 logic payload helper；submit gate 继续保留地图与库侧风险检查
  - 验证：review 输入由 logic payload helper 直接从持久化 OCR payload 构造，已通过 `go test ./logic -run '^Test(BuildMerchantDocumentReviewInputFromPayloads_RepairsFoodPermitFromRawText|RepairMerchantFoodPermitFromNormalized_UsesNormalizedResult|EvaluateMerchantDocumentReview_UsesBusinessLicenseReadiness|EvaluateMerchantDocumentReview_UsesFoodPermitProviderFailure)$'` 与 `go test ./api -run '^Test(CheckMerchantApplicationApproval_(UsesBusinessLicenseReadiness|UsesFoodPermitProviderFailure)|SubmitMerchantApplication_(Approved_FoodPermitRawTextFallback|Approved_FoodPermitOCRJobNormalizedFallback|BadRequest_FoodPermitNameUnreadable))$'`

### E. 接口与逻辑实现

- [x] E1. 实现 OCR readiness 回写
  - worker 回写字段级 state 与 readiness
  - 验证：OCR succeeded but partial / provider_failed 已有明确持久化形态，submit 前置校验会优先消费 readiness

- [x] E2. 实现 review run 创建与执行逻辑
  - rider 与 merchant 均已支持提交时创建 run，并由 review worker 或 sync fallback 执行；其中 merchant 当前由 submit gate 先完成自动通过前置判定，再交给 shared review owner 执行审批事务
  - 验证：同一申请可重跑，有审计轨迹

- [x] E3. 实现 approval -> credential activation 投影
  - 审核通过后把当前证照切换到 ledger active
  - 验证：旧 active 会被安全失活，新 active 原子生效

- [x] E4. 实现 credential reminder scheduler
  - 每日扫描 7 天内到期的 active credential
  - 发通知并回写 `last_reminded_at`
  - 验证：聚焦测试已覆盖发通知、fallback 落库与同一窗口去重，见 [locallife/scheduler/credential_governance_scheduler_test.go](locallife/scheduler/credential_governance_scheduler_test.go)

- [x] E5. 实现 credential expiry enforcement scheduler
  - 扫描已过期且未替换的 active credential
  - merchant 调用暂停交易能力
  - rider 调用暂停接单能力
  - 验证：聚焦测试已覆盖 merchant/rider claim suspension、ledger 回写与已暂停记录跳过，见 [locallife/scheduler/credential_governance_scheduler_test.go](locallife/scheduler/credential_governance_scheduler_test.go)

- [x] E6. 实现 owned release 恢复逻辑
  - 新证照复审通过后，仅在 `document_expired` 仍占有暂停位时释放
  - 验证：不会清掉其他原因的暂停

- [x] E7. 实现 required credential matrix 恢复门禁
  - 商户两类证照全部满足后才恢复
  - 骑手治理范围内证照全部满足后才恢复
  - 验证：局部换证不会提前恢复

- [x] E8. 实现通知文案与 related metadata
  - [x] 到期前 7 日提醒
  - [x] 已过期暂停通知
  - [x] 换证复审通过恢复通知
  - 验证：reminder / suspension / restore 通知均已带治理上下文，见 [locallife/scheduler/credential_governance_scheduler_test.go](locallife/scheduler/credential_governance_scheduler_test.go) 与 [locallife/api/onboarding_review_notification_test.go](locallife/api/onboarding_review_notification_test.go)

### F. API 与投影

- [x] F1. 调整提交接口，只触发 review run，不直接执行完整同步审核
  - rider 与 merchant submit 均已优先触发 review run + enqueue，并在异步边界不可用时同步 fallback
  - 验证：handler 明显变薄

- [x] F2. 调整申请查询接口，返回 review_summary
  - 验证：前端可区分 OCR 问题、补传问题、人工复核问题

- [x] F3. 新增当前生效证照摘要接口或字段投影
  - 验证：可直接展示“距离到期天数 / 是否已暂停 / 最近提醒时间”

### G. 测试

- [x] G1. 契约单测
  - readiness 枚举
  - outcome 枚举
  - required credential matrix 判定
  - 验证：已补 worker readiness 枚举契约测试与 onboarding review summary outcome 契约测试，并复核 required credential matrix 判定测试；`go test ./worker -run '^Test(BuildOCRReadiness_ReadyAndPartial|FailedOCRReadiness_DefaultsProviderError)$'`、`go test ./logic -run '^Test(BuildOnboardingReviewSummaryJSON_PreservesManualOutcomeContract|CredentialGovernanceServiceRestoreMerchantIfEligible_BlockedByMatrix|CredentialGovernanceServiceRestoreRiderIfEligible_BlockedByExpiredCredential)$'` 通过

- [x] G2. SQL / sqlc 测试
  - active 唯一性
  - expiring / expired 扫描
  - owned release
  - 验证：已补 `credential_ledger` db/sqlc 聚焦测试，覆盖 merchant active 唯一性、reminder/expired 扫描过滤、merchant/rider owned release；`go test ./db/sqlc -run '^Test(CreateMerchantCredentialLedger_RejectsDuplicateActiveDocument|CredentialLedgerScans_FilterByWindowAndActiveStatus|ReleaseMerchantTakeoutSuspensionIfOwned|ReleaseRiderSuspensionIfOwned)$'` 通过

- [x] G3. worker 单测
  - OCR succeeded but partial 回写
  - review run 执行
  - activation 投影
  - 当前已覆盖 merchant business license OCR 的 partial readiness 回写，以及 rider/merchant onboarding review worker 的 review run 执行和 merchant activation/restore 投影
  - 验证：相关 `go test ./worker` 聚焦用例通过

- [x] G4. scheduler 单测
  - 7 天提醒去重
  - 过期暂停幂等
  - 恢复后不再重复暂停
  - 验证：已有 reminder 去重、expired suspension 幂等与分页测试，另补恢复后不重复暂停显式用例；`go test ./scheduler -run '^TestDataCleanupScheduler_(RemindExpiringCredentials_SkipsAlreadyRemindedWindow|EnforceExpiredCredentials_ClaimsSuspensionsAndPublishesAlert|EnforceExpiredCredentials_DoesNotResuspendRestoredLedger)$'` 通过

- [x] G5. 集成测试：恢复不能误清其他暂停
  - 商户处于 manual compliance hold 时，换证通过不能解除该暂停
  - 验证：已补 real store + `CredentialGovernanceService` 集成用例，matrix 满足但 `manual compliance hold` 仍占有暂停位时不释放；`go test -count=1 -p 1 ./integration -run '^TestMerchantCredentialRestoreIntegration_DoesNotReleaseManualComplianceHold$'` 通过

- [x] G6. 集成测试：商户局部换证不得提前恢复
  - 仅更新营业执照但食品经营许可证仍过期时，交易保持暂停
  - 验证：已补 real store + `CredentialGovernanceService` + `/v1/orders` 集成用例，food permit 仍过期时 restore 不释放 `document_expired` 暂停位，外卖下单继续返回 403；`go test -count=1 -p 1 ./integration -run '^TestMerchantCredentialRestoreIntegration_PartialMerchantRenewalKeepsTakeoutBlocked$'` 通过

- [x] G7. 集成测试：骑手换证恢复闭环
  - 健康证过期后暂停接单
  - 新健康证复审通过后自动恢复
  - 验证：已补 real store + `CredentialGovernanceService` + `/v1/delivery/grab/:order_id` 集成用例，健康证续期后 rider suspension 自动释放，抓单接口恢复可用；`go test -count=1 -p 1 ./integration -run '^TestRiderCredentialRestoreIntegration_RestoresGrabAvailability$'` 通过

- [x] G8. 集成测试：完整闭环
  - 首次审核通过
  - 7 天提醒
  - 到期暂停
  - 换证上传
  - OCR / review
  - 台账切换
  - owned release 恢复
  - 验证：已补 rider 全链路集成用例，覆盖首次审核激活、7 天提醒、到期暂停、续证激活、台账切换、owned release 恢复与 /v1/delivery/grab 恢复；`go test -count=1 -p 1 ./integration -run '^TestRiderCredentialGovernanceIntegration_ClosedLoop$'` 通过

## 推荐实现顺序

- [ ] P1. 先完成 A1-A4
- [ ] P2. 再完成 B1-B4 与 C1-C4
- [ ] P3. 再完成 D1-D3 与 E1-E3
- [ ] P4. 再完成 E4-E8
- [ ] P5. 最后完成 F1-F3 与 G1-G8

## 每阶段验证要求

- A/B/C 完成后：至少更新任务卡与契约文档，不允许边做边改枚举含义
- C 完成后：运行 `make sqlc`
- D/E/F 完成后：运行最小相关包级测试
- G 完成后：至少执行相关包级测试和关键集成测试

## 当前状态

- 当前状态：P1-P4 与 F1-F3 主链已落地，进入 G 聚焦验证补强与 D3 边界继续收口阶段
- 已冻结契约稿：见 [artifacts/onboarding-auto-review-contract-2026-04-22.md](artifacts/onboarding-auto-review-contract-2026-04-22.md)
- 已落库源变更：见 [locallife/db/migration/000216_create_onboarding_review_and_credential_ledgers.up.sql](locallife/db/migration/000216_create_onboarding_review_and_credential_ledgers.up.sql#L1)
- 执行规则：实现一项，勾选一项；如果某项前置未完成，不允许跳项报完成