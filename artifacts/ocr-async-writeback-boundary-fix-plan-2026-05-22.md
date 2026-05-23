# TASK-OCR-ASYNC-WRITEBACK-001：统一 OCR 异步写回边界改为先绑定后派发并校验陈旧任务

日期：2026-05-22
范围：仅 `locallife/` 后端。
风险等级：G3。原因：涉及 OCR 异步 worker、申请单状态机、私有媒体绑定、证件 OCR 写回、重复投递和失败恢复。

> 交接要求：这张卡必须能在上下文压缩、切换 agent 或完全失忆后单独执行。执行者先读本卡，再按“上下文重建清单”打开源文件，不要靠会话记忆补判断。

## 目标

OCR 任务只有在申请单 pending 绑定写入成功后才能派发到 worker；worker 在调用 OCR provider 和写回申请单前，必须确认任务仍然对应当前申请单的当前媒体资产、当前 OCR job 和可写状态。陈旧任务、被替换的图片、已提交/已审核申请单、mark pending 失败后的孤儿 job，都不能再触发 provider 调用或覆盖业务字段。

## 上下文重建清单

每次接手、压缩后恢复或切换上下文时，从仓库根目录 `/home/sam/locallife` 重新读：

1. `.github/README.md`
2. `.github/copilot-instructions.md`
3. `locallife/AGENTS.md`
4. `.github/instructions/backend-locallife.instructions.md`
5. `.github/instructions/backend-ocr.instructions.md`
6. `.github/instructions/backend-worker.instructions.md`
7. `.github/prompts/backend-bugfix.prompt.md`

然后只读下面这些源文件和测试：

- `locallife/api/ocr.go`
  - `enqueueOCRJob`：约在 265-301 行。
  - `markMerchantApplicationOCRPending`：约在 304-385 行。
  - `markOperatorApplicationOCRPending`：约在 387-449 行。
  - `markGroupApplicationOCRPending`：约在 461-530 行。
  - `markOCRPending`：约在 532-544 行。
  - media moderation 后续派发：`processPendingOCRJobsForMediaModeration` 约在 781-820 行。
  - `createOCRJob`：约在 852-976 行。
  - `validateOCRJobOwnerEditable`：约在 978-991 行。
  - `retryOCRJob`：约在 1145-1234 行。
- `locallife/api/ocr_rider_pending.go`
  - rider pending 写入和资产绑定：约在 15-102 行。
- `locallife/api/merchant_application.go`
  - `checkApplicationEditable`：约在 159-171 行。
- `locallife/worker/task_merchant_application_ocr.go`
  - merchant business license 写回：约在 117-188 行。
  - merchant food permit 写回：约在 216-274 行。
  - merchant id card 写回：约在 294-365 行。
- `locallife/worker/task_operator_application_ocr.go`
  - operator business license 写回：约在 75-145 行。
  - operator id card 写回：约在 168-237 行。
- `locallife/worker/task_group_application_ocr.go`
  - group business license 写回：约在 69-134 行。
  - group id card 写回：约在 137-223 行。
- `locallife/worker/task_rider_application_ocr.go`
  - rider stale asset guard：约在 101-113 行、175-239 行、349 行以后。它是本卡 worker guard 的参考，不是要整体重写 rider。
- `locallife/db/query/ocr_job.sql`
  - `UpsertOCRJob`：约在 1-18 行。
  - `MarkOCRJobProcessing`：约在 57-77 行。
  - `FailPendingOCRJob`：约在 112-124 行。
- `locallife/db/sqlc/models.go`
  - `MerchantApplication`：约在 1104-1149 行。
  - `MerchantGroupApplication`：约在 1241-1258 行。
  - `OperatorApplication`：约在 1548-1580 行。
  - `RiderApplication`：约在 2145-2170 行。
