# 媒体系统改造任务计划

本文档为 [MEDIA_OSS_CDN_FINAL_IMPLEMENTATION_PLAN_2026-03-18.md](./MEDIA_OSS_CDN_FINAL_IMPLEMENTATION_PLAN_2026-03-18.md) 的施工落地任务拆分，按阶段顺序排列，完成一项勾选一项。

**前置文档：**
- 实施方案：`./MEDIA_OSS_CDN_FINAL_IMPLEMENTATION_PLAN_2026-03-18.md`
- 测试清单：`MEDIA_TEST_AND_ACCEPTANCE_CHECKLIST_2026-03-18.md`
- 上线 Runbook：`./MEDIA_RELEASE_RUNBOOK_2026-03-18.md`
- 小程序迁移手册：`./MEDIA_WEAPP_MIGRATION_PLAYBOOK_2026-03-18.md`

---

## Phase 0：前置确认（不写代码）

- [x] 确认 OSS 厂商选型：**阿里云 OSS**（`storage_oss.go` 已实现，`app.env` 已填入正式 bucket）
- [x] 确认 CDN 厂商选型：CDN 域名已配置到 `app.env`（`CDN_PUBLIC_BASE_URL`）
- [x] 确认 STS / 临时授权方案：**采用服务端 HMAC-SHA1 签名 POST Policy**，AccessKey 不下发客户端，无需 STS
- [x] 确认图片处理能力：**OSS 图片处理样式**（`resolver.go` 已按 `?x-oss-process=style/thumb|card|detail` 格式生成 URL，Phase 1 在控制台建三个样式即可）
- [x] 确认私有图片签名方案：**OSS presigned GET URL**（`storage_oss.go` `CreatePrivateDownloadURL` 已实现）
- [x] 确认 app.env 生产配置由谁维护，并安排双人复核：执行时机在 Phase 10 上线前，上线 Runbook §4 检查清单已覆盖
- [x] 确认 MEDIA_MAX_UPLOAD_BYTES 的数值：**10MB**（`app.env` `MEDIA_MAX_UPLOAD_BYTES=10485760`）
- [x] 确认 PRIVATE_DOWNLOAD_URL_TTL 的值：**5 分钟**（`registry.go` `DownloadObject` 默认 5 min，`app.env` 可覆盖）

---

## Phase 1：基础设施

### 1.1 OSS

- [ ] 创建 **media-public** 公共桶
  - [ ] 关闭公共读（仅 CDN 回源 IP / Referer 白名单可读）
  - [ ] 配置 CORS：允许 Web 和小程序直传域名
  - [ ] 配置图片处理样式：`thumb`（200px）、`card`（400px）、`detail`（960px）
  - [ ] 开启版本控制或内容哈希策略（sha256 文件名已自带去重）
- [ ] 创建 **media-private** 私有桶
  - [ ] 关闭所有公共访问
  - [ ] 配置 CORS：允许后端服务器域名
  - [ ] 确认 presigned URL 过期时间配置生效
- [ ] 配置 STS 角色和策略（或等价临时授权方案）
  - [ ] 公共桶直传权限：`PutObject` on `media-public/*`
  - [ ] 私有桶直传权限：`PutObject` on `media-private/*`
  - [ ] 后端读取私有桶权限：`GetObject`, `HeadObject` on `media-private/*`

### 1.2 CDN

- [ ] 创建 CDN 域名（如 `cdn.locallife.example.com`）
- [ ] 配置回源到 media-public 桶源站
- [ ] 配置 HTTPS 证书
- [ ] 配置 HTTP/2
- [ ] 配置 Brotli / Gzip 压缩
- [ ] 配置长缓存规则（静态文件 365 天，或按 sha256 文件名）
- [ ] 配置屏蔽直接回源访问（Referer / 回源鉴权）
- [ ] 在 CDN 或 OSS 层验证图片处理样式 URL 生效（访问 `?x-oss-process=style/thumb` 类路径）
- [ ] **CDN 预热**：上线后对高频菜品图、商户图执行预热，避免首屏冷启动

---

## Phase 2：数据库迁移

