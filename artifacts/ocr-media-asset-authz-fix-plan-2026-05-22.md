# TASK-OCR-MEDIA-AUTHZ-001：统一 OCR 创建入口补齐媒体资产对象级授权

日期：2026-05-22
范围：仅 `locallife/` 后端。
风险等级：G3。原因：涉及 OCR、私有媒体、身份证/健康证等敏感证件、申请单归属、对象级授权和后台内部读取私有对象。

> 交接要求：这张卡必须能在上下文压缩、切换 agent 或完全失忆后单独执行。执行者先读本卡，再按“上下文重建清单”打开源文件，不要靠会话记忆补判断。

## 目标

`POST /v1/ocr/jobs` 和 `POST /v1/ocr/jobs/{id}/retry` 在创建或重试 OCR job 前，必须确认当前登录用户既有申请单权限，也有把该 `media_asset_id` 用于当前 owner/document/side 的权限。任何用户都不能把别人的私有证件、错误分类的图片、未确认上传完成的媒体资产绑定到自己的申请单并触发内部 OCR 读取。

## 上下文重建清单

每次接手、压缩后恢复或切换上下文时，从仓库根目录 `/home/sam/locallife` 重新读：

1. `.github/README.md`
2. `.github/copilot-instructions.md`
3. `locallife/AGENTS.md`
4. `.github/instructions/backend-locallife.instructions.md`
5. `.github/instructions/backend-ocr.instructions.md`
6. `.github/instructions/backend-media.instructions.md`
7. `.github/prompts/backend-bugfix.prompt.md`

然后只读下面这些源文件和测试：

- `locallife/api/ocr.go`
  - `isOCROwner` / `canAccessOCRJob`：约在 220-263 行。
  - `getOCRMediaModerationStatus`：约在 559-568 行。
  - `createOCRJob`：约在 852-976 行。
  - `retryOCRJob`：约在 1145-1234 行。
- `locallife/api/media.go`
  - `getMediaPrivateAccess`：约在 189-238 行。
  - `isOwnerOnlyPrivateMedia` / `isPrivateDocumentMediaModerationExempt`：约在 240-259 行。
  - `getMediaAsset`：约在 299-335 行。
- `locallife/media/policy.go`
  - 媒体分类常量：约在 12-29 行。
  - 分类可见性策略：约在 47-64 行。
- `locallife/media/registry.go`
  - `ReadMediaAsset`：约在 311-339 行。注意它是内部读取，不做调用者授权。
- `locallife/db/sqlc/models.go`
  - `MediaAsset` 字段：约在 980-1003 行，重点是 `MediaCategory`、`UploadStatus`、`ModerationStatus`、`UploadedBy`、`Visibility`、`DeletedAt`。
- `locallife/api/ocr_test.go`
  - 现有 `TestCreateOCRJob_*` / `TestRetryOCRJob_*`。
- `locallife/api/media_moderation_test.go`
  - pending media moderation 触发 OCR 后续处理的现有测试。

## 当前 finding

OCR 创建入口当前只校验申请单 owner：

- `createOCRJob` 先通过 `isOCROwner` 确认 `owner_type + owner_id` 属于当前用户，或者当前用户是 admin。
- 随后只调用 `getOCRMediaModerationStatus(ctx, req.MediaAssetID)`。
- `getOCRMediaModerationStatus` 只按 `media_asset_id` 调 `store.GetMediaAssetByID`，再根据 moderation status 返回 `approved` / `pending` / blocked。
- 这个路径没有校验 `media_assets.uploaded_by`、`media_category`、`upload_status`，也没有复用媒体私有访问接口里已有的 owner-only 规则。
- worker 后续通过 `media.Registry.ReadMediaAsset` 内部读取对象；这个内部 reader 只按 asset id 读对象，不知道 HTTP 调用者是谁，因此授权必须在 OCR job 创建/重试前完成。

可利用路径：