- 现有测试：
  - `locallife/api/ocr_test.go`
  - `locallife/api/ocr_async_response_test.go`
  - `locallife/worker/task_merchant_application_ocr_test.go`
  - `locallife/worker/task_operator_application_ocr_test.go`
  - `locallife/worker/task_operator_application_idcard_test.go`
  - `locallife/worker/task_group_application_ocr_test.go`
  - `locallife/worker/task_rider_application_ocr_test.go`

## 当前 finding

当前 `createOCRJob` 的顺序是：

1. 校验 owner 和 media moderation。
2. `UpsertOCRJob`。
3. 如果 moderation approved 且 job pending，先 `enqueueOCRJob`。
4. 然后才 `markOCRPending` 写业务申请单上的 media asset id 和 OCR pending JSON。

风险：

- 如果 `enqueueOCRJob` 成功但 `markOCRPending` 失败，请求会返回 500，但 worker 已经可能开始执行。
- worker 执行后可把 OCR 成功/失败结果写回申请单，即使请求侧没有成功绑定 pending。
- merchant/operator/group worker 在 success 和 failure 分支都缺少 “当前申请单仍绑定这个 asset / 当前 OCR JSON 仍是这个 job / 当前状态仍可写” 的 guard。
- `validateOCRJobOwnerEditable` 当前只对 rider 做 draft 校验；merchant/operator/group 依赖 `markOCRPending` 的内部逻辑，但这个逻辑在现有顺序里发生得太晚。
- operator/group 的 `mark*OCRPending` 在非 draft 时直接 `return nil`，这会让调用方以为 pending 已写入成功，后续仍可能创建和派发 job。

可利用或事故路径：

1. 用户在申请单 A 上发起 OCR job。
2. job 先入队。
3. `markOCRPending` 因状态、JSON、DB 或重置失败而失败。
4. 请求返回错误，前端/用户以为未成功。
5. worker 仍读取媒体并写回 OCR JSON 或业务字段。
6. 如果期间申请单已提交、已审核、被重置或用户替换了图片，陈旧 worker 仍可能覆盖当前字段。

## 修复后的不变式

1. 请求路径不变式：
   - 所有同步校验和业务 pending 写入必须先成功。
   - 只有当前申请单成功绑定 `media_asset_id` 和 `ocr_job_id` 后，才允许派发 worker。
   - 如果 pending 写入失败，不得派发 worker。
   - 如果派发失败，不能留下一个看起来会自动处理但永远不会处理的静默 pending；必须显式标记 job/业务 OCR 为失败，或返回可观察错误并让业务字段保持一致。
2. worker preflight 不变式：
   - worker 调用 provider 前，先加载 OCR job 和对应申请单。
   - job 的 `owner_type`、`owner_id`、`document_type`、`side`、`media_asset_id` 必须与 task payload 和当前申请单绑定一致。
   - 当前申请单必须处于可写状态。本卡收敛为 worker 只写 `draft`，避免提交/审核后的异步覆盖。
   - 当前 OCR JSON 中的 `ocr_job_id` 必须等于本次 job id；无法解析 OCR JSON 时 fail closed，跳过 provider 和写回，并记录日志。
3. worker writeback 不变式：
   - success 分支和 failure 分支都必须重新确认 guard，或者使用同一个 preflight 返回的 application 快照并在写回前再轻量复核。
   - 陈旧任务是 no-op：记录 structured log / audit，返回 nil，不触发 asynq 重试风暴。
   - stale skip 不应调用 provider；否则仍会发生敏感媒体读取和第三方 OCR 外呼。
4. media moderation 后续派发不变式：
   - `processPendingOCRJobsForMediaModeration` 在 moderation approved 后派发 pending jobs 前，也必须确认业务 pending 绑定仍存在且当前 owner 可写。
   - 被审核拒绝/quarantine 的 pending job 仍可按现有 `FailPendingOCRJob + markOCRFailed` 收敛，但 `markOCRFailed` 不得覆盖已替换 asset/job 的当前 OCR JSON。

