# 媒体系统最终态实施方案

## 1. 文档目标

本文档用于指导 Locallife 在正式上线前一次性完成图片媒体系统改造，直接落成生产级最终态，避免后续二次返工。

本文档关注以下目标：

1. 应用服务器退出图片上传、图片转码、公共图片分发链路。
2. 公共图片全部迁移至 OSS 存储并通过 CDN 加速访问。
3. 私有敏感图片全部迁移至私有 OSS 并通过短期签名地址访问。
4. Web 与小程序统一改为直传 OSS，不再通过业务接口直接上传图片文件。
5. 前端默认读取规格图，不再读取原图，彻底解决大图加载慢的问题。
6. 数据库存储统一收敛为 media_asset_id，不再存完整 URL。

## 2. 当前问题与根因

### 2.1 当前实现

当前图片链路的核心行为如下：

1. 业务接口直接接收 multipart/form-data 图片上传。
2. 应用服务器执行图片内容安全检查。
3. 应用服务器写本地 uploads 目录。
4. 部分公共图在应用服务器内调用 cwebp 尝试转为 WebP。
5. 公共图和私有图都通过应用服务的本地路径或签名下载方式对外提供。

当前关键代码位置：

1. 本地上传器：util/upload.go
2. 本机 WebP 转码：util/image.go
3. 本地图片访问路由：api/server.go
4. 本地签名文件访问：api/upload_signed.go
5. Web 端 FormData 上传：web/src/lib/api.ts
6. 小程序 wx.uploadFile 上传：weapp/miniprogram/utils/request.ts

### 2.2 当前问题

1. 应用服务器承担大文件上传流量、文件 IO、图片转码、图片分发，资源压力大。
2. 转码逻辑依赖应用容器环境，失败后回退原图，缺乏稳定产物保障。
3. 前端大量直接读取原图，导致移动端和弱网场景加载明显偏慢。
4. 数据库存储语义混乱，既有 uploads 相对路径，也有 URL 语义字段，后续迁移成本高。
5. 上传能力分散在多个业务接口，难以统一审计、监控、幂等和策略管理。

### 2.3 根因

根因不是“未上 OSS”这么简单，而是当前读写链路都耦合在应用服务器：

1. 写链路没有从业务服务解耦。
2. 转码没有从业务服务解耦。
3. 读链路没有规格化图片输出。
4. 媒体元数据没有独立建模。

## 3. 最终态架构原则

### 3.1 总原则

1. 应用服务器不接收图片大文件主流量。
2. 应用服务器不承担图片转码工作。
3. 应用服务器不承担公共图片下发工作。
4. OSS 负责对象存储，CDN 负责公共图片分发和加速。
5. 私有图片只允许鉴权后获得短期访问地址。
6. 数据库存储对象标识，不存最终访问域名。

### 3.2 目标架构

公共图片链路：

1. 客户端向后端申请上传会话。
2. 后端校验权限并签发 OSS 直传凭证。
3. 客户端直传 OSS 公共桶。
4. 客户端回调后端确认上传完成。
5. 后端写入媒体元数据与业务关联。
6. 前端通过 CDN 地址读取规格图。

私有图片链路：

1. 客户端向后端申请上传会话。
2. 后端校验权限并签发 OSS 私有桶直传凭证。
3. 客户端直传私有桶。
4. 客户端回调后端确认上传完成。
5. 后端写入媒体元数据与业务关联。
6. 下载时后端再次鉴权，并签发短期私有访问地址。

## 4. 存储设计

### 4.1 桶设计

直接使用双桶，不使用单桶混合 public/private 前缀作为最终态。

| 桶 | 用途 | 访问方式 | 缓存 | 说明 |
|---|---|---|---|---|
| media-public | logo、菜品图、桌台图、评价图、头像、二维码等公共素材 | 仅通过 CDN 域名访问 | 长缓存 | 源桶不直接向终端暴露 |
| media-private | 营业执照、身份证、健康证、OCR 材料等敏感素材 | 鉴权后获取短期签名地址 | 不公开缓存 | 严格私有 |

### 4.2 对象键规范

所有对象键必须稳定、可推导、无业务域名信息。

公共图片示例：