> 所有 migration 须编写对应 `.down.sql`，并在空库和旧库各跑一遍 up 验证。

### 2.1 新增核心表

- [x] `000140_create_media_assets.up.sql`
  - 字段：`id`, `object_key`（唯一）, `visibility`, `media_category`, `mime_type`, `file_size`, `width`, `height`, `checksum_sha256`, `upload_status`, `moderation_status`, `uploaded_by`, `source_client`, `created_at`, `updated_at`, `deleted_at`
  - 索引：`unique(object_key)`, `(uploaded_by)`, `(media_category)`, `(visibility, moderation_status)`, `(created_at DESC)`
  - upload_status 枚举约束：`pending`, `uploaded`, `confirmed`, `failed`, `deleted`
  - moderation_status 枚举约束：`pending`, `approved`, `rejected`, `quarantined`
- [x] `000141_create_media_upload_sessions.up.sql`
  - 字段：`id`（`upload_id` 字符串主键）, `media_asset_id`（可空 FK）, `user_id`, `business_type`, `media_category`, `visibility`, `object_key`, `checksum_sha256`, `status`, `expire_at`, `created_at`
  - 索引：`(user_id, media_category, checksum_sha256)` 支持幂等查询

### 2.2 业务表改造：单图字段

- [x] `000142_media_merchants_logo.up.sql`：`merchants` 表增加 `logo_media_asset_id bigint REFERENCES media_assets(id)`
- [x] `000143_media_dishes_image.up.sql`：`dishes` 表增加 `image_media_asset_id bigint REFERENCES media_assets(id)`
- [x] `000144_media_users_avatar.up.sql`：`users` 表增加 `avatar_media_asset_id bigint REFERENCES media_assets(id)`
- [x] `000145_media_group_brands_logo.up.sql`：`group_brands`（或等价品牌表）增加 `logo_media_asset_id`

### 2.3 业务表改造：多图关联表

- [x] `000146_create_review_images.up.sql`
  - 新建 `review_images` 表：`id`, `review_id REFERENCES reviews(id) ON DELETE CASCADE`, `media_asset_id REFERENCES media_assets(id)`, `sort_order int`, `created_at`
  - 唯一约束：`(review_id, media_asset_id)`
- [x] `000147_migrate_table_images_to_media_asset.up.sql`：`table_images` 增加 `media_asset_id bigint REFERENCES media_assets(id)`（原 `image_url` 保留直到旧数据迁移完）

### 2.4 申请材料表改造

- [x] `000148_media_merchant_applications.up.sql`
  - `merchant_applications` 增加：`business_license_media_asset_id`, `food_permit_media_asset_id`
- [x] `000149_media_rider_applications.up.sql`
  - `rider_applications`（或等价表）增加：`idcard_front_media_asset_id`, `idcard_back_media_asset_id`, `health_cert_media_asset_id`
- [x] `000150_media_operator_applications.up.sql`
  - `operator_applications` 增加：`business_license_media_asset_id`, `license_image_media_asset_id`, `logo_media_asset_id`

### 2.5 sqlc 更新

- [x] 执行 `make migrateup` 在开发库跑完全部 migration
- [x] 执行 `make sqlc` 重新生成 Go 数据层代码
- [x] 确认生成文件无编译错误

---

## Phase 3：后端配置扩展

- [x] `util/config.go`：新增以下字段（参见实施方案 §13）
  - `FileStorageProvider string`（`local` / `oss`）
  - `OSSPublicEndpoint string`
  - `OSSPrivateEndpoint string`
  - `OSSPublicBucket string`
  - `OSSPrivateBucket string`
  - `OSSRegion string`
  - `OSSStsRoleArn string`
  - `OSSStsExternalID string`
  - `CdnPublicBaseURL string`
  - `PrivateDownloadURLTTL time.Duration`
  - `MediaMaxUploadBytes int64`
  - `MediaAllowedImageTypes []string`
  - `MediaDirectUploadExpire time.Duration`
  - `ImageVariantThumbWidth int`
  - `ImageVariantCardWidth int`
  - `ImageVariantDetailWidth int`
