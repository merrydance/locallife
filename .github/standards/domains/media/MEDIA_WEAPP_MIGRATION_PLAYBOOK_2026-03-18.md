# 小程序媒体改造施工单

## 1. 文档目标

本文档用于指导微信小程序从“直接调用业务图片上传接口 + 页面直接消费 image_url”的旧模式，迁移到“媒体中心直传 OSS + 业务提交 media_asset_id + 页面默认读取规格图”的最终态。

本施工单与以下文档配套使用：

1. MEDIA_OSS_CDN_FINAL_IMPLEMENTATION_PLAN_2026-03-18.md
2. MEDIA_DATABASE_SCHEMA_DESIGN_2026-03-18.md
3. MEDIA_API_CONTRACT_DESIGN_2026-03-18.md

## 2. 当前小程序问题梳理

### 2.1 上传链路现状

当前小程序存在两类旧上传链路：

1. 通用 uploadFile 封装
   位置：weapp/miniprogram/utils/request.ts

2. 各业务模块内部再次手写 wx.uploadFile
   典型位置：
   - weapp/miniprogram/api/dish.ts
   - weapp/miniprogram/api/upload.ts
   - weapp/miniprogram/api/table-device-management.ts

这些旧链路的共同问题：

1. 图片字节流仍上传到业务服务。
2. 上传结果返回 image_url，而不是 media_asset_id。
3. 业务接口直接依赖旧图片上传入口。
4. 零散逻辑多，难以统一容错、重试、埋点和预压缩。

### 2.2 读图链路现状

当前读图主要依赖：

1. image_url
2. logo_url
3. avatar_url
4. getPublicImageUrl

问题：

1. 页面默认不知道拿到的是原图还是缩略图。
2. 小程序列表页也可能直接消费原图。
3. getPublicImageUrl 当前职责只是“路径补全”，没有规格选择能力。

## 3. 最终目标

小程序侧最终必须满足：

1. 不再调用 /v1/dishes/images/upload、/v1/tables/images/upload、/v1/reviews/images/upload 等旧图片上传接口。
2. 所有上传都经由媒体中心：upload-sessions -> 直传 OSS -> complete。
3. 小程序业务接口统一提交 media_asset_id。
4. 页面默认读取 thumb、card、detail 等规格图。
5. 原图只用于预览、审核、放大查看。

## 4. 总体改造策略

### 4.1 两条主线

改造分为两条主线：

1. 写链路
旧：wx.uploadFile -> 业务接口 -> image_url
新：createMediaUploadSession -> wx 直传 OSS -> completeMediaUpload -> media_asset_id

2. 读链路
旧：getPublicImageUrl(image_url)
新：getMediaDisplayUrl(media, variant)

### 4.2 兼容期策略

兼容期允许：

1. 后端继续返回 image_url、logo_url、avatar_url 字段名
2. 但这些字段的值应已是默认规格图 URL

同时建议逐步增加：

1. media_asset_id
2. image_variants
3. logo_variants
4. avatar_variants

## 5. 需要新增的小程序能力

### 5.1 新增文件建议

建议新增：

1. weapp/miniprogram/api/media.ts
2. weapp/miniprogram/types/media.ts
3. weapp/miniprogram/utils/media.ts

### 5.2 新增核心函数

至少新增以下函数：

1. createMediaUploadSession
2. uploadMediaFileToOSS
3. completeMediaUpload
4. uploadMedia
5. getMediaDisplayUrl
6. compressImageIfNeeded

## 6. API 层改造设计

### 6.1 createMediaUploadSession

建议签名：

```ts
export async function createMediaUploadSession(req: CreateUploadSessionRequest): Promise<CreateUploadSessionResponse>
```

职责：

1. 调用 /v1/media/upload-sessions
2. 返回 upload_id、object_key、upload_host、form

### 6.2 uploadMediaFileToOSS

建议签名：

```ts
export async function uploadMediaFileToOSS(filePath: string, session: CreateUploadSessionResponse): Promise<void>
```

职责：

1. 使用 wx.uploadFile 直接上传到 upload_host
2. 由 session.form 提供表单字段
3. 不允许前端写死 OSS 表单字段结构

### 6.3 completeMediaUpload

建议签名：

```ts
export async function completeMediaUpload(req: CompleteMediaUploadRequest): Promise<MediaAssetResponse>
```

职责：

1. 调用 /v1/media/complete
2. 获取 media_asset_id 和 variants

### 6.4 uploadMedia

建议签名：

```ts
export async function uploadMedia(filePath: string, options: UploadMediaOptions): Promise<MediaAssetResponse>
```

职责：

1. 读取图片信息
2. 可选压缩
3. 创建 upload session
4. 直传 OSS
5. complete
6. 返回完整媒体对象

## 7. 图片 URL 工具层改造

### 7.1 当前 getPublicImageUrl 的问题