```text
uploads/public/merchants/{merchant_id}/logo/{sha256}.jpg
uploads/public/merchants/{merchant_id}/storefront/{sha256}.jpg
uploads/public/merchants/{merchant_id}/environment/{sha256}.jpg
uploads/public/merchants/{merchant_id}/dishes/{sha256}.jpg
uploads/public/merchants/{merchant_id}/tables/{sha256}.jpg
uploads/public/reviews/{user_id}/{sha256}.jpg
uploads/public/avatars/{user_id}/{sha256}.jpg
uploads/public/qrcodes/{merchant_id}/{sha256}.png
```

私有图片示例：

```text
uploads/private/merchants/{user_id}/business_license/{sha256}.jpg
uploads/private/merchants/{user_id}/food_permit/{sha256}.jpg
uploads/private/merchants/{user_id}/id_front/{sha256}.jpg
uploads/private/merchants/{user_id}/id_back/{sha256}.jpg
uploads/private/riders/{user_id}/idcard/{sha256}.jpg
uploads/private/riders/{user_id}/healthcert/{sha256}.jpg
uploads/private/operators/{user_id}/license/{sha256}.jpg
uploads/private/operators/{user_id}/idcard_front/{sha256}.jpg
uploads/private/operators/{user_id}/idcard_back/{sha256}.jpg
```

### 4.3 命名规则

使用 sha256 命名的原因：

1. 天然去重。
2. 支持内容寻址，减少同图重复上传。
3. 强缓存友好，避免覆盖同名文件。
4. 便于迁移、校验和审计。

## 5. CDN 与图片处理设计

### 5.1 CDN 设计

公共图片必须统一走 CDN 域名，例如：

```text
https://cdn.locallife.example.com/
```

CDN 设计要求：

1. 仅加速公共桶内容。
2. 回源地址使用 OSS 公共桶源站域名。
3. OSS 源站不直接暴露给客户端。
4. 启用 HTTPS。
5. 启用 HTTP/2。
6. 启用 Brotli 或 Gzip。
7. 启用长缓存。
8. 使用文件名哈希保证缓存安全。
9. 忽略无关 query 参数，避免缓存碎片。

> **注意：CDN 动态图片处理（规格图）为按需生成，首次访问存在处理延迟。高频业务图片（如菜单菜品图、商户列表图）上线后应提前执行 CDN 预热，避免首屏集中冷启动。**

### 5.2 图片处理能力

最终态不依赖应用服务器转码，改为使用 OSS 或 CDN 自带图片处理能力，动态输出规格图。

要求支持：

1. 等比缩放。
2. 指定宽度或长边限制。
3. 自动格式转换为 WebP。
4. 有条件时支持 AVIF。
5. 质量压缩。
6. 可选中心裁切。

### 5.2.1 水印能力

最终态优先使用 OSS 或 CDN 原生图片处理能力打水印，不在应用服务器内自行实现水印写入。

推荐策略：

1. 公共图片默认不加水印，保持展示质量与缓存命中率。
2. 仅对确有业务需要的图片启用水印，例如审核外发图、招商素材、带版权保护的运营图。
3. 优先使用对象存储或 CDN 的动态水印能力，在图片访问 URL 的样式规则中附加水印参数。
4. 若未来要求“水印后的文件必须固化为独立对象”，再增加异步媒体处理任务生成衍生对象，不在本次主改造中阻塞上线。

说明：

1. 主流 OSS 产品通常具备基础水印能力，常见包括文字水印、图片水印、位置、透明度、缩放等。
2. 若选用的 OSS 厂商原生不支持，CDN 图像处理层通常也能提供等价能力。
3. 自研打水印会显著扩大本次改造范围，因为它意味着重新引入服务端图像处理流水线，与“应用服务器退出媒体处理链路”的目标相冲突。

### 5.3 规格体系

定义统一规格，禁止前端自行约定。

| 规格 | 典型用途 | 目标宽度 |
|---|---|---|
| thumb | 列表、头像、小卡片 | 200 |
| card | 商品卡片、搜索结果、商户列表 | 400 |
| detail | 详情页主图 | 960 |
| original | 大图预览、审核、下载 | 原图 |

说明：

1. 列表页只允许使用 thumb 或 card。
2. 详情页只允许使用 detail。
3. original 只用于预览、审核、下载。

## 6. 数据模型设计

### 6.1 新增 media_assets 表