- [x] `app.env.example`：补充所有新配置项的示例值和注释
- [x] `app.env`（本地开发）：填入开发环境对应值（`FileStorageProvider=local` 先跑通）

---

## Phase 4：后端媒体中心模块

> 建议新建 `locallife/media/` 包，不污染现有 api/ 和 util/。

### 4.1 ObjectStorage 接口层

- [x] 定义 `media/storage.go`：`ObjectStorage` 接口
  ```go
  CreateDirectUpload(ctx, req) (CreateDirectUploadResult, error)
  StatObject(ctx, bucket, objectKey) (ObjectMetadata, error)
  CreatePrivateDownloadURL(ctx, bucket, objectKey, ttl) (string, error)
  DeleteObject(ctx, bucket, objectKey) error
  ```
- [x] 实现 `media/storage_local.go`：本地 fallback（开发环境用）
  - `CreateDirectUpload`：返回后端本地接收地址（暂时兼容旧 FormData 路径或新增临时接口）
  - `StatObject`：读取 `uploads/` 目录
  - `CreatePrivateDownloadURL`：返回带签名的本机 URL（复用现有 `api/upload_signed.go` 逻辑）
  - `DeleteObject`：删除本地文件
- [x] 实现 `media/storage_oss.go`：OSS 生产实现
  - `CreateDirectUpload`：生成 POST Policy 表单凭证（含 OSS STS 或 HMAC 签名）
  - `StatObject`：调用 OSS HeadObject
  - `CreatePrivateDownloadURL`：生成 presigned GET URL
  - `DeleteObject`：调用 OSS DeleteObject
- [x] 单元测试 `media/storage_local_test.go`
- [x] 单元测试 `media/storage_oss_test.go`（mock OSS SDK 或用 test bucket）

### 4.2 MediaPolicy

- [x] `media/policy.go`：实现 `MediaPolicy` 结构体
  - 输入：`userID`, `businessType`, `mediaCategory`, `contentType`, `contentLength`
  - 输出：`visibility`, `objectKeyPrefix`, `policyConstraints`
  - 实现所有 media_category 的路由规则（见实施方案 §4.2）
  - 输入校验：非法 `content_type` 拒绝；`content_length` 超限拒绝；非法 `media_category` 拒绝
- [x] 单元测试 `media/policy_test.go`：覆盖所有角色 × category 组合

### 4.3 MediaRegistry

- [x] `media/registry.go`：实现 `MediaRegistry` 结构体，依赖 sqlc 生成的查询层
  - `CreateUploadSession`：幂等创建上传会话（同 user+category+checksum 复用未完成会话）
  - `CompleteUpload`：幂等确认完成，写入 `media_assets`，建立业务绑定
  - `GetMediaAsset`：按 `media_asset_id` 查询
  - `SoftDeleteMediaAsset`：标记 deleted，异步投递删除任务
  - `BindResource`：建立 `media_asset_id` 与业务资源的关联
- [x] 单元测试 `media/registry_test.go`（使用 mock DB；18 cases：CreateUploadSession ×4、CompleteUpload ×7、GetAsset ×3、SoftDelete ×2、CreatePrivateAccessURL ×2）

### 4.4 MediaURLResolver

- [x] `media/resolver.go`：实现 `MediaURLResolver` 结构体
  - `PublicVariantURL(objectKey, variant string) string`：返回 CDN 规格图地址
  - `PublicOriginalURL(objectKey string) string`：返回 CDN 原图地址
  - `PrivateSignedURL(ctx, mediaID int64, ttl) (string, error)`：鉴权后签发短期地址（不暴露 objectKey 给调用方）
  - 替换现有所有 `normalizeUploadURLForClient` 调用
- [x] 单元测试 `media/resolver_test.go`

### 4.5 API Handler

