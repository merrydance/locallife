# 后端媒体模块代码结构设计稿

## 1. 文档目标

本文档用于指导 Go 后端在当前仓库结构下实现媒体系统最终态，明确：

1. 新媒体模块应放在哪些包中
2. 接口和实现如何分层
3. 依赖如何注入到 Server
4. 旧本地上传链路如何退场
5. 代码迁移顺序如何安排

本文档配套以下文档使用：

1. MEDIA_OSS_CDN_FINAL_IMPLEMENTATION_PLAN_2026-03-18.md
2. MEDIA_DATABASE_SCHEMA_DESIGN_2026-03-18.md
3. MEDIA_API_CONTRACT_DESIGN_2026-03-18.md
4. MEDIA_TEST_AND_ACCEPTANCE_CHECKLIST_2026-03-18.md

## 2. 当前后端结构判断

结合当前仓库代码，后端总体模式是：

1. api 包负责 HTTP handler、鉴权、路由与请求响应转换
2. logic 包承载跨 handler 的业务服务接口与实现
3. db/sqlc 负责查询与事务
4. worker 负责异步任务分发
5. util 提供配置、加解密、本地文件等基础能力

现有 Server 注入模式已经比较成熟：

1. util.Config 注入配置
2. db.Store 注入数据访问
3. wechatClient、taskDistributor、paymentFacade 等通过构造函数注入
4. Server struct 保存跨 handler 共用依赖

因此媒体系统不应做成“api 包里几个大文件加 util 函数”的形式，而应按现有项目习惯，拆成：

1. api 层 handler
2. logic 层 service/interface
3. db/sqlc 查询层
4. storage provider 抽象层

## 3. 设计原则

### 3.1 必须遵守的原则

1. 禁止新增包级全局状态。
2. 依赖必须通过构造函数注入。
3. 外部存储能力必须抽象成 Interface，不直接耦合具体 OSS SDK 到 handler。
4. Handler 只做参数解析、鉴权、调用 service、组织响应。
5. 媒体元数据持久化必须通过 db.Store 或新增的 sqlc 查询完成。

### 3.2 不采用的实现方式

以下方式不采用：

1. 在 util 下继续堆 upload_oss.go 并由 handler 直接调用
2. 在 api 层直接写 OSS SDK 调用逻辑
3. 在应用服务器同步完成图片转码或打水印
4. 继续保留业务 handler 直接接收 multipart 图片上传作为主链路

## 4. 推荐包结构

### 4.1 新增目录建议

建议新增以下目录或文件：