## 业务绑定字段矩阵

worker guard 必须以当前申请单中的绑定字段为准：

| owner_type | document_type | side | 当前媒体绑定字段 | 当前 OCR JSON 字段 |
| --- | --- | --- | --- | --- |
| `merchant_application` | `business_license` | 空 | `BusinessLicenseMediaAssetID` | `BusinessLicenseOcr` |
| `merchant_application` | `food_permit` | 空 | `FoodPermitMediaAssetID` | `FoodPermitOcr` |
| `merchant_application` | `id_card` | `front` | `IDCardFrontMediaAssetID` | `IDCardFrontOcr` |
| `merchant_application` | `id_card` | `back` | `IDCardBackMediaAssetID` | `IDCardBackOcr` |
| `operator_application` | `business_license` | 空 | `BusinessLicenseMediaAssetID` | `BusinessLicenseOcr` |
| `operator_application` | `id_card` | `front` | `IDCardFrontMediaAssetID` | `IDCardFrontOcr` |
| `operator_application` | `id_card` | `back` | `IDCardBackMediaAssetID` | `IDCardBackOcr` |
| `group_application` | `business_license` | 空 | `LicenseMediaAssetID` | `application_data.business_license_ocr` |
| `group_application` | `id_card` | `front` | `application_data.id_card_front_asset_id` | `application_data.id_card_front_ocr` |
| `group_application` | `id_card` | `back` | `application_data.id_card_back_asset_id` | `application_data.id_card_back_ocr` |
| `rider_application` | `id_card` | `front` | `IDCardFrontMediaAssetID` | `IDCardOcr` |
| `rider_application` | `id_card` | `back` | `IDCardBackMediaAssetID` | `IDCardOcr` |
| `rider_application` | `health_cert` | 空 | `HealthCertMediaAssetID` | `HealthCertOcr` |

## 推荐实现位置

API 顺序修复：

- 修改 `locallife/api/ocr.go` 的 `createOCRJob` 和 `retryOCRJob`。
- 必要时新增小 helper：

```go
func (server *Server) markOCRJobDispatchFailed(ctx *gin.Context, job db.OcrJob, cause error) {
    failedJob, err := server.store.FailPendingOCRJob(ctx, db.FailPendingOCRJobParams{
        ID:           job.ID,
        ErrorCode:    pgtype.Text{String: "ocr_enqueue_failed", Valid: true},
        ErrorMessage: pgtype.Text{String: "enqueue OCR job failed", Valid: true},
    })
    if err == nil {
        _ = server.markOCRFailed(ctx, failedJob, "ocr_enqueue_failed", "enqueue OCR job failed")
    }
}
```

worker guard：

- 优先新增 `locallife/worker/ocr_writeback_guard.go`，避免把每个 task 文件继续拉长。
- 推荐小而直接的 guard，不要做一个过度通用的大框架。
- guard 可以按 owner 切分：

```go
type ocrWritebackGuardResult struct {
    Job db.OcrJob
    Stale bool
    Reason string
}

func (processor *RedisTaskProcessor) guardMerchantOCRWriteback(ctx context.Context, payload merchantApplicationOCRPayload, documentType ocr.DocumentType) (ocrWritebackGuardResult, db.MerchantApplication, error)
func (processor *RedisTaskProcessor) guardOperatorOCRWriteback(ctx context.Context, payload operatorApplicationOCRPayload, documentType ocr.DocumentType) (ocrWritebackGuardResult, db.OperatorApplication, error)
func (processor *RedisTaskProcessor) guardGroupOCRWriteback(ctx context.Context, payload groupApplicationOCRPayload, documentType ocr.DocumentType) (ocrWritebackGuardResult, db.MerchantGroupApplication, error)
```

- rider 已有 stale asset guard，可先不改。只有当统一 preflight 能低风险复用时，才轻量补齐 provider 前 skip。

## 分步修复计划