1. 用户 A 拥有自己的申请单。
2. 用户 A 获得或猜到用户 B 的 `media_asset_id`，尤其是身份证正反面或健康证等私有媒体。
3. 用户 A 调 `POST /v1/ocr/jobs`，body 里传自己的 `owner_id` 和用户 B 的 `media_asset_id`。
4. 当前后端创建 job、绑定媒体并派发 worker。
5. worker 内部读取用户 B 的私有对象，OCR 归一化结果写入用户 A 的申请单，随后用户 A 可通过自己的申请单/OCR job 结果读取敏感信息。

## 修复后的不变式

1. `media_asset_id` 只是对象 id，不是授权凭据。
2. OCR 创建/重试必须同时满足：
   - 当前用户能操作该 OCR owner；
   - 媒体资产存在、未删除；
   - 媒体资产已经完成上传确认：`upload_status == "confirmed"`；
   - 媒体分类与 `owner_type + document_type + side` 严格匹配；
   - 普通用户必须是媒体上传者：`asset.UploadedBy == authPayload.UserID`；
   - owner-only 私有证件媒体不得被 admin 以“审核便利”为由跨用户用于新 OCR job。
3. `isPrivateDocumentMediaModerationExempt` 只影响内容审核状态是否可视为 approved，不影响对象级授权。
4. `retryOCRJob` 不能只凭 “能访问旧 job” 就重试；它必须重新校验旧 job 里的 `media_asset_id` 仍允许由当前调用者用于该 owner/document/side。
5. 所有拒绝必须 fail closed：
   - 媒体不存在：404，稳定错误 `media asset not found`。
   - 媒体归属不符：403，稳定错误 `forbidden` 或 `media.ErrUnauthorized`。
   - 分类/side 不符或未确认上传：400，稳定中文/现有 error shape，不继续创建 job。
   - DB/基础设施错误：500，走 `internalError`。

## 分类匹配矩阵

实现时把这个矩阵写成一个小 helper，并用测试锁住：

| owner_type | document_type | side | 允许的 media_category |
| --- | --- | --- | --- |
| `merchant_application` | `business_license` | 空 | `business_license` |
| `merchant_application` | `food_permit` | 空 | `food_permit` |
| `merchant_application` | `id_card` | `front` | `id_card_front` |
| `merchant_application` | `id_card` | `back` | `id_card_back` |
| `operator_application` | `business_license` | 空 | `business_license` |
| `operator_application` | `id_card` | `front` | `id_card_front` |
| `operator_application` | `id_card` | `back` | `id_card_back` |
| `rider_application` | `id_card` | `front` | `id_card_front` |
| `rider_application` | `id_card` | `back` | `id_card_back` |
| `rider_application` | `health_cert` | 空 | `health_cert` |
| `group_application` | `business_license` | 空 | `group_license` 或 `business_license` |
| `group_application` | `id_card` | `front` | `id_card_front` |
| `group_application` | `id_card` | `back` | `id_card_back` |

集团营业执照兼容说明：

- `media.Policy` 已有 `CategoryGroupLicense = "group_license"`，但现有 group OCR pending 写入 `LicenseMediaAssetID`，前端历史上传可能仍传 `business_license`。
- 实施前必须用 `rg -n "group_license|business_license" weapp web merchant_app locallife/api locallife/media` 查一遍真实上传入口。
- 如果现状确认 group 仍上传 `business_license`，本次后端兼容 `group_license` 和 `business_license` 两类；不要因为安全修复导致现有 group 入口直接断流。
- 如果现状确认 group 只上传 `group_license`，只允许 `group_license`，并在测试里锁住。

## 推荐实现位置

优先在 `locallife/api/ocr.go` 内新增小 helper；如果文件过大或更清晰，可以新建 `locallife/api/ocr_media_authz.go`。不要把 HTTP error response 写进 `media/` 包。

推荐 helper 签名：