```text
locallife/
  api/
    media.go
    media_types.go
    media_errors.go
  logic/
    media_interfaces.go
    media_service.go
    media_service_test.go
    media_url_resolver.go
  media/
    policy.go
    storage.go
    storage_oss.go
    storage_local_dev.go
    variants.go
  db/query/
    media_asset.sql
    media_upload_session.sql
    review_image.sql
    merchant_application_image.sql
```
```

说明：

1. api 负责对外 HTTP 契约。
2. logic 负责编排上传会话、complete、私有授权、删除逻辑。
3. media 包负责纯媒体领域能力，如 policy、storage provider、variants 规则。
4. db/query 负责 SQL 源文件，由 sqlc 生成到 db/sqlc。

### 4.2 为什么新增独立 media 包

原因：

1. storage provider 不适合放 api 包
2. storage provider 也不适合直接放 logic 包，因为它更像基础设施适配层
3. media 包可以承载 OSS、本地 dev fallback、variants 与 media policy 等纯媒体领域能力

## 5. 分层职责定义

### 5.1 api 层

建议新增文件：

1. api/media.go
2. api/media_types.go
3. api/media_errors.go

职责：

1. 定义 upload session、complete、private access、delete、get media 的请求响应结构体
2. 完成 gin 参数绑定与认证信息提取
3. 调用 logic.MediaService
4. 返回统一 APIResponse

禁止事项：

1. 不直接访问 OSS
2. 不直接拼 object_key
3. 不直接查询 db/sqlc

### 5.2 logic 层

建议新增文件：

1. logic/media_interfaces.go
2. logic/media_service.go
3. logic/media_url_resolver.go

职责：

1. 编排 upload session 创建
2. 编排 complete 流程
3. 编排私有访问鉴权与签名
4. 编排媒体删除与引用检查
5. 输出 MediaAssetResponse 所需的 variants URL

### 5.3 media 包

建议新增文件：

1. media/policy.go
2. media/storage.go
3. media/storage_oss.go
4. media/storage_local_dev.go
5. media/variants.go

职责：

1. Policy：业务类型与媒体类型到 bucket/object_key 的映射规则
2. Storage：对象存储接口抽象
3. OSS 实现：CreateDirectUpload、StatObject、CreatePrivateDownloadURL、DeleteObject
4. LocalDev 实现：开发环境本地 fallback，仅用于本地联调
5. Variants：统一生成 thumb/card/detail/original URL 规则

### 5.4 db/sqlc 层

职责：

1. media_assets 的 CRUD
2. media_upload_sessions 的 CRUD
3. review_images 等关联表查询
4. 被引用检查查询

## 6. 核心接口设计

### 6.1 logic.MediaService

建议定义：

```go
type MediaService interface {
    CreateUploadSession(ctx context.Context, input CreateMediaUploadSessionInput) (CreateMediaUploadSessionResult, error)
    CompleteUpload(ctx context.Context, input CompleteMediaUploadInput) (MediaAssetView, error)
    CreatePrivateAccessURL(ctx context.Context, input CreatePrivateAccessURLInput) (CreatePrivateAccessURLResult, error)
    GetMedia(ctx context.Context, input GetMediaInput) (MediaAssetView, error)
    DeleteMedia(ctx context.Context, input DeleteMediaInput) error
}
```

### 6.2 media.ObjectStorage

建议定义：

```go
type ObjectStorage interface {
    CreateDirectUpload(ctx context.Context, req CreateDirectUploadRequest) (CreateDirectUploadResult, error)
    StatObject(ctx context.Context, bucketType string, objectKey string) (ObjectMetadata, error)
    CreatePrivateDownloadURL(ctx context.Context, bucketType string, objectKey string, ttl time.Duration) (string, error)
    DeleteObject(ctx context.Context, bucketType string, objectKey string) error
}
```

### 6.3 media.PolicyResolver

建议定义：

```go
type PolicyResolver interface {
    ResolveUploadPolicy(ctx context.Context, input ResolveUploadPolicyInput) (ResolvedUploadPolicy, error)
}
```

### 6.4 logic.MediaURLResolver

建议定义：

```go
type MediaURLResolver interface {
    ResolvePublicVariants(asset MediaAssetView) MediaVariantURLs
    ResolvePrivateOriginal(ctx context.Context, asset MediaAssetView, ttl time.Duration) (string, error)
}
```

## 7. 推荐输入输出模型

### 7.1 logic 输入模型

建议新增以下 input/result 结构：

1. CreateMediaUploadSessionInput
2. CreateMediaUploadSessionResult
3. CompleteMediaUploadInput
4. CreatePrivateAccessURLInput
5. CreatePrivateAccessURLResult
6. DeleteMediaInput
7. GetMediaInput
8. MediaAssetView
9. MediaVariantURLs

### 7.2 MediaAssetView 角色

MediaAssetView 是 logic 层的稳定输出 DTO，供：

1. api 层直接序列化返回
2. 业务响应层做图片字段 hydration

它不是 db/sqlc model，也不是直接的 Swagger 请求体。

## 8. Server 注入改造

### 8.1 Server 新增字段

建议在 Server 中新增：

1. mediaService logic.MediaService

如果你们想在 api 层直接复用 URL hydration helper，也可以加：

2. mediaURLResolver logic.MediaURLResolver

但更推荐统一通过 mediaService 暴露读取能力。

### 8.2 NewServer 中的构造顺序

建议在 NewServer 中新增构造顺序：

1. 构造 objectStorage
2. 构造 policyResolver
3. 构造 mediaURLResolver
4. 构造 mediaService
5. 注入到 Server

伪代码：

```go
objectStorage := media.NewOSSStorage(config)
policyResolver := media.NewPolicyResolver(config)
mediaURLResolver := logic.NewMediaURLResolver(config, objectStorage)
mediaService := logic.NewMediaService(store, objectStorage, policyResolver, mediaURLResolver)