### 任务 1：补 API 顺序失败测试

修改：`locallife/api/ocr_test.go`

- [ ] 新增 `TestCreateOCRJob_DoesNotEnqueueWhenMarkPendingFails`
  - `UpsertOCRJob` 返回 pending job。
  - `markOCRPending` 对应的 application 更新返回错误。
  - 预期 HTTP 500。
  - 预期不调用 task distributor。
- [ ] 新增 `TestCreateOCRJob_MarksPendingBeforeEnqueue`
  - 用 gomock `InOrder` 或计数器确认 `Update*Application*OCR` 发生在 `DistributeTask*OCR` 前。
  - 预期 HTTP 200。
- [ ] 新增 `TestRetryOCRJob_DoesNotEnqueueWhenMarkPendingFails`
  - retry 创建新 job 后，pending 写入失败。
  - 预期不派发。

运行：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob)_(DoesNotEnqueueWhenMarkPendingFails|MarksPendingBeforeEnqueue)' -count=1
```

预期：新增测试先失败，失败点应显示当前实现先派发后 pending。

### 任务 2：调整 create/retry 顺序为先 pending 后 enqueue

修改：`locallife/api/ocr.go`

目标顺序：

1. `UpsertOCRJob`
2. 如果 `job.Status == pending`，先 `markOCRPending(ctx, job)`
3. 如果 `job.Status == pending` 且 moderation status 是 `approved`，再 `enqueueOCRJob(ctx, job)`
4. 如果 enqueue 失败：
   - 记录 structured log。
   - 调 `FailPendingOCRJob` 尝试把 job 收敛为 failed。
   - 调 `markOCRFailed` 尝试把业务 OCR JSON 从 pending 收敛为 failed。
   - 返回 `502` 或现有 `BadGateway` 分支。
5. 如果 `job.Status != pending`，不重复 mark pending、不重复派发。
6. 如果 moderation status 是 `pending`，保留 pending job 和业务 pending JSON，等待 media moderation 回调后再派发。

同时修改：

- `markOperatorApplicationOCRPending` 和 `markGroupApplicationOCRPending` 在非 draft 时不能 `return nil`。本卡要求返回 `ErrApplicationNotDraft` 或稳定业务错误，让 API 不派发。
- `validateOCRJobOwnerEditable` 应覆盖 merchant/operator/group/rider，而不是只覆盖 rider。
  - merchant 可沿用 `checkApplicationEditable`：`draft/rejected/approved/submitted` 可进入 pending 写入，其中 rejected/approved/submitted 由 `markMerchantApplicationOCRPending` 负责 reset 到 draft。
  - operator/group 没有现成 reset 语义时，先要求 `status == "draft"`，否则拒绝。
  - rider 继续要求 `draft`。

运行：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob)' -count=1
```

预期：API OCR 测试通过，新增顺序测试通过。

### 任务 3：实现 merchant worker preflight guard

修改：

- 新增 `locallife/worker/ocr_writeback_guard.go` 或在 `task_merchant_application_ocr.go` 中新增局部 helper。
- 修改 `locallife/worker/task_merchant_application_ocr.go`。
- 修改 `locallife/worker/task_merchant_application_ocr_test.go`。

规则：

- 在 `processMerchantApplicationBusinessLicenseOCRJob`、`processMerchantApplicationFoodPermitOCRJob`、`processMerchantApplicationIDCardOCRJob` 调 `processor.ocrService.ExecuteJob` 前，先执行 guard。
- guard 加载 `store.GetOCRJob(payload.OCRJobID)` 和 `store.GetMerchantApplication(payload.ApplicationID)`。
- guard 校验：
  - `job.OwnerType == "merchant_application"`
  - `job.OwnerID == payload.ApplicationID`
  - `job.MediaAssetID == payload.MediaAssetID`
  - `job.DocumentType` 与当前 task 对应
  - ID card task 的 `job.Side` 与 payload side 对应
  - application `Status == "draft"`
  - 当前媒体绑定字段等于 `payload.MediaAssetID`
  - 当前 OCR JSON 内 `ocr_job_id == payload.OCRJobID`