新增统一媒体元数据表：media_assets。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint | 主键 |
| object_key | text | OSS 对象键，唯一 |
| visibility | text | public 或 private，决定路由到哪个桶 |
| media_category | text | logo、dish、table、review、business_license 等 |
| mime_type | text | image/jpeg、image/png 等 |
| file_size | bigint | 文件大小 |
| width | integer | 原图宽度 |
| height | integer | 原图高度 |
| checksum_sha256 | text | 内容哈希 |
| upload_status | text | pending、uploaded、confirmed、failed、deleted |
| moderation_status | text | pending、approved、rejected、quarantined |
| uploaded_by | bigint | 上传用户 ID |
| source_client | text | web、weapp、operator-web 等 |
| created_at | timestamptz | 创建时间 |
| updated_at | timestamptz | 更新时间 |
| deleted_at | timestamptz | 软删除时间 |

建议索引：

1. unique index on object_key
2. index on uploaded_by
3. index on media_category
4. index on visibility, moderation_status
5. index on created_at desc

### 6.2 业务表字段策略

由于当前业务表和响应结构大量使用 image_url、logo_url、avatar_url 等字段，建议采用以下方案：

1. 对外 API 字段名可短期保留，减少前端改造面。
2. 数据库存储不再保存 object_key 到业务表图片字段中。
3. 业务表统一改为保存 media_asset_id。
4. object_key、bucket_type、visibility、mime_type 等信息统一收口在 media_assets 表。
5. 返回给前端时通过 media_asset_id 关联 media_assets，再由 MediaURLResolver 输出公共规格图 URL 或私有签名地址。

### 6.3 是否直接改成 media_asset_id

本方案直接采用最终目标：业务表统一引用 media_asset_id，不再采用“业务表先存 object_key”的过渡模式。

原因如下：

1. 当前服务尚未正式上线，历史包袱小，适合一次性做正。
2. media_asset_id 语义更稳定，不会把存储细节泄漏到业务表。
3. 后续替换 OSS 厂商、CDN 域名、对象键规则时，业务表无需变更。
4. 更利于一图多用途、多规格、审核状态、软删除、审计等统一管理。
5. 更利于 future-proof 设计，例如视频、PDF、合同扫描件等统一进入媒体中心。

落地要求：

1. 所有新增或重构的图片关系字段统一使用 media_asset_id bigint。
2. media_asset_id 应建立外键到 media_assets.id。
3. 对于一对多图片集合场景，不再在主表中存图片 URL 数组，而是改为单独关联表。
4. 对于历史仅允许单图的字段，如 logo、avatar、primary_image，可直接替换为 media_asset_id。
5. API 响应层允许暂时继续返回 image_url、logo_url 等字段，但这些字段由 media_asset_id 反查生成。

推荐的业务建模方式：

1. 单图字段：主表增加 media_asset_id 外键。
2. 多图字段：新增资源图片关联表，字段包括 resource_id、media_asset_id、sort_order、role。
3. 主图场景：通过 role=primary 或单独主图字段表达，不再依赖数组第一个元素约定。

典型改造示例：

1. merchants.logo_url -> merchants.logo_media_asset_id
2. dishes.image_url -> dishes.image_media_asset_id
3. users.avatar_url -> users.avatar_media_asset_id
4. table_images.image_url -> table_images.media_asset_id
5. review_images.image_url -> review_images.media_asset_id

## 7. 后端模块设计

### 7.1 新增模块边界

新增 media 模块，拆分为四层：

1. MediaPolicy
2. ObjectStorage
3. MediaRegistry
4. MediaURLResolver

### 7.2 MediaPolicy

职责：

1. 校验谁可以上传什么类型媒体。
2. 决定媒体是否 public/private。
3. 决定 object_key 前缀。
4. 限制最大文件大小、允许格式和数量。

输入参数：

1. user_id
2. business_type
3. media_category
4. content_type
5. content_length

输出参数：

1. visibility
2. bucket_type
3. object_key
4. policy constraints

### 7.3 ObjectStorage

职责：

1. 生成直传凭证。
2. 生成直传表单或签名信息。
3. 确认对象是否存在。
4. 读取对象元数据。
5. 生成私有下载签名地址。
6. 删除对象。

建议接口：

```go
type ObjectStorage interface {
    CreateDirectUpload(ctx context.Context, req CreateDirectUploadRequest) (CreateDirectUploadResult, error)
    StatObject(ctx context.Context, bucket string, objectKey string) (ObjectMetadata, error)
    CreatePrivateDownloadURL(ctx context.Context, bucket string, objectKey string, ttl time.Duration) (string, error)
    DeleteObject(ctx context.Context, bucket string, objectKey string) error
}
```

### 7.4 MediaRegistry

职责：