```go
type authorizedOCRMediaAsset struct {
    Asset            db.MediaAsset
    ModerationStatus string
}

func (server *Server) loadAuthorizedOCRMediaAsset(
    ctx *gin.Context,
    authPayload *token.Payload,
    ownerType ocr.OwnerType,
    documentType ocr.DocumentType,
    side ocr.DocumentSide,
    mediaAssetID int64,
) (authorizedOCRMediaAsset, error)
```

推荐内部规则：

1. `store.GetMediaAssetByID(ctx, mediaAssetID)` 加载资产。
2. `asset.UploadedBy != authPayload.UserID` 时返回授权错误。不要给 owner-only 证件媒体开 admin 旁路。
3. `strings.ToLower(strings.TrimSpace(asset.UploadStatus)) != "confirmed"` 时返回业务错误。
4. 用 `expectedOCRMediaCategories(ownerType, documentType, side)` 返回允许分类集合，不匹配时返回业务错误。
5. moderation status 复用现有逻辑：
   - 私有文档豁免分类仍可视为 `approved`。
   - 其他分类取 `strings.ToLower(strings.TrimSpace(asset.ModerationStatus))`。
6. 返回 asset 和 moderation status，避免创建入口再二次查 DB。

错误处理建议：

- 定义包内 sentinel error，例如：

```go
var (
    errOCRMediaUnauthorized  = errors.New("forbidden")
    errOCRMediaWrongCategory = errors.New("media asset category does not match OCR document")
    errOCRMediaNotConfirmed  = errors.New("media asset upload is not confirmed")
)
```

- 在 `createOCRJob` / `retryOCRJob` 里把这些错误映射成 403/400，不要直接把底层 SQL 或 storage 错误暴露给用户。

## 分步修复计划

### 任务 1：先补失败测试，锁住越权路径

修改：`locallife/api/ocr_test.go`

- [ ] 新增 `TestCreateOCRJob_RejectsMediaOwnedByAnotherUser`
  - 当前用户 owns merchant application。
  - `GetMediaAssetByID` 返回 `UploadedBy` 为另一个用户的 `MediaAsset`。
  - 预期 HTTP 403。
  - 预期不调用 `UpsertOCRJob`。
  - 预期不调用 task distributor。
- [ ] 新增 `TestCreateOCRJob_RejectsWrongMediaCategoryForIDCardSide`
  - 请求 `document_type=id_card, side=front`。
  - asset category 返回 `id_card_back` 或 `business_license`。
  - 预期 HTTP 400。
  - 预期不创建 job。
- [ ] 新增 `TestCreateOCRJob_RejectsUnconfirmedMedia`
  - asset `UploadStatus="uploaded"` 或 `"pending"`。
  - 预期 HTTP 400。
  - 预期不创建 job。
- [ ] 新增 `TestRetryOCRJob_RevalidatesMediaOwnership`
  - 旧 job 可由当前用户访问。
  - 旧 job 的 `media_asset_id` 对应 asset `UploadedBy` 为另一个用户。
  - 预期 HTTP 403。
  - 预期不创建新 retry job、不派发任务。