- stale 时记录 log 后 `return nil`，不得调用 `ExecuteJob`。
- success/failure 写回前可复用 preflight 返回的 app，但至少在写回前保证 payload 和 job 一致；如果实现选择二次加载 app，必须复核同一套绑定条件。

测试：

- [ ] `TestProcessTaskMerchantApplicationBusinessLicenseOCR_SkipsStaleAssetBeforeProvider`
- [ ] `TestProcessTaskMerchantApplicationBusinessLicenseOCR_SkipsNonDraftApplication`
- [ ] `TestProcessTaskMerchantApplicationIDCardOCR_SkipsStaleAssetBeforeProvider`
- [ ] 现有成功测试 payload 必须补 `MediaAssetID`，否则新 guard 会 fail closed。

运行：

```bash
cd locallife
go test ./worker -run 'TestProcessTaskMerchantApplication.*OCR' -count=1
```

预期：merchant worker 新旧测试通过，stale 测试确认 provider 未被调用。

### 任务 4：实现 operator worker preflight guard

修改：

- `locallife/worker/task_operator_application_ocr.go`
- `locallife/worker/task_operator_application_ocr_test.go`
- `locallife/worker/task_operator_application_idcard_test.go`
- 共享 guard 可放在 `locallife/worker/ocr_writeback_guard.go`。

规则：

- 在 operator business license 和 id card 调 provider 前执行 guard。
- guard 加载 `GetOCRJob` 和 `GetOperatorApplicationByID`。
- 校验 owner/document/side/media/status/当前绑定/OCR job id。
- 非 `draft` 或绑定不一致：log + `return nil`，不重试，不调用 provider。

测试：

- [ ] `TestProcessTaskOperatorApplicationBusinessLicenseOCR_SkipsStaleAssetBeforeProvider`
- [ ] `TestProcessTaskOperatorApplicationBusinessLicenseOCR_SkipsNonDraftApplication`
- [ ] `TestProcessTaskOperatorApplicationIDCardOCR_SkipsStaleAssetBeforeProvider`
- [ ] 现有成功测试 payload 补 `MediaAssetID`。

运行：

```bash
cd locallife
go test ./worker -run 'TestProcessTaskOperatorApplication.*OCR' -count=1
```

预期：operator worker 新旧测试通过。

### 任务 5：实现 group worker preflight guard

修改：

- `locallife/worker/task_group_application_ocr.go`
- `locallife/worker/task_group_application_ocr_test.go`
- 共享 guard 可放在 `locallife/worker/ocr_writeback_guard.go`。

规则：

- 在 group business license 和 id card 调 provider 前执行 guard。
- group business license 校验 `LicenseMediaAssetID == payload.MediaAssetID`。
- group id card 校验 `application_data.id_card_front_asset_id` 或 `application_data.id_card_back_asset_id` 等于 payload media。
- group OCR JSON 是 `application_data.*_ocr`，里面必须能读到 `ocr_job_id == payload.OCRJobID`。
- `application_data` 无法解析、字段类型不对、缺 asset id 或缺 ocr_job_id 时，一律 stale skip，不能调用 provider。
- 非 `draft`：log + `return nil`，不重试。

测试：

- [ ] `TestProcessTaskGroupApplicationBusinessLicenseOCR_SkipsStaleAssetBeforeProvider`
- [ ] `TestProcessTaskGroupApplicationBusinessLicenseOCR_SkipsNonDraftApplication`
- [ ] `TestProcessTaskGroupApplicationIDCardOCR_SkipsStaleAssetBeforeProvider`
- [ ] `TestProcessTaskGroupApplicationIDCardOCR_SkipsMalformedApplicationDataBeforeProvider`
- [ ] 现有成功测试 payload 补 `MediaAssetID`，并在 `ApplicationData` 中包含对应 asset id 和 pending OCR job id。