- [x] `api/media.go`：实现以下 handler
  - `POST /v1/media/upload-sessions`（见实施方案 §8.1）
    - 鉴权（未登录 401，无上传权限 403）
    - 调用 MediaPolicy 校验
    - 调用 ObjectStorage.CreateDirectUpload
    - 返回 `upload_id`, `upload_host`, `form`（不返回 object_key 给私有图）
  - `POST /v1/media/complete`（见实施方案 §8.2）
    - 幂等
    - 校验 `upload_id` 归属当前用户
    - 校验 `object_key` 与会话一致
    - 调用 ObjectStorage.StatObject 确认对象存在
    - 写入 media_assets，触发异步审核
    - 返回 `media_id`, `visibility`, `urls`（公共图）
  - `POST /v1/media/private-access`（见实施方案 §8.3）
    - 接收 `media_id`（不接收 object_key）
    - 鉴权：校验调用者有权访问该 media_asset
    - 返回短期签名地址，写访问审计日志
  - `DELETE /v1/media/{id}`
    - 软删除 media_assets
    - 异步投递 OSS 对象删除任务
    - 写审计日志
  - `GET /v1/media/{id}`
    - 返回媒体元数据（私有媒体需鉴权）
- [x] 在 `api/server.go` 注册 `/v1/media/*` 路由
- [x] API 集成测试：覆盖实施方案 §5.1~5.5 列出的所有测试面（对应测试清单第 5 节）（25 cases，`api/media_test.go`）

---

## Phase 5：后端业务接口改造

> 逐表逐接口改造，每个子项均须通过对应的业务回归测试。

### 5.1 菜品（dishes）

- [x] `api/dish.go`：创建/更新接口接受 `image_media_asset_id`（不再接收图片文件流）
- [x] `api/dish.go`：列表和详情响应通过 `MediaURLResolver` 返回规格图 URL
  - 列表默认返回 `card_url`，详情返回 `detail_url`，保留兼容字段 `image_url`（指向 `card_url`）
- [x] 回归测试：菜品 CRUD + 图片展示（`TestListDishesByMerchantWithImageURLs`、`TestGetDishWithImageURL` 覆盖 CDN URL 字段）

### 5.2 桌台图片（table_images）

- [x] `api/` 桌台相关接口：添加图片改为提交 `media_asset_id`（`addTableImageRequest.MediaAssetID int64 binding:"required,min=1"` 写入 `table_images.media_asset_id`）
- [x] 桌台图列表响应通过 `MediaURLResolver` 返回规格图 URL（`roomDetailResponse.ImageURLs []string` + `PrimaryImageURL` + `roomListItemResponse.ImageURL`）
- [x] 主图逻辑 `is_primary` 在新模型下仍正确
- [x] 回归测试：桌台图片增删改查

### 5.3 评价（reviews）

- [x] `api/` 评价接口：提交评价接受 `media_asset_ids []int64`
- [x] 写入 `review_images` 关联表（不再写 `reviews.images` 数组）
- [x] 评价详情和列表响应通过 `MediaURLResolver` 返回规格图（`enrichSingleReviewImages` / `enrichReviewListImages`）
- [x] 回归测试：评价上传 + 展示

### 5.4 商户设置与品牌

- [x] 商户 logo 上传接口改为接受 `logo_media_asset_id`（`updateMerchantRequest.LogoAssetID *int64 json:"logo_asset_id"` 映射到 `arg.LogoMediaAssetID`）
- [x] 商户详情/列表响应保留兼容字段 `logo_url`（由 `MediaURLResolver` 生成，已覆盖 merchant.go / favorite.go / membership.go / operator_merchant_rider.go / group.go merchantResponse）
- [x] 品牌/集团 logo 同步改造（`brandResponse.LogoURL` + `groupMerchantResponse.LogoURL` 已注入）
- [x] 回归测试：商户设置 logo + 列表展示

### 5.5 用户头像

- [x] `api/user.go` + `db/query/user.sql`：头像改为同时接受 `avatar_media_asset_id`（新字段）和兼容旧 `avatar_url`
- [x] 用户信息响应：若 `avatar_media_asset_id` 有值则通过 `MediaURLResolver.VariantOriginal` 生成 `avatar_url`，否则回退旧逻辑
- [x] 回归测试：头像更新 + 展示

### 5.6 商户入驻申请（merchant_applications）