1. 创建上传会话记录。
2. 确认上传完成。
3. 写入 media_assets。
4. 建立业务绑定。
5. 负责状态流转。
6. 对外暴露基于 media_asset_id 的查询能力。

### 7.5 MediaURLResolver

职责：

1. 将 object_key 解析为公共 CDN 地址。
2. 将 object_key 解析为规格图地址。
3. 将私有 object_key 转为短期签名地址。

该模块将替代当前所有 normalizeUploadURLForClient 等路径转换逻辑。

## 8. API 设计

### 8.1 申请上传会话

接口：

```text
POST /v1/media/upload-sessions
```

请求示例：

```json
{
  "business_type": "merchant_dish",
  "media_category": "dish",
  "visibility": "public",
  "filename": "dish.jpg",
  "content_type": "image/jpeg",
  "content_length": 438221,
  "checksum_sha256": "..."
}
```

返回示例：

```json
{
  "upload_id": "up_01J...",
  "object_key": "uploads/public/merchants/1001/dishes/abc123.jpg",
  "bucket_type": "public",
  "upload_host": "https://oss-direct.example.com",
  "expire_at": "2026-03-18T10:00:00Z",
  "form": {
    "policy": "...",
    "signature": "...",
    "access_key_id": "...",
    "dir": "uploads/public/merchants/1001/dishes/"
  }
}
```

说明：

1. 申请上传会话阶段仍会返回 object_key，因为客户端直传 OSS 时需要使用该对象键。
2. object_key 仅用于上传协议与存储层，不写入业务表。
3. 业务侧最终持久化使用 media_asset_id。

### 8.2 确认上传完成

接口：

```text
POST /v1/media/complete
```

请求示例：

```json
{
  "upload_id": "up_01J...",
  "object_key": "uploads/public/merchants/1001/dishes/abc123.jpg",
  "etag": "...",
  "bind": {
    "resource_type": "dish",
    "resource_id": 0
  }
}
```

返回示例：

```json
{
  "media_id": 123,
  "visibility": "public",
  "urls": {
    "thumb": "https://cdn.locallife.example.com/...",
    "card": "https://cdn.locallife.example.com/...",
    "detail": "https://cdn.locallife.example.com/...",
    "original": "https://cdn.locallife.example.com/..."
  },
  "status": "approved"
}
```

### 8.3 获取私有访问地址

接口：

```text
POST /v1/media/private-access
```

请求示例：

```json
{
  "media_id": 123,
  "reason": "merchant_audit_review"
}
```

说明：

1. 客户端只持有 media_id，不持有 object_key。
2. 后端根据 media_id 查询 media_assets，鉴权后再解析出 object_key 签发地址。
3. object_key 不下发到客户端，避免内部存储路径泄漏。

返回示例：

```json
{
  "download_url": "https://private-download.example.com/...",
  "expire_at": "2026-03-18T10:05:00Z"
}
```

### 8.4 删除媒体

接口：

```text
DELETE /v1/media/{id}
```

行为要求：

1. 软删除 media_assets 记录。
2. 异步删除 OSS 对象。
3. 写审计日志。
4. 删除失败进入重试队列。

## 9. 公共图读链路规范

### 9.1 后端返回规范

公共图推荐统一返回以下结构：

```json
{
  "image": {
    "media_asset_id": 123,
    "thumb_url": "https://cdn.locallife.example.com/...",
    "card_url": "https://cdn.locallife.example.com/...",
    "detail_url": "https://cdn.locallife.example.com/...",
    "original_url": "https://cdn.locallife.example.com/..."
  }
}
```

若当前响应结构不便一次改完，则短期折中要求：

1. image_url 默认返回 card_url 或 detail_url。
2. 新增可选字段 media_asset_id、original_image_url 或 image_variants。

### 9.2 前端使用规范

1. 列表页、搜索页、卡片页使用 thumb 或 card。
2. 详情页使用 detail。
3. 放大预览时使用 original。
4. 禁止前端自己拼接图片规格参数。

## 10. 私有图读链路规范

1. 业务侧仅存 media_asset_id，不直接持久化 object_key。
2. 获取私有图前必须通过业务接口鉴权。
3. 后端签发短期访问地址，默认 5 分钟有效。
4. 前端不得缓存永久私有 URL。
5. 关键敏感下载需记录 request_id、user_id、object_key、reason。

## 11. 上传客户端改造

### 11.1 Web 端改造

当前问题：

1. web/src/lib/api.ts 使用 FormData 上传。
2. 页面组件直接调用业务图片上传接口。

目标改造：