当前位置：weapp/miniprogram/utils/image.ts

当前职责仅是：

1. 补全相对路径
2. 拼接 API_BASE

问题：

1. 不知道当前要读哪种规格
2. 无法区分原图与缩略图
3. 与媒体中心模型不匹配

### 7.2 新增 getMediaDisplayUrl

建议新增函数：

```ts
export function getMediaDisplayUrl(input: MediaLike | string | undefined, variant: 'thumb' | 'card' | 'detail' | 'original'): string
```

读取优先级建议：

1. input.variants?.[variant]
2. input.image_variants?.[variant]
3. input.logo_variants?.[variant]
4. input.avatar_variants?.[variant]
5. 兼容字段 image_url / logo_url / avatar_url
6. 最后回退到 getPublicImageUrl

### 7.3 保留 getPublicImageUrl 的定位

保留 getPublicImageUrl 作为兼容层，但不再推荐新代码直接使用。

新代码必须优先使用 getMediaDisplayUrl。

## 8. 小程序上传实现细节

### 8.1 标准上传顺序

统一执行顺序：

1. 用户选择图片
2. 获取图片本地路径
3. 可选压缩图片
4. createMediaUploadSession
5. uploadMediaFileToOSS
6. completeMediaUpload
7. 拿到 media_asset_id
8. 后续业务接口提交 media_asset_id

### 8.2 压缩策略

优先使用小程序端轻量压缩，减少上传时延与失败率。

建议规则：

1. 最长边大于 4096 时压缩
2. 文件体积超过目标上限阈值时压缩
3. 非透明 PNG 可在必要时转 JPG
4. 保留原文件兜底路径，压缩失败不阻塞上传

可用能力：

1. wx.compressImage
2. 必要时结合 wx.getImageInfo

### 8.3 失败处理

错误分类：

1. 创建上传会话失败
2. 直传 OSS 失败
3. complete 失败
4. 压缩失败但原图回退上传成功

用户提示要求：

1. 不直接展示 OSS 原始错误
2. 展示统一用户提示，如“图片上传失败，请重试”
3. 记录详细日志用于排障

## 9. 业务 API 改造清单

### 9.1 菜品管理

文件：

1. weapp/miniprogram/api/dish.ts

当前问题：

1. 内部直接 wx.uploadFile 到 /v1/dishes/images/upload
2. 返回 image_url

目标改造：

1. 删除 uploadDishImage 旧实现
2. 改为调用统一 uploadMedia
3. 菜品创建与更新接口提交 image_media_asset_id

### 9.2 桌台图片管理

文件：

1. weapp/miniprogram/api/table-device-management.ts

当前问题：

1. uploadTableImageFile 返回 image_url
2. uploadTableImage 再把 image_url 提交到 /tables/{id}/images

目标改造：

1. uploadTableImageFile 改为返回 MediaAssetResponse
2. uploadTableImage 提交 media_asset_id 和 sort_order
3. TableImageResponse 增加 media_asset_id

### 9.3 评价上传

文件：

1. weapp/miniprogram/api/review.ts

当前问题：

1. 直接调用 /v1/reviews/images/upload
2. 返回 image_url

目标改造：

1. 改为统一 uploadMedia
2. 评价提交时传 media_asset_id 数组

### 9.4 商户入驻与 OCR 材料

文件：

1. weapp/miniprogram/api/onboarding.ts
2. 历史上还存在 weapp/miniprogram/api/merchant-application.ts，现已删除并收敛到 onboarding.ts
3. weapp/miniprogram/api/ocr.ts

目标改造：

1. 证照上传改为统一 uploadMedia
2. OCR 接口若仍要求 image_url，后端可短期兼容从 media_asset_id 反查对象
3. 最终业务表提交 business_license_media_asset_id、food_permit_media_asset_id 等字段

### 9.5 通用上传服务

文件：

1. weapp/miniprogram/api/upload.ts
2. weapp/miniprogram/utils/request.ts

目标改造：

1. 逐步废弃通用 uploadFile 用于图片上传的职责
2. 保留 uploadFile 仅作为历史兼容或非媒体文件兜底能力
3. 新增 uploadMedia 作为唯一图片上传入口

## 10. 页面与适配层改造

### 10.1 重点改造方向

小程序目前大量页面通过适配层将 image_url 传给组件。

重点位置包括：

1. adapters/dish.ts
2. adapters/order.ts
3. adapters/order-card.ts
4. pages/dine-in/menu/menu.ts
5. pages/dine-in/checkout/checkout.ts
6. pages/reservation/index.ts
7. pages/takeout/cart/index.ts
8. pages/takeout/order-confirm/index.ts
9. pages/takeout/combo-detail/index.ts

### 10.2 改造原则

1. 适配层优先输出规格化 URL，而不是原始 image_url
2. 小图位默认使用 thumb 或 card
3. 详情页使用 detail