运行：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob)_(RejectsMediaOwnedByAnotherUser|RejectsWrongMediaCategoryForIDCardSide|RejectsUnconfirmedMedia|RevalidatesMediaOwnership)' -count=1
```

预期：这些新增测试先失败，失败点应暴露现有实现仍调用 `UpsertOCRJob` 或返回 200。

### 任务 2：实现媒体分类矩阵和授权 helper

修改：`locallife/api/ocr.go` 或新增 `locallife/api/ocr_media_authz.go`

- [ ] 新增 `expectedOCRMediaCategories(ownerType, documentType, side)`。
- [ ] 新增 `ocrMediaCategoryAllowed(asset.MediaCategory, expected)`。
- [ ] 新增 `loadAuthorizedOCRMediaAsset`。
- [ ] 保留现有 `isPrivateDocumentMediaModerationExempt`，但只在授权、上传状态、分类校验通过后使用。
- [ ] 不修改 `media.Registry.ReadMediaAsset`，它仍是内部 reader；调用前的授权由 API 层负责。

运行：

```bash
cd locallife
go test ./api -run 'TestCreateOCRJob_(RejectsMediaOwnedByAnotherUser|RejectsWrongMediaCategoryForIDCardSide|RejectsUnconfirmedMedia)' -count=1
```

预期：任务 1 中 create 相关新增测试通过。

### 任务 3：替换 create/retry 中的弱校验

修改：`locallife/api/ocr.go`

- [ ] 在 `createOCRJob` 中，用 `loadAuthorizedOCRMediaAsset` 替换 `getOCRMediaModerationStatus(ctx, req.MediaAssetID)`。
- [ ] 在 `retryOCRJob` 中，用旧 job 的 `OwnerType`、`DocumentType`、`Side`、`MediaAssetID` 调同一个 helper。
- [ ] 如果 `retryOCRJob` 允许 admin 访问旧 job，也必须执行同样媒体授权。owner-only 私有证件媒体跨用户仍拒绝。
- [ ] 保持现有 moderation 行为：`approved` 可派发，`pending` 可创建但暂不派发，`rejected/quarantined` 返回 `ErrImageContentSafetyFailed`。

运行：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob)' -count=1
```

预期：OCR API 相关测试通过。若现有测试的 mock asset 没有设置 `UploadStatus` 或 `UploadedBy`，按新不变式补全测试数据，不要放宽生产校验。

### 任务 4：补回归测试覆盖允许路径

修改：`locallife/api/ocr_test.go`

- [ ] 调整现有成功测试，让 `GetMediaAssetByID` 返回：
  - `UploadedBy == authPayload.UserID`
  - `UploadStatus == "confirmed"`
  - `MediaCategory` 与矩阵匹配
  - `ModerationStatus == "approved"` 或私有证件豁免分类。
- [ ] 保留这些已有成功语义：
  - owner 自己创建 merchant business license job 成功。
  - owner 自己创建 id card job 成功。
  - 私有身份证/健康证 moderation 不为 approved 时仍因私有文档豁免可继续。
  - moderation pending 时 job 创建但不派发。

运行：

```bash
cd locallife
go test ./api -run 'TestCreateOCRJob' -count=1
go test ./api -run 'TestRetryOCRJob' -count=1
```

预期：全部通过。

## 明确不做

- 不修改媒体上传协议。
- 不修改 `media.Registry.ReadMediaAsset` 的内部读取职责。
- 不把 admin 变成 owner-only 身份证媒体的跨用户 OCR 创建者。
- 不改 OCR job 表结构。
- 不改 worker 写回顺序；worker 防陈旧写回属于 `TASK-OCR-ASYNC-WRITEBACK-001`。
- 不顺手调整前端、小程序或 Flutter 入口。

## 验证命令

最小验证：

```bash
cd locallife
go test ./api -run 'Test(CreateOCRJob|RetryOCRJob)' -count=1
```

媒体 moderation 回归：

```bash
cd locallife
go test ./api -run 'TestMiniProgramMediaCheckNotify_.*OCR|TestCreateOCRJob_DelaysDispatchWhileMediaModerationPending' -count=1
```

如果只改 `api/*.go` 和测试，通常不需要：

- `make sqlc`
- `make swagger`
- `make mock`

如果新增 helper 后 mock 接口没有变化，也不需要 `make mock`。如果实际实现改了 store interface 或 SQL 查询，必须跑：

```bash
cd locallife
make sqlc
make check-generated
```

## 停止条件

- 如果发现 group 上传分类事实与本卡矩阵不一致，先更新本卡“分类匹配矩阵”和测试预期，再写代码。
- 如果实现需要 admin 代用户创建 OCR job，必须先写清楚 owner-only 私有媒体例外策略；不能默认给 admin 放行身份证和健康证跨用户 OCR。
- 如果新增 SQL 才能表达安全校验，先闭合 migration/sqlc，再继续 API 改动。
- 如果媒体授权测试必须靠放宽错误语义才能通过，停止并重新确认不变式。