1. 新增 createMediaUploadSession。
2. 新增 completeMediaUpload。
3. 新增 ossDirectUpload 工具。
4. 所有页面统一改为“申请上传会话 -> 直传 -> complete -> 提交业务表单”。

Web 端标准流程：

1. 用户选择图片。
2. 浏览器执行轻量预压缩。
3. 调用 /v1/media/upload-sessions。
4. 使用返回的表单字段直传 OSS。
5. 调用 /v1/media/complete。
6. 获取 media_asset_id。
7. 将 media_asset_id 写入后续业务提交数据。

### 11.2 小程序改造

当前问题：

1. weapp/miniprogram/utils/request.ts 直接封装 wx.uploadFile 到业务接口。
2. 多个模块重复上传逻辑。

目标改造：

1. 新增 createMediaUploadSession。
2. 新增 wx 直传 OSS 封装。
3. 新增 completeMediaUpload。
4. 删除所有业务侧图片上传接口调用。

小程序标准流程：

1. 用户选择图片。
2. 小程序端进行尺寸和质量压缩。
3. 调用 /v1/media/upload-sessions。
4. 直传 OSS。
5. 调用 /v1/media/complete。
6. 返回 media_asset_id。
7. 业务接口提交 media_asset_id。

### 11.3 客户端预压缩要求

客户端预压缩不是最终图片处理，只是上传优化，要求如下：

1. 最长边不超过 4096。
2. 清理 EXIF 方向问题。
3. JPEG 质量默认 0.82 左右。
4. 超大 PNG 按策略转 JPEG，仅在不影响透明需求时执行。

## 12. 内容安全与审核

### 12.1 公共图

公共图上传后进入 pending 状态，异步执行内容安全审核。

状态流转：

1. pending
2. approved
3. rejected
4. quarantined

要求：

1. 前端默认只展示 approved 图。
2. 上传者本人可立即看到自己刚上传的 pending 状态图片（不展示占位图），其他用户等待 approved 后才可见。
3. pending 状态对非上传者显示占位图。
4. 审核失败写审计日志并通知业务方。

### 12.2 私有图

私有图允许上传后立即绑定业务，但仍需后台异步执行：

1. OCR 识别。
2. 合规扫描。
3. 风险告警。

## 13. 配置设计

在 util.Config 中新增以下配置：

| 配置项 | 用途 |
|---|---|
| FILE_STORAGE_PROVIDER | local 或 oss，生产固定 oss |
| OSS_PUBLIC_ENDPOINT | 公共桶上传或管理端点 |
| OSS_PRIVATE_ENDPOINT | 私有桶上传或管理端点 |
| OSS_PUBLIC_BUCKET | 公共桶名称 |
| OSS_PRIVATE_BUCKET | 私有桶名称 |
| OSS_REGION | 存储区域 |
| OSS_STS_ROLE_ARN | 直传临时授权角色 |
| OSS_STS_EXTERNAL_ID | STS 额外约束 |
| CDN_PUBLIC_BASE_URL | 公共 CDN 域名 |
| PRIVATE_DOWNLOAD_URL_TTL | 私有签名下载 TTL |
| MEDIA_MAX_UPLOAD_BYTES | 单图最大上传大小 |
| MEDIA_ALLOWED_IMAGE_TYPES | 允许的图片类型 |
| MEDIA_DIRECT_UPLOAD_EXPIRE | 直传凭证有效期 |
| IMAGE_VARIANT_THUMB_WIDTH | thumb 宽度 |
| IMAGE_VARIANT_CARD_WIDTH | card 宽度 |
| IMAGE_VARIANT_DETAIL_WIDTH | detail 宽度 |

说明：

1. 生产环境不再依赖 UPLOADS_BASE_DIR 作为主路径。
2. 本地 uploads 仅可保留开发环境 fallback。

## 14. 安全要求

1. 私有桶必须关闭公开访问。
2. 公共桶也不直接裸露给客户端，统一走 CDN。
3. 上传权限使用 STS 或等价临时授权，禁止长期 AK/SK 下发到客户端。
4. 必须校验 content_type、content_length、checksum_sha256。
5. 必须校验业务上传类别与 object_key 前缀一致。
6. 私有访问签名必须短期有效，并记录访问审计。

## 15. 幂等与一致性设计

### 15.1 上传会话幂等

对于同一用户、同一业务对象、同一 checksum、同一 media_category，可复用未完成上传会话，避免重复会话膨胀。

### 15.2 complete 幂等