- [x] 接口改为接受 `business_license_media_asset_id`, `food_permit_media_asset_id`
- [x] OCR 链路：改为从 `media_assets.object_key` 取图路径，调用 OCR 服务
- [x] 回归测试：申请提交 + OCR 识别

### 5.7 骑手入驻申请

- [x] 接口改为接受 `idcard_front_media_asset_id`, `idcard_back_media_asset_id`, `health_cert_media_asset_id`
- [x] OCR 链路适配
- [x] 回归测试：申请提交 + 材料展示

### 5.8 运营商入驻申请

- [x] 接口改为接受 `business_license_media_asset_id`, `id_card_front_media_asset_id`, `id_card_back_media_asset_id`（实际 DB 字段与原计划 `license_image/logo_media_asset_id` 有出入，以实际迁移为准）
- [x] OCR 链路适配
- [x] 回归测试：申请提交 + 材料展示

### 5.9 全局图片 URL 生成替换

- [x] `grep -r "normalizeUploadURLForClient\|UPLOADS_BASE_URL\|image_url.*uploads"` 全量排查
- [x] 用户/员工/会员/顾客头像：`avatar_media_asset_id` 有值时通过 `publicImageURL(ctx, id, VariantOriginal)` 生成，否则回退 `normalizeUploadURLForClient(avatarUrl)` for 旧数据
  - 涉及：`api/user.go`, `api/staff.go`, `api/membership.go`, `api/merchant_stats.go`
  - SQL 查询同步新增 `u.avatar_media_asset_id`：`ListMerchantStaffByMerchant`, `ListMerchantMembers`, `GetMerchantCustomerStats`, `GetCustomerMerchantDetail`
- [x] 剩余 `normalizeUploadURLForClient` 均为合法回退（旧 storefront_images JSONB、直传响应、QR 码），不在此阶段替换

### 5.10 业务响应 URL 填充（response-side enrichment）

- [x] `api/kitchen.go`：`kitchenOrderItem.ImageURL` — `batchPublicImageURLs` 在 `convertToKitchenOrder` 循环后批量填充（`VariantThumb`）
- [x] `api/cart.go`：`cartItemResponse.ImageURL` — 新增 `enrichCartImageURLs` 方法，在两处 `toCartResponse` 调用点后注入
- [x] `api/cart.go`：`browseHistoryItem.ImageURL` — 复用已查询的 `dishMap`/`merchantMap`，单次 `batchPublicImageURLs` 替换旧 TODO 注释
- [x] `api/combo.go`：`comboSetWithDetailsResponse.ImageURL` — 两处构造点均调用 `publicImageURL`
- [x] `api/combo.go`：`comboSetWithDetailsResponse.DishImageURLs []string` — `enrichSingleComboImages` 同时填充 asset ID 列表和 CDN URL 列表
- [x] `api/search.go`：`searchDishResponse.ImageURL` + `MerchantLogoURL` — `enrichSearchDishURLs` 在两条搜索路径各自 `ctx.JSON` 前注入
- [x] `api/search.go`：`searchMerchantResponse.LogoURL` — `enrichSearchMerchantURLs` 在合并路径后注入
- [x] `api/search.go`：`searchComboResponse.ImageURL` + `MerchantLogoURL` — `enrichSearchComboURLs` 注入
- [x] `api/merchant.go`：`publicDishItem.ImageURL` — `getPublicMerchantDishes` 建列后批量填充（`VariantCard`）
- [x] `api/merchant.go`：`publicComboItem.ImageURL` + `DishImageURLs []string` — `enrichPublicComboListImages` 重构为同时解析套餐自身图片和成员图片 URL
- [x] `api/favorite.go`：`favoriteMerchantResponse.MerchantLogoURL` + `favoriteDishResponse.ImageURL` — 两个 list 处理器各自批量填充
- [x] `api/membership.go`：`membershipResponse.LogoURL` — 单项 `getMerchantMembership` + 列表 `listUserMemberships` 各自注入
- [x] `api/operator_merchant_rider.go`：`merchantDetailResponse.LogoURL` — 提取响应体变量后注入
- [x] `api/review.go`：`reviewResponse.MerchantLogoURL` — `getMyReviews` 批量填充
- [x] `api/table.go`：`roomDetailResponse.ImageURLs []string`（图集）+ `PrimaryImageURL` + `MerchantLogoURL`；`roomListItemResponse.ImageURL` — 三处 handler 各自注入
- [x] `api/order.go`：`orderItemResponse.ImageURL` — `createOrder` + `getMerchantOrder` 两处循环后批量填充（`VariantCard`）
- [x] `api/group.go`：`groupMerchantResponse.LogoURL` + `brandResponse.LogoURL` — `listGroupMerchants`、`listGroupBrands`、`createGroupBrand`、`getBrand` 均已注入
- [x] `api/media_url.go` 中 enrich helper 函数集成测试覆盖（`api/media_url_test.go`，16 cases：`batchPublicImageURLs` ×5、`publicImageURL` ×3、`enrichCartImageURLs` ×2、`enrichSearchDishURLs` ×3、`enrichSearchMerchantURLs` ×2、`enrichSearchComboURLs` ×1；含 2 个 HTTP 回归测试覆盖 dish list/get）