server := &Server{
    ...
    mediaService: mediaService,
}
```

### 8.3 开发环境 fallback

如果开发环境暂时没有 OSS，可在 NewServer 中根据配置选择：

1. production: OSSStorage
2. development/test: LocalDevStorage 或 fake storage

但生产路径必须固定使用 OSSStorage。

## 9. SQL 与 Store 改造建议

### 9.1 新增 sqlc 查询文件

建议新增：

1. db/query/media_asset.sql
2. db/query/media_upload_session.sql
3. db/query/review_image.sql
4. db/query/merchant_application_image.sql

### 9.2 Store 扩展方式

推荐方式：

1. 继续由 sqlc 生成 Queries
2. 在 db.Store 接口中扩展需要的媒体查询方法

不要新建绕过 db.Store 的“第二套数据库访问层”。

### 9.3 需要的基础查询

至少包括：

1. CreateMediaAsset
2. GetMediaAsset
3. GetMediaAssetByObjectKey
4. UpdateMediaAssetStatus
5. SoftDeleteMediaAsset
6. CreateMediaUploadSession
7. GetMediaUploadSessionByUploadID
8. CompleteMediaUploadSession
9. CheckMediaAssetReference

## 10. 旧代码退场策略

### 10.1 第一类：直接退场的代码

以下逻辑在最终态中不再是主链路：

1. util/upload.go
2. util/image.go
3. api/dish_upload.go
4. api/review_upload.go
5. api/table_upload.go
6. api/merchant.go 中 uploadMerchantImage handler
7. api/upload_signed.go 中本地文件签名读取主流程
8. router.GET("/uploads/*filepath", ...)

### 10.2 第二类：需要改写后保留的代码

以下逻辑需要保留概念，但实现要改：

1. 现有签名下载权限判断逻辑
2. 现有图片 URL 归一化逻辑
3. 现有 OCR 与微信上传逻辑中对图片对象的读取能力

### 10.3 退场顺序

建议顺序：

1. 新媒体接口上线
2. Web/小程序切换到新媒体接口
3. 业务写接口改为接受 media_asset_id
4. 业务读接口改为基于 media_asset_id 输出图片字段
5. 下线旧上传接口
6. 下线 /uploads 本地主路由

## 11. URL Hydration 统一方案

### 11.1 当前问题

当前多个 handler 直接使用 normalizeUploadURLForClient，将 db 中字符串路径转换成 URL。

最终态不能继续让各个 handler 自己判断路径类型。

### 11.2 推荐做法

新增统一 helper，例如：

1. logic/media_url_resolver.go 中的 resolver
2. api 层可包装成 hydrateMediaFields 或 mapMediaURL helper

要求：

1. 单图字段统一根据 media_asset_id hydrate
2. 多图字段统一批量 hydrate
3. 页面所需的默认规格统一由后端决定

### 11.3 禁止事项

禁止：

1. handler 中再手工拼 CDN URL
2. handler 中通过字符串前缀判断 public/private 并拼 URL

## 12. OCR 与外部上传链路适配

### 12.1 当前问题

当前部分 OCR 流程和微信支付入驻上传流程仍依赖本地路径或 uploads 目录。

典型场景：

1. 商户申请 OCR
2. 运营商申请 OCR
3. 微信上传证照到支付侧接口

### 12.2 目标改造

这些链路要从“读取本地文件”改成“通过 media_asset_id 解析对象并拉取字节流”。

建议新增能力：

```go
type MediaBinaryReader interface {
    ReadObject(ctx context.Context, bucketType string, objectKey string) ([]byte, string, error)
}
```

说明：

1. 这不是公开对外接口，而是服务端内部给 OCR/外部 API 上传使用。
2. 不应再假设文件存在于本地 uploads 目录。

## 13. 媒体异步任务边界

### 13.1 本次不引入的能力

本次不把以下能力做进同步主链路：

1. 自研图片转码
2. 自研固化水印
3. 离线多规格图回写

### 13.2 若未来需要扩展

未来如需要异步媒体处理，可在 worker 中新增：

1. task_process_media_asset
2. task_generate_watermarked_variant

但本次上线目标中，媒体 service 本身不依赖这些任务存在。

## 14. 文件级改造清单

### 14.1 新增文件

建议新增：

1. api/media.go
2. api/media_types.go
3. api/media_errors.go
4. logic/media_interfaces.go
5. logic/media_service.go
6. logic/media_url_resolver.go
7. media/policy.go
8. media/storage.go
9. media/storage_oss.go
10. media/storage_local_dev.go
11. media/variants.go

### 14.2 必改文件

1. api/server.go
2. util/config.go
3. db/sqlc 生成物对应 SQL
4. 业务 handler 中所有 image_url 校验逻辑
5. OCR/入驻/微信上传相关逻辑

### 14.3 删除或废弃文件

1. api/dish_upload.go
2. api/review_upload.go
3. api/table_upload.go
4. util/upload.go 生产主用途
5. util/image.go 生产主用途

## 15. 推荐实施顺序

### 阶段 1：基础设施适配

1. 扩展 util.Config
2. 实现 media/storage.go 接口与 OSS 实现
3. 实现 media/policy.go

### 阶段 2：数据库与 service

1. 添加 migration
2. 添加 sqlc 查询
3. 实现 logic/media_service.go
4. 实现 logic/media_url_resolver.go

### 阶段 3：HTTP 接口

1. 新增 api/media.go
2. 注册 /v1/media 路由
3. 补齐 Swagger 注释

### 阶段 4：业务切换

1. 修改业务写接口接受 media_asset_id
2. 修改业务读接口基于 media_asset_id 输出图片字段
3. 修改 OCR/外部上传依赖本地文件的链路

### 阶段 5：旧链路下线

1. 删除旧图片上传接口路由
2. 删除本地 /uploads 主链路

## 16. 测试落点建议

### 16.1 api 层测试

新增：

1. api/media_test.go

覆盖：

1. upload-sessions
2. complete
3. private-access
4. delete

### 16.2 logic 层测试

新增：

1. logic/media_service_test.go
2. logic/media_url_resolver_test.go

### 16.3 storage 层测试

新增：

1. media/storage_oss_test.go
2. media/policy_test.go

## 17. 代码审查重点

评审媒体模块实现时必须重点看：

1. Handler 是否仍直接依赖 OSS SDK
2. 是否存在新的本地文件系统依赖
3. 是否把 object_key 泄漏进业务表写入
4. 是否仍在 handler 内部手工拼 URL
5. 是否保持依赖注入和 interface 抽象

## 18. 完成标准

后端模块层面视为完成的标准：

1. media service 已通过构造函数注入到 Server
2. 新媒体接口已可用
3. ObjectStorage 已封装 OSS 访问
4. 业务接口已不再持久化 image_url 或 object_key
5. 业务读接口已通过 media_asset_id hydrate 图片字段
6. 旧本地上传主链路已退出生产路径
