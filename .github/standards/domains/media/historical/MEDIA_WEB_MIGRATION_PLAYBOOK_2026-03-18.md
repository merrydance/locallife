# Web 端媒体改造施工单

## 1. 文档目标

本文档用于指导 Web 前端从“业务接口直接上传图片 + 页面直接读取 image_url 原图”的旧模式，迁移到“媒体中心直传 OSS + 业务表提交 media_asset_id + 页面默认读取规格图”的最终态。

本文档为执行型施工单，目标是让前端工程师按步骤落地，不依赖额外口头说明。

## 2. 适用范围

本施工单适用于 web 目录下的前端代码：

1. web/src/lib
2. web/src/types
3. web/src/components
4. web/src/app

## 3. 当前问题梳理

### 3.1 当前上传模式

当前 Web 使用 FormData 直接调用业务图片上传接口：

1. web/src/lib/api.ts 中存在 apiUpload
2. dishes 页面直接调 /dishes/images/upload
3. tables 页面直接调 /tables/images/upload
4. merchant settings 页面直接 fetch /merchants/images/upload

这类模式的问题：

1. 图片字节流仍经过业务服务。
2. 页面逻辑直接依赖旧上传接口。
3. 上传成功后得到的是 image_url，不是 media_asset_id。
4. 无法与最终媒体中心模型对齐。

### 3.2 当前读图模式

当前页面普遍直接消费：

1. image_url
2. logo_url
3. avatar_url

并通过 getMediaUrl 做路径归一化。

问题：

1. 页面不知道当前拿到的是原图还是规格图。
2. 列表页、表格页、小卡片页经常直接读 image_url。
3. 缺少统一的图片规格选择器。

## 4. 最终目标

Web 端最终必须满足以下条件：

1. 不再调用旧的 multipart 图片上传接口。
2. 所有上传都通过媒体中心 API 创建 upload session。
3. 浏览器直传 OSS。
4. 完成上传后拿到 media_asset_id。
5. 所有业务表单提交 media_asset_id，而不是 image_url。
6. 页面展示默认使用 thumb、card、detail 中的合适规格。
7. 列表页、表格页、搜索页不再请求 original。

## 5. 核心改造策略

### 5.1 两条主线并行改造

Web 改造分两条主线：

1. 写链路改造
旧：file -> apiUpload -> image_url -> 业务表单
新：file -> upload session -> 直传 OSS -> complete -> media_asset_id -> 业务表单

2. 读链路改造
旧：页面直接消费 image_url
新：页面消费 image 变体结构或后端回填的规格 URL

### 5.2 兼容策略

为了减少前端一次性改造面，允许短期兼容：

1. 业务响应仍保留 image_url、logo_url、avatar_url 字段名
2. 但这些字段的值必须已经是规格化后的默认展示 URL

同时新增：

1. media_asset_id
2. image_variants 或结构化 image 字段

## 6. 需要新增的前端能力

### 6.1 新增媒体上传 SDK

新增文件建议：

1. web/src/lib/media.ts
2. web/src/types/media.ts

### 6.2 新增能力列表

至少新增以下函数：

1. createMediaUploadSession
2. uploadFileToOSS
3. completeMediaUpload
4. uploadMedia
5. getPreferredImageUrl

### 6.3 建议的 types/media.ts

建议定义：

1. MediaVariantUrls
2. MediaAssetResponse
3. CreateUploadSessionRequest
4. CreateUploadSessionResponse
5. CompleteMediaUploadRequest
6. CompleteMediaUploadResponse
7. UploadMediaOptions

## 7. 推荐 API 封装设计

### 7.1 createMediaUploadSession

签名建议：

```ts
export async function createMediaUploadSession(req: CreateUploadSessionRequest): Promise<CreateUploadSessionResponse>
```

职责：

1. 调用 /v1/media/upload-sessions
2. 返回 upload_id、object_key、upload_host、form

### 7.2 uploadFileToOSS

签名建议：

```ts
export async function uploadFileToOSS(file: File, session: CreateUploadSessionResponse): Promise<void>
```

职责：

1. 构造浏览器 FormData
2. 按后端下发字段提交到 upload_host
3. 不自行猜 OSS 字段名
4. 以 session.form 为准

### 7.3 completeMediaUpload

签名建议：

```ts
export async function completeMediaUpload(req: CompleteMediaUploadRequest): Promise<MediaAssetResponse>
```

职责：

1. 调用 /v1/media/complete
2. 返回 media_asset_id 与 variants