complete 接口必须幂等，若已确认成功：

1. 不重复创建 media_assets。
2. 直接返回已存在记录。
3. 不重复创建业务关联关系。

### 15.3 删除一致性

删除流程：

1. 业务记录解除引用。
2. media_assets 标记 deleted。
3. 异步删除 OSS 对象。
4. 删除失败进入重试队列。

## 16. 可观测性要求

必须新增以下指标：

1. 上传会话申请成功率。
2. 上传完成确认成功率。
3. OSS 直传失败率。
4. 私有签名地址生成失败率。
5. 公共图 CDN 命中率。
6. 图片审核通过率。
7. 图片删除重试次数。
8. 单规格图片访问耗时。

必须新增以下日志字段：

1. request_id
2. user_id
3. object_key
4. media_category
5. visibility
6. upload_id

## 17. 施工清单

### 17.1 后端施工项

1. 新增 migration，创建 media_assets 表。
2. 新增 media 包。
3. 新增 OSS ObjectStorage 实现。
4. 新增 MediaPolicy。
5. 新增 MediaRegistry。
6. 新增 MediaURLResolver。
7. 新增 /v1/media/upload-sessions。
8. 新增 /v1/media/complete。
9. 新增 /v1/media/private-access。
10. 新增 /v1/media/{id} 删除接口。
11. 将业务表图片字段改为 media_asset_id 或资源关联表。
12. 全量替换业务响应中的图片 URL 生成逻辑。
13. 下线本地上传主链路。
14. 下线 /uploads 本地文件服务主链路。

### 17.2 Web 施工项

1. 新增媒体上传 SDK。
2. 删除 FormData 业务上传依赖。
3. 页面统一走上传会话 + 直传 + complete。
4. 表单提交流程统一提交 media_asset_id。
5. 图片组件统一支持 variants。
6. 列表页默认读取 thumb 或 card。

### 17.3 小程序施工项

1. 新增 OSS 直传封装。
2. 删除 wx.uploadFile 到业务接口的老链路。
3. 页面统一走上传会话 + 直传 + complete。
4. 表单提交流程统一提交 media_asset_id。
5. 列表页统一读取公共规格图。

### 17.4 基础设施施工项

1. 创建 public/private 双桶。
2. 配置 CDN 公共域名。
3. 配置 OSS 图片处理能力。
4. 配置 STS 临时授权。
5. 配置日志、监控和审计。

## 18. 建议开发顺序

### 阶段 1：基础设施与数据层

1. 创建 OSS 双桶。
2. 配置 CDN。
3. 配置图片处理样式。
4. 新增 media_assets 表。
5. 扩展 util.Config。

### 阶段 2：后端媒体中心

1. 实现 ObjectStorage。
2. 实现 MediaPolicy。
3. 实现 MediaRegistry。
4. 实现 MediaURLResolver。
5. 开放统一媒体接口。

### 阶段 3：客户端改造

1. 改造 Web 直传。
2. 改造小程序直传。
3. 改造图片组件只读取规格图。

### 阶段 4：业务路由替换

1. 替换所有图片上传入口。
2. 替换所有图片 URL 返回逻辑。
3. 下线本地 uploads 主链路。

## 19. 上线验收标准

满足以下条件才视为完成：

1. 应用服务器不再接收图片上传主流量。
2. 应用服务器不再执行图片转码。
3. 应用服务器不再提供公共图片分发。
4. 公共图片全部通过 CDN 域名访问。
5. 私有图片全部通过短期签名地址访问。
6. 所有公共业务接口默认返回规格图地址。
7. 列表页和搜索页不再请求 original 图。
8. 图片上传、确认、删除、下载具备完整监控与审计。
9. 生产环境不再依赖本地 uploads 目录。
10. 当前“读大图慢”问题在首屏和列表页得到明显改善。

## 20. 明确废弃项

以下能力在最终态中不再作为生产主链路保留：

1. util/upload.go 本地上传器。
2. util/image.go 本机 WebP 转码器。
3. /uploads/*filepath 本地文件访问路由。
4. 业务接口直接接收 multipart 图片上传。
5. 业务表中存完整图片访问 URL 的模式。

## 21. 后续文档拆分建议

本方案确定后，可继续拆分为以下施工文档：

1. 数据库表结构设计稿。
2. 后端 API 详细契约稿。
3. OSS 与 CDN 配置手册。
4. Web 改造施工单。
5. 小程序改造施工单。
6. 测试与验收清单。