运行：

```bash
cd locallife
go test ./worker -run 'TestProcessTaskGroupApplication.*OCR' -count=1
```

预期：group worker 新旧测试通过。

### 任务 6：收口 media moderation 后续派发和失败写回

修改：`locallife/api/ocr.go`

- [ ] 在 `processPendingOCRJobsForMediaModeration` moderation approved 分支派发前，调用一个只读校验 helper，确认业务 pending 绑定仍存在。
- [ ] 如果业务绑定已不存在，调用 `FailPendingOCRJob` 把 job 标记为 failed/cancelled，记录 log，不派发。
- [ ] moderation rejected/quarantined 分支调用 `markOCRFailed` 前，也要防止覆盖已替换 asset/job 的 OCR JSON。
- [ ] 这部分可复用 API 层的 pending 绑定校验 helper，但不要依赖 worker 包，避免 import cycle。

测试：

- [ ] `TestMiniProgramMediaCheckNotify_ApprovedSkipsStalePendingOCRJob`
- [ ] `TestMiniProgramMediaCheckNotify_RejectedDoesNotOverwriteReplacedOCRPending`

运行：

```bash
cd locallife
go test ./api -run 'TestMiniProgramMediaCheckNotify_.*OCR' -count=1
```

预期：media moderation 后续处理不会派发或覆盖陈旧 job。

### 任务 7：全量聚焦验证

运行：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob|MiniProgramMediaCheckNotify_.*OCR)' -count=1
go test ./worker -run 'TestProcessTask(MerchantApplication|OperatorApplication|GroupApplication|RiderApplication).*OCR' -count=1
go test ./ocr -count=1
```

如果改了 SQL 查询或 store interface，再运行：

```bash
cd locallife
make sqlc
make mock
make check-generated
```

如果只改 API/worker Go 代码和测试，通常不需要 `make swagger`。

## 明确不做

- 不改 OCR provider 协议。
- 不改媒体对象级授权；那属于 `TASK-OCR-MEDIA-AUTHZ-001`。
- 不引入新的后台队列或 outbox。
- 不把 stale worker 任务当错误重试；stale 是预期 no-op。
- 不把 submitted/approved/rejected 申请单上的陈旧 OCR 写回当作兼容行为保留。
- 不顺手重构所有 OCR JSON DTO。

## 验证命令

最小验证：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob)' -count=1
go test ./worker -run 'TestProcessTask(MerchantApplication|OperatorApplication|GroupApplication).*OCR' -count=1
```

包含 rider 防回归：

```bash
cd locallife
go test ./worker -run 'TestProcessTask(RiderApplication|MerchantApplication|OperatorApplication|GroupApplication).*OCR' -count=1
```

包含 media moderation 后续派发：

```bash
cd locallife
go test ./api -run 'TestMiniProgramMediaCheckNotify_.*OCR|TestCreateOCRJob_DelaysDispatchWhileMediaModerationPending' -count=1
```

## 停止条件

- 如果某个 worker 现有 payload 没有 `media_asset_id`，先修 enqueue payload 和测试数据，不要让 guard 在缺字段时放行。
- 如果 group `application_data` 里历史 JSON 缺 `ocr_job_id`，新 job 仍必须写入；旧历史 task 可 stale skip，不补历史兼容放行。
- 如果 pending 写入失败后已经派发过任务，必须先调整顺序，再讨论失败收敛；不能只给 worker 加 guard 就认为已修。
- 如果实现需要新增条件更新 SQL 来原子校验 asset/job/status，先闭合 migration/sqlc，再继续 worker/API 逻辑。
- 如果任何测试需要通过“provider 被调用但最后不写回”来证明安全，停止并重写；本卡要求 stale 在 provider 调用前被拦截。