### 10.3 推荐适配方式

旧：

```ts
imageUrl: getPublicImageUrl(dto.image_url)
```

新：

```ts
imageUrl: getMediaDisplayUrl(dto, 'card')
```

说明：

1. 在 DTO 未完全结构化前，getMediaDisplayUrl 内部负责兼容 image_url。
2. 页面层不再关心 image_url 是否原图。

## 11. 类型定义改造

### 11.1 原则

小程序类型层逐步从单一 image_url 过渡为：

1. media_asset_id
2. image_variants
3. 兼容字段 image_url

### 11.2 推荐字段结构

```ts
type DishDTO = {
  image_url?: string
  image_media_asset_id?: number
  image_variants?: MediaVariantUrls
}
```

### 11.3 头像与 Logo

对 avatar_url、logo_url 采用同样策略：

1. avatar_media_asset_id + avatar_variants + avatar_url
2. logo_media_asset_id + logo_variants + logo_url

## 12. 状态管理改造

### 12.1 单图场景

旧：

```ts
business_license_image_url: string
```

新：

```ts
business_license_media_asset_id?: number
business_license_media?: MediaAssetResponse
```

补充说明：运营商、商户、骑手三条私有材料主申请链路现已在前端运行时代码中切到 asset_id 模式；本文中出现的旧 URL 字段示例仅用于解释迁移背景，不应继续作为当前契约实现。

### 12.2 多图场景

旧：

```ts
images: string[]
```

新：

```ts
images: Array<{
  media_asset_id: number
  preview_url: string
  sort_order: number
}>
```

## 13. 提交接口改造要求

### 13.1 必须提交 media_asset_id

业务提交中必须使用：

1. 单图字段：xxx_media_asset_id
2. 多图字段：media_asset_id 数组或关联对象数组

禁止继续提交：

1. image_url
2. logo_url
3. avatar_url
4. business_license_image_url
5. food_permit_url
6. legal_person_id_front_url
7. legal_person_id_back_url
8. id_card_front_url
9. id_card_back_url
10. health_cert_url

### 13.2 OCR 兼容策略

若 OCR 接口短期仍要求 image_url：

1. 由后端在 OCR 接口内部接受 media_asset_id 并反查 object_key
2. 小程序侧不再直接依赖 image_url 作为持久化字段

## 14. 具体施工顺序

### 阶段 1：基础能力

1. 新增 types/media.ts
2. 新增 api/media.ts
3. 新增 utils/media.ts
4. 新增图片压缩工具

### 阶段 2：上传入口替换

1. dish.ts
2. table-device-management.ts
3. review.ts
4. onboarding.ts
5. upload.ts

### 阶段 3：读图工具替换

1. 用 getMediaDisplayUrl 替换关键适配层 getPublicImageUrl 调用
2. 优先替换列表、小卡片、订单图等高频位置

### 阶段 4：业务提交模型替换

1. 表单 state 由 URL 改为 media_asset_id
2. 多图集合从 string[] 改为对象数组

## 15. 验收标准

小程序侧视为完成的标准：

1. 不再通过旧业务接口上传图片文件
2. 所有图片上传统一走 upload-sessions -> 直传 OSS -> complete
3. 业务接口统一提交 media_asset_id
4. 菜品图、桌台图、评价图、证照图全部切换到媒体中心
5. 高流量页面默认读取规格图，不再读取原图
6. getPublicImageUrl 不再承担新链路主要职责

## 16. 风险与处理

### 16.1 风险：旧代码里 uploadFile 使用范围广

处理：

1. 不立即删除 uploadFile
2. 先禁止其继续用于图片上传
3. 新图片上传一律走 uploadMedia

### 16.2 风险：适配层大量直接消费 image_url

处理：

1. 优先改 adapters 层
2. 页面层尽量少直接处理 URL

### 16.3 风险：小程序端压缩兼容性

处理：

1. 压缩失败时回退原图上传
2. 不让压缩失败阻塞主链路

## 17. 当前重点文件清单

首批重点文件包括：

1. weapp/miniprogram/utils/request.ts
2. weapp/miniprogram/utils/image.ts
3. weapp/miniprogram/api/upload.ts
4. weapp/miniprogram/api/dish.ts
5. weapp/miniprogram/api/review.ts
6. weapp/miniprogram/api/table-device-management.ts
7. weapp/miniprogram/api/onboarding.ts
8. 历史旧封装 weapp/miniprogram/api/merchant-application.ts（现已删除）
9. weapp/miniprogram/adapters/dish.ts
10. weapp/miniprogram/adapters/order.ts
11. weapp/miniprogram/adapters/order-card.ts

## 18. 下一步建议

在本施工单之后，建议继续补：

1. 测试与验收清单
2. 后端媒体模块代码结构设计稿