---

## Phase 6：Web 端改造

### 6.1 媒体上传 SDK

- [x] `web/src/lib/media.ts`：实现 `createMediaUploadSession(req)`
- [x] `web/src/lib/media.ts`：实现 `ossDirectUpload(uploadHost, form, file)` — 直传 OSS POST Policy
- [x] `web/src/lib/media.ts`：实现 `completeMediaUpload(uploadId, objectKey, etag)` → 返回 `mediaId`
- [x] `web/src/lib/media.ts`：实现 `uploadMedia(file, options)` — 统一调用上面三步的入口函数
- [x] 删除 `web/src/lib/api.ts` 中所有 FormData 图片上传调用（`apiUpload` 函数已移除）

### 6.2 图片展示组件

- [x] 新增 `web/src/components/ui/media-image.tsx`：`<MediaImage>` 组件，封装 Next.js `<Image>`，优先直接消费后端返回的 CDN / 绝对 URL；开发态仅识别 dev-only `/dev/uploads/...` 路径，不再对历史 `uploads/...` 字符串做自动补全，加载出错时回退占位图
- [x] 列表页后端已通过 `MediaURLResolver` 返回 CDN 低规格图（`card`/`thumb`），前端直接使用 `image_url` 字段无需客户端变体选择
- [x] 详情页后端已返回 `detail` 规格图
- [x] 放大预览按需在调用处使用 `original` 变体（后端直接返回对应 URL）

### 6.3 各页面表单改造

- [x] **菜品创建 / 编辑页**：`uploadMedia` + 提交 `image_asset_id`（`dishes-page-client.tsx`）
- [x] **商户 logo 设置页**：`uploadMedia` + 提交 `logo_asset_id`（`settings-page-client.tsx`）
- [x] **桌台图片管理页**：`uploadMedia` + 提交 `media_asset_id`（`tables-page-client.tsx`）
- [x] **入驻申请页（商户）**：Web 端无独立申请页面，由小程序端（Phase 7）覆盖
- [x] **入驻申请页（骑手）**：同上
- [x] **入驻申请页（运营商）**：`operator/applyment/page.tsx` 仅提交银行账户信息，无图片上传，无需改造
- [x] **评价提交页**：Web 端评价为只读展示（商户回复），无图片上传功能

### 6.4 验收

- [ ] Web 端不再调用旧图片上传接口（network 面板 0 次旧接口调用）
- [ ] 所有上传链路：upload-sessions → OSS → complete 全流程成功
- [ ] 列表页图片来源为 CDN 域名

---

## Phase 7：小程序端改造

> 详见 `./MEDIA_WEAPP_MIGRATION_PLAYBOOK_2026-03-18.md`

### 7.1 媒体上传 SDK