### 7.4 uploadMedia

签名建议：

```ts
export async function uploadMedia(file: File, options: UploadMediaOptions): Promise<MediaAssetResponse>
```

职责：

1. 可选执行客户端预压缩
2. 调 createMediaUploadSession
3. 调 uploadFileToOSS
4. 调 completeMediaUpload
5. 向页面返回完整媒体对象

### 7.5 getPreferredImageUrl

签名建议：

```ts
export function getPreferredImageUrl(asset: MediaAssetLike | undefined, variant: 'thumb' | 'card' | 'detail' | 'original'): string
```

职责：

1. 优先读取 variants 对应字段
2. 若后端仍处于兼容阶段，回退读取 image_url / logo_url / avatar_url
3. 最终替代 getMediaUrl 的主要职责

## 8. 图片组件层改造

### 8.1 目标

建立统一图片消费规则，避免页面自行决定读哪个 URL。

### 8.2 新增组件建议

建议新增：

1. web/src/components/shared/media-image.tsx

建议参数：

1. media
2. variant
3. alt
4. width
5. height
6. className
7. fallback

### 8.3 组件职责

1. 从 media 或 variants 中选择 URL
2. 处理空图 fallback
3. 统一 next/image 用法
4. 屏蔽页面对 image_url 的直接依赖

### 8.4 variant 使用规则

1. 表格、列表、小卡片：thumb
2. 商品卡片、商户卡片、信息卡片：card
3. 详情页主图、编辑预览主图：detail
4. 放大预览、下载、审核：original

## 9. 页面与模块改造清单

### 9.1 第一批必须改造的上传页面

#### 9.1.1 菜品管理

文件：

1. web/src/components/merchant/dishes-page-client.tsx

当前问题：

1. 调用 apiUpload("/dishes/images/upload", file)
2. 依赖返回 image_url
3. 表单最终提交 image_url

目标改造：

1. 改为 uploadMedia(file, { business_type: 'merchant_dish', media_category: 'dish', visibility: 'public' })
2. 表单状态从 image_url 改为 image_media_asset_id
3. 预览图优先读取 upload 返回的 variants.detail_url

#### 9.1.2 桌台管理

文件：

1. web/src/components/merchant/tables-page-client.tsx
2. web/src/app/merchant/tables/[id]/page.tsx

当前问题：

1. 先上传得到 image_url
2. 再 POST /tables/{id}/images 提交 image_url

目标改造：

1. 上传完成后拿到 media_asset_id
2. 改为 POST /tables/{id}/images 提交 media_asset_id
3. pendingImages 从 string[] 改为 MediaAssetResponse[] 或至少 media_asset_id[] + preview_url

#### 9.1.3 商户设置

文件：

1. web/src/components/merchant/settings-page-client.tsx

当前问题：

1. 手写 fetch 调 /merchants/images/upload
2. 返回 logo_url
3. 更新 profile.logo_url

目标改造：

1. 使用统一 uploadMedia
2. profile 状态新增 logo_media_asset_id
3. 页面展示改为从 media variants.detail_url 预览

### 9.2 第二批展示页面改造

需要统一替换的展示页类型：

1. 商户列表
2. 菜品列表
3. 订单列表中的商品缩略图
4. 会员头像
5. 员工头像
6. 包间和桌台图片
7. 搜索结果图
8. 统计页面头像或商品图

目标：

1. 禁止页面直接写 <Image src={item.image_url}>
2. 改为走 MediaImage 或 getPreferredImageUrl

## 10. 状态管理改造

### 10.1 原则

页面本地状态不再以 image_url 为唯一真相源。

### 10.2 推荐状态结构

旧：

```ts
type EditDish = {
  image_url?: string
}
```

新：

```ts
type EditDish = {
  image_media_asset_id?: number
  image?: MediaAssetResponse
}
```

说明：

1. image_media_asset_id 用于最终提交。
2. image 用于页面预览、删除、展示 variants。

### 10.3 多图场景状态结构

旧：

```ts
string[]
```

新：

```ts
Array<{
  media_asset_id: number
  preview_url: string
  sort_order: number
  role?: string
}>
```

## 11. 类型定义改造

### 11.1 原则

Web 类型层要逐步从 image_url 导向 media_asset_id + 变体结构。

### 11.2 推荐改法

以兼容方式演进：

```ts
type Dish = {
  image_url?: string
  image_media_asset_id?: number
  image_variants?: MediaVariantUrls
}
```