- [x] `weapp/miniprogram/utils/media.ts`（新文件）：实现 `createMediaUploadSession(req)`
- [x] `weapp/miniprogram/utils/media.ts`：实现 `ossDirectUpload(uploadHost, form, filePath)` — 使用 `wx.uploadFile` 直传 OSS
- [x] `weapp/miniprogram/utils/media.ts`：实现 `completeMediaUpload(uploadId, objectKey)` → 返回 `mediaId`
- [x] `weapp/miniprogram/utils/media.ts`：实现 `uploadMedia(tempFilePath, options)` — 含客户端压缩（最长边 4096，JPEG 质量 0.82）
- [x] `weapp/miniprogram/utils/request.ts` 中旧业务接口的 `wx.uploadFile` 调用已从各 OCR 入口移除；所有入驻 OCR 流程统一改为 `uploadMedia` + `POST /v1/ocr/jobs`

### 7.2 图片读取

- [x] `weapp/miniprogram/utils/media.ts`：已实现 `getMediaDisplayUrl(url)` — 优先消费 CDN / 绝对 URL，开发态仅识别 dev-only `/dev/uploads/...` 路径，不再对历史 `uploads/...` 字符串做自动补全
- [x] `getPublicImageUrl`（`utils/image.ts`）已通过“http 开头直传”逻辑优先消费 CDN URL，并对开发态相对路径做补全，功能等价于兼容层
- [x] 菜单页、购物车页、订单确认页、预约页均经由 `getPublicImageUrl(item.image_url)` 读取，`image_url` 由后端通过 `MediaURLResolver` 注入 CDN 地址

### 7.3 各页面表单改造

- [x] **菜品图片上传页**：`DishManagementService.uploadDishImage` → `uploadMedia` + 提交 `image_asset_id`
- [x] **评价上传页**：`ReviewService.uploadReviewImage` → `uploadMedia` + 提交 `media_asset_ids`
- [x] **桌台图片上传页**：`tableManagementService.uploadTableImage` → `uploadMedia` + 提交 `media_asset_id`
- [x] **入驻申请 — 商户证照**：`api/onboarding.ts` 中 `ocrBusinessLicense` / `ocrFoodPermit` / `ocrIdCard` 均已调用 `uploadMedia` + 传 `media_asset_id`
- [x] **入驻申请 — 骑手证照**：`api/rider-application.ts` 中 `ocrRiderIdCard` / `ocrRiderHealthCert` 均已调用 `uploadMedia`
- [x] **入驻申请 — 运营商证照**：`api/operator-application.ts` 中 `ocrOperatorBusinessLicense` / `ocrOperatorIdCard` 均已调用 `uploadMedia`
- [x] **集团入驻申请**：`api/group-application.ts` 中 `ocrGroupBusinessLicense` 已改用 `uploadMedia` + `POST /v1/ocr/jobs`，通过统一 OCR 作业完成识别并刷新申请数据

### 7.4 验收

- [ ] 小程序不再通过旧业务接口上传图片
- [ ] wx 压缩失败时可 fallback 原图上传
- [ ] 各上传场景得到 `media_asset_id` 并成功提交

---

## Phase 8：旧链路下线

> **必须在 Web 和小程序均验证通过后才执行**

- [x] 在后端禁用 `POST /v1/dishes/images/upload`（返回 410 Gone）
- [x] 在后端禁用 `POST /v1/tables/images/upload`（返回 410 Gone）
- [x] 在后端禁用 `POST /v1/reviews/images/upload`（返回 410 Gone）
- [x] 在后端禁用 `POST /v1/merchants/images/upload`（返回 410 Gone）
- [x] 关闭 `/uploads/*filepath` 本地文件服务主路由（现已替换为仅供开发调试的 `/dev/uploads/*filepath`）
- [x] 将 `util/upload.go` 标记为 Deprecated，仅保留 local fallback 供开发环境
- [x] 将 `util/image.go` 标记为 Deprecated
- [x] 生产配置确认不再有 `UPLOADS_BASE_DIR` 依赖（app.env 注释已更新：旧链路已下线，生产无需填写）

---

## Phase 9：测试与验收

> 逐项对照 `MEDIA_TEST_AND_ACCEPTANCE_CHECKLIST_2026-03-18.md`

### 9.1 数据库层

- [x] 空库全量跑 migration 无错误（本地 PG，000001→000158，耗时 729ms，无报错）
- [ ] 旧库（从 000139）增量跑无错误
- [x] 枚举约束验证（upload_status, moderation_status）：CHECK 约束拦截非法值 ✅
- [x] 外键约束验证：`media_upload_sessions.media_asset_id` FK 生效 ✅
- [x] 唯一约束验证（object_key）：`idx_media_assets_object_key` 拦截重复插入 ✅

### 9.2 后端 API

- [x] upload-sessions 全场景测试（正常 + 401/400 + 幂等）✅ `api/media_test.go`
- [x] complete 全场景测试（正常 + 404/403/410/422 + 幂等）✅
- [x] private-access 全场景测试（正常 + 404/403 + 401）✅
- [x] delete 全场景测试（正常 + 403 + 404 + 400 + 401）✅
- [x] GET media/{id} 测试（公开/私有/403/404）✅
- [ ] 审计日志落地验证（private-access 访问记录）
- [ ] TTL 过期验证（签名 URL 到期后不可访问）

### 9.3 业务回归

- [ ] 菜品增删改查 + 图片展示
- [ ] 桌台图片增删改查
- [ ] 评价上传与展示
- [ ] 商户设置 logo
- [ ] 入驻申请材料上传与 OCR
- [ ] 订单列表、详情商品图片展示
- [ ] 搜索结果、购物车、预约页图片展示

### 9.4 OSS / CDN 集成

- [ ] 直传 OSS 成功，object_key 与会话一致
- [ ] 规格图 CDN URL 可正常访问（thumb / card / detail 三规格）
- [ ] 私有签名 URL 可用 + 过期后不可用
- [ ] CDN 命中率达预期（通过 CDN 控制台观察）

### 9.5 安全

- [ ] 普通用户无法上传商户专属 media_category
- [ ] 未授权用户无法访问他人私有媒体
- [ ] 非法 object_key 冒用无法完成 complete
- [ ] 签名地址过期后不可访问

### 9.6 性能基线

- [ ] 记录改造前列表页首屏图片总下载体积（基线）
- [ ] 记录改造后列表页首屏图片总下载体积
- [ ] 改造后应用服务 CPU 不随上传流量显著增长
- [ ] 列表页图片请求平均耗时有明显改善

---

## Phase 10：上线执行

> 严格按照 `./MEDIA_RELEASE_RUNBOOK_2026-03-18.md` 执行，以下为关键节点

- [ ] Runbook §4 上线前检查清单全部勾选
- [ ] `make migrateup` 执行成功，migration 后检查通过（Runbook §7.3）
- [ ] 后端新版本部署，`/v1/media/upload-sessions` 可访问（Runbook §8.3）
- [ ] Web 新版本部署，上传链路人工验证（Runbook §9.2）
- [ ] 小程序新版本发布，上传链路人工验证（Runbook §10.2）
- [ ] 公共图链路人工验收（Runbook §12.1）
- [ ] 私有图链路人工验收（Runbook §12.2）
- [ ] 旧链路下线（Phase 8）
- [ ] 监控观察窗口启动，持续观察 1 个完整业务高峰（Runbook §13）

---

## 任务总计与进度跟踪

| 阶段 | 任务数 | 完成数 |
|---|---|---|
| Phase 0 前置确认 | 8 | 8 |
| Phase 1 基础设施 | ~15 | 0 |
| Phase 2 数据库迁移 | 14 | 14 |
| Phase 3 后端配置 | 3 | 3 |
| Phase 4 媒体中心模块 | 14 | 14 |
| Phase 5 业务接口改造 | ~25 | 25 |
| Phase 6 Web 端 | ~15 | ~12 |
| Phase 7 小程序端 | ~15 | ~14 |
| Phase 8 旧链路下线 | 8 | 8 |
| Phase 9 测试验收 | ~20 | 9 |
| Phase 10 上线执行 | 10 | 0 |

> 提示：可按阶段分支管理（`feat/media-phase-1`、`feat/media-phase-2` 等），每阶段 PR Review 后合并主干。