说明：

1. 首期允许三者并存，方便后后端逐步切换。
2. 页面优先读 image_variants，其次 image_url。

### 11.3 必须新增的类型能力

所有会展示图片的类型至少允许容纳以下字段之一：

1. xxx_media_asset_id
2. xxx_variants
3. 兼容字段 xxx_url

## 12. 与后端接口联调约定

### 12.1 上传完成后业务提交规则

上传完成后，业务提交必须携带：

1. 单图场景：xxx_media_asset_id
2. 多图场景：media_asset_id 数组或关联对象数组

禁止继续提交：

1. image_url
2. logo_url
3. avatar_url

### 12.2 旧字段兼容期

兼容期内：

1. 后端仍可返回 image_url 供展示
2. 但写接口不得再要求 image_url

## 13. 浏览器直传实现细节

### 13.1 上传步骤

标准执行顺序：

1. 用户选图
2. 校验格式与大小
3. 可选预压缩
4. createMediaUploadSession
5. uploadFileToOSS
6. completeMediaUpload
7. 更新表单 state

### 13.2 失败处理

失败时的错误展示规则：

1. createMediaUploadSession 失败：提示“无法创建上传会话”
2. uploadFileToOSS 失败：提示“文件上传失败”
3. completeMediaUpload 失败：提示“上传确认失败”

不得直接透出 OSS 原始报错给终端用户。

### 13.3 上传态 UI

所有上传按钮统一支持：

1. uploading 状态
2. progress 状态，若可实现则加
3. retry 能力
4. 删除已上传媒体能力

## 14. 图片展示性能要求

### 14.1 页面级规范

1. 列表页只能使用 thumb 或 card
2. 详情页默认使用 detail
3. 预览弹窗才允许 original

### 14.2 Next Image 使用规范

1. 尽量显式给 width 和 height
2. 长列表优先小尺寸 variant
3. 禁止把 original_url 用于 40x40、48x48、64x64 这类缩略图位

## 15. 分阶段施工建议

### 阶段 1：基础能力

1. 新增 types/media.ts
2. 新增 lib/media.ts
3. 新增 shared/media-image.tsx
4. 新增浏览器预压缩工具，可选

### 阶段 2：上传入口替换

1. merchant/dishes-page-client.tsx
2. merchant/tables-page-client.tsx
3. merchant/settings-page-client.tsx

### 阶段 3：读图入口替换

1. 列表页
2. 卡片页
3. 订单商品图
4. 头像场景
5. 详情页

### 阶段 4：清理旧能力

1. 删除 apiUpload 在图片上传中的使用
2. 删除页面中对旧上传接口的直接依赖
3. 弱化 getMediaUrl 的历史路径兼容职责

## 16. 验收标准

Web 侧视为完成的标准：

1. 不再调用 /dishes/images/upload、/tables/images/upload、/merchants/images/upload 等旧图片上传接口
2. 所有上传都经过 upload-sessions -> 直传 OSS -> complete
3. 所有写接口都提交 media_asset_id
4. 列表页不再直接消费 original 图
5. 关键页面改用统一的 MediaImage 或规格 URL 选择工具
6. 页面预览、删除、替换图片逻辑可正常工作

## 17. 风险与处理

### 17.1 风险：页面改造点多

处理：

1. 先集中改上传入口
2. 再统一读图组件
3. 最后批量替换页面使用点

### 17.2 风险：兼容字段并存导致混乱

处理：

1. 明确读取优先级：variants > 兼容 url
2. 明确写入只提交 media_asset_id

### 17.3 风险：部分页面直接使用 image_url 的原值

处理：

1. 引入 lint 规则或全局搜索清单
2. 禁止新增裸用 image_url 的 <Image src={...}> 写法

## 18. 参考文件清单

当前重点改造文件包括但不限于：

1. web/src/lib/api.ts
2. web/src/components/merchant/dishes-page-client.tsx
3. web/src/components/merchant/tables-page-client.tsx
4. web/src/components/merchant/settings-page-client.tsx
5. web/src/app/merchant/tables/[id]/page.tsx
6. web/src/types/dish.ts
7. web/src/types/table.ts
8. web/src/types/merchant-settings.ts
9. web/src/types/combo.ts
10. web/src/types/staff.ts

## 19. 下一步建议

在本施工单之后，建议继续产出：

1. 小程序直传施工单
2. 后端媒体模块代码结构设计稿
3. 测试与验收清单
