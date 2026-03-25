# 旧 `/uploads` 兼容链路迁移施工单

## 1. 目标

本施工单用于将系统中“本地 uploads 相对路径 + 前后端自行补全访问地址”的旧模式分阶段移除，最终收敛到以下约束：

1. 公共展示图片只返回 CDN 或其它可直接访问的绝对 URL，不再返回 `uploads/...` 或 `/uploads/...`。
2. 小程序与 Web 的公共读图逻辑不再把旧相对路径拼接成业务域名地址。
3. 私有材料不再依赖 `/v1/uploads/sign` 的旧路径签名模型，而是统一走媒体中心私有访问模型。
4. 二维码等历史上写入本地 `uploads` 的公共资源迁移到 OSS/CDN 后，再删除后端本地 `/uploads` 路由与兼容代码。

本次迁移不是“简单删除所有 `/uploads` 字符串”。如果不区分公共展示、私有材料、二维码三个子链路，直接删除兼容代码会引入可见回归。

## 2. 当前事实与约束

### 2.1 初始盘点中确认过的旧兼容逻辑位置

1. 后端公共展示兼容：api/upload_url.go 中的 normalizeUploadURLForClient。
2. 后端公共图片输出调用位：api/search.go、api/merchant.go。
3. 后端二维码旧链路：api/table.go、api/scan.go。
4. 后端旧签名链路：api/upload_signed.go，接口曾为 POST /v1/uploads/sign；现已在 B2 中下线。
5. 小程序公共读图兼容：weapp/miniprogram/utils/media.ts 中的 getMediaDisplayUrl。
6. 小程序私有材料兼容：weapp/miniprogram/utils/image-security.ts 中曾包含 resolveImageURL 与 isPublicUploads；现已删除，仅保留 private-access helper。
7. Web 公共读图兼容：web/src/lib/media.ts 与 web/src/lib/api.ts 中的 getMediaDisplayUrl、getMediaUrl。
8. Web 私有材料兼容：web/src/lib/api.ts 中曾包含 resolveProtectedMediaUrl、resolveProtectedMediaCandidates、normalizeUploadPathForSign；现已删除。

### 2.2 为什么必须分阶段

1. 公共展示图片已经具备迁移条件。它们本质上应由 CDN 直接服务，不应再依赖应用服务上的 `/uploads/...` 路径。
2. 私有材料不能按字符串做一刀切删除。虽然 `/v1/uploads/sign` 已经下线，但仍有部分页面和接口契约保留旧 URL 字段回读，如果不继续收敛到 `media_asset_id -> private-access`，仍会造成私有材料预览链路不一致。
3. 二维码链路暂时不能直接拔除。当前桌台二维码仍在 api/scan.go 中落到本地 uploads 语义，并由 api/table.go 返回给前端。只有等二维码改成 OSS/CDN 资源后，才能删除这部分兼容。

### 2.3 当前执行范围说明

本施工单已完成阶段 A 与阶段 B，当前未完成重点为：进入二维码与本地 `/uploads` 路由的最终清理。

## 3. 迁移总原则

1. 先切公共展示，再迁私有材料，最后迁二维码与本地路由。
2. 每一步都要先明确调用位，再修改输出，再补测试，再更新勾选状态。
3. 已确认前端不再调用 POST /v1/uploads/sign 后，才允许删除该接口；当前该步骤已完成。
4. 在二维码仍可能返回旧路径之前，不删除 table/scan 的旧字段处理。
5. 任何阶段都不能让客户端重新依赖“API 域名 + `/uploads/...`”作为公共图片主链路。

## 4. 分阶段执行清单

### 阶段 A：清理公共展示链路

- [x] A0. 发布本施工单
  - 原因：后续每一步都需要以本文作为范围边界和验收依据，避免出现“以为能删，实际还有依赖”的误判。
  - 完成标准：本文进入仓库 docs 目录，明确标注执行顺序、暂缓项、验收标准、回滚思路。

- [x] A1. 后端公共展示字段停止输出旧 `/uploads` 路径
  - 原因：在 `FILE_STORAGE_PROVIDER=oss` 场景下，公共展示图片应直接返回 CDN URL；继续返回 `/uploads/...` 会把客户端重新绑回应用服务路由，并在生产上制造无意义的 404 风险。
  - 涉及文件：api/upload_url.go、api/search.go、api/merchant.go、api/media_url_test.go。
  - 方案：
    1. 保留 normalizeUploadURLForClient 作为旧链路基础函数，避免影响二维码和私有签名。
    2. 新增仅面向公共展示的后端辅助函数，在 `oss` 模式下把已知公共 uploads 路径重写为 CDN URL，在 `local` 模式下保持原行为。
    3. 仅替换 search 与 merchant 商户门头图/环境图这些公共展示调用位，不改 table/scan。
    4. 不改写私有 uploads 路径，避免误把证照类资源暴露成公共地址。
  - 验收标准：
    1. 搜索结果中的 `cover_image` 不再输出 `/uploads/...`。
    2. 商户门头图、环境图响应字段不再输出 `/uploads/...`。
    3. 二维码字段 `qr_code_url` 仍保持当前行为不变。
    4. 相关 Go 单元测试通过。
  - 回滚方式：仅回退本项改动；恢复 search/merchant 对 normalizeUploadURLForClient 的直接调用。
  - 当前执行结果：已完成。后端新增仅面向公共展示路径的解析函数，并将 search 与 merchant 的公共图片输出切换到该函数；table/scan 未改动。

- [x] A2. 小程序公共读图工具不再为展示场景拼接旧相对路径
  - 原因：一旦后端公共展示字段稳定输出绝对 URL，小程序继续保留“`API_BASE + uploads/...`”只会掩盖异常数据来源，延长兼容期。
  - 涉及文件：weapp/miniprogram/utils/media.ts，以及所有仅消费公共展示图的页面调用点。
  - 方案：
    1. 将 getMediaDisplayUrl 收敛为“仅接受绝对 URL，否则返回空串或占位图”。
    2. 保留 image-security.ts 里的私有材料签名逻辑，不在本阶段删除。
    3. 修正受影响页面，确保它们依赖后端提供的绝对 URL，而不是本地补全。
  - 验收标准：
    1. 小程序公共商户图、菜品图页面不再把 `uploads/...` 拼成请求地址。
    2. 私有材料预览链路不回归。
  - 当前执行结果：已完成。getMediaDisplayUrl 现在只接受绝对 URL 或本地临时文件路径；私有材料仍由 image-security.ts 负责签名解析。

- [x] A3. Web 公共读图工具不再为展示场景补全旧相对路径
  - 原因：Web 当前既有 `getMediaDisplayUrl`，又有 `getMediaUrl`，会继续吞掉错误数据，导致旧链路长期存活。
  - 涉及文件：web/src/lib/media.ts、web/src/lib/api.ts，以及仅展示公共图片的组件调用位。
  - 方案：
    1. 将公共展示入口限制为绝对 URL 或占位图。
    2. 暂时保留 resolveProtectedMediaUrl 及其相关旧签名逻辑，直到私有材料完成迁移。
    3. 将桌台二维码相关页面排除在本阶段外，因为它仍依赖旧二维码路径。
  - 验收标准：
    1. Web 商户设置、菜品列表、组合餐、库存、评论等公共图片页面不再补全 `/uploads/...`。
    2. 依赖旧签名的私有材料页面仍可访问。
  - 当前执行结果：已完成。web/src/lib/media.ts 的公共展示 helper 已收紧为仅接受绝对 URL 或浏览器本地预览 URL；纯公共展示页面已切换到该 helper。tables 页面因仍混合二维码与本地预览链路，暂不纳入本阶段删除范围。

### 阶段 B：迁移私有材料链路

- [x] B1. 盘点并切换所有私有材料页面到媒体中心私有访问模型
  - 原因：只有当前端不再提交或消费旧 uploads 路径，才能删除 `/v1/uploads/sign`。
  - 涉及文件：weapp/miniprogram/utils/image-security.ts、web/src/lib/api.ts，以及所有注册、审核、资质查看页面。
  - 方案：
    1. 确认相关接口是否已返回 `media_asset_id` 或可直接申请私有访问地址。
    2. 将前端从“路径签名”切换到“media_asset_id -> private-access”模式。
    3. 删除 `isPublicUploads`、`normalizeUploadPathForSign` 这类仅服务旧路径模型的兼容函数。
  - 验收标准：
    1. 前端不再向 `/v1/uploads/sign` 发请求。
    2. 旧 `uploads/...` 私有材料路径不再出现在接口契约中。
  - 当前执行结果：已完成。Web 管理端运营商申请详情页、小程序运营商注册页、小程序商户入驻页、小程序骑手注册页已切换到 `media_asset_id -> /v1/media/private-access`；Web 端旧 `resolveProtectedMediaUrl` / `resolveProtectedMediaCandidates` 已删除，小程序 `weapp/miniprogram/utils/image-security.ts` 中旧 `/v1/uploads/sign` 调用与兼容函数也已删除。当前实际在跑的小程序申请链路类型定义已去除运营商、商户、骑手私有材料的旧 URL 字段回读，私有材料重试逻辑改为基于 asset 标识定位。剩余 `business_license_image_url`、`food_permit_url` 等字段仅存在于公开商户详情等对外展示语义中，不属于 B1 私有材料链路范围；数据库历史迁移与历史设计文档中的旧字段说明也已明确标注为历史信息。

- [x] B2. 后端下线 `/v1/uploads/sign` 及旧路径签名权限判定
  - 原因：保留该接口意味着旧路径模型仍可被新代码继续依赖。
  - 涉及文件：api/upload_signed.go、api/server.go、相关测试。
  - 方案：
    1. 在前端全部迁移完成并验收后，删除路由注册与实现。
    2. 删除基于 uploads 路径的公开/私有判定逻辑。
  - 验收标准：
    1. 代码库中不再存在新的 `/v1/uploads/sign` 调用。
    2. 后端不再暴露旧路径签名 API。
  - 当前执行结果：已完成。后端已删除 `POST /v1/uploads/sign` 路由注册与对应签名权限实现；local 文件存储模式下 `/uploads/*filepath` 已改为仅供开发调试读取的兼容路由，不再承担私有文件签名校验职责。Swagger 与主注释中的旧接口暴露也已同步移除。

### 阶段 C：迁移二维码与本地路由

- [x] C1. 将桌台二维码生成与存储迁移到 OSS/CDN 语义
  - 原因：二维码是当前仍明显依赖本地 uploads 语义的公共资源。如果不先迁它，就不能删除 table/scan 的旧路径输出与本地 `/uploads` 路由残留。
  - 涉及文件：api/scan.go、api/table.go、对应上传或媒体模块、相关前端桌台页面。
  - 方案：
    1. 生成二维码后直接写入 OSS 公共对象键。
    2. 在表结构或媒体表中保存可解析为 CDN 的对象标识，而不是旧相对路径。
    3. 前端桌台页面改为直接消费 CDN 绝对 URL。
  - 验收标准：
    1. `qr_code_url` 不再是 `uploads/...` 或 `/uploads/...`。
    2. 桌台二维码预览、下载、打印均正常。
  - 当前执行进展：已完成。`api/scan.go` 的二维码生成链路已从本地 `uploads` 文件写入切换为服务端直写对象存储，并同步写入 `media_assets` 记录；新生成的 `qr_code_url` 已改为媒体中心解析出的公共 URL，不再返回本地 uploads 相对路径。与此同时，`api/table.go` 与 `api/scan.go` 对历史二维码值的读取已统一改成公共 URL 解析，在 OSS 模式下会把遗留公共 uploads 路径重写为 CDN URL；商户端命中历史已打标二维码时，`/v1/tables/{id}/qrcode` 还会把解析后的公共 URL 自愈回写到 `tables.qr_code_url`。Web 商户桌台页与小程序商户桌台页中的二维码预览/下载也都已改为直接消费后端返回的公共 URL，不再额外调用旧路径兼容 helper；桌台创建/更新接口中客户端手工写入 `qr_code_url` 的入口也已删除，避免旧路径语义重新写回数据库。

- [x] C2. 删除后端本地 `/uploads` 公共访问兼容与剩余路径归一化逻辑
  - 原因：这是最终“连根拔除”的动作，必须在公共展示、私有材料、二维码三条链路全部完成后进行。
  - 涉及文件：api/upload_url.go、api/server.go、残余调用位与测试。
  - 方案：
    1. 删除仅服务旧展示路径的归一化输出逻辑。
    2. 删除 local 模式专用的公共 `/uploads` 读取兼容，若开发环境仍需要本地调试，则迁移到新的 dev 专用调试路径，不再复用生产契约。
  - 验收标准：
    1. 代码库运行时不再以 `/uploads/...` 作为公共展示返回值。
    2. 旧兼容函数仅保留确有必要的开发专用实现，或被完全删除。
  - 当前执行进展：已完成。local 文件存储模式下，后端已不再注册 `/uploads/*filepath` 公共读取路由，改为仅注册 dev-only 的 `/dev/uploads/*filepath` 调试路径；`resolvePublicUploadURLForClient(...)` 在 local 模式下对公共展示素材也已改为返回 `/dev/uploads/...`，避免运行时继续向客户端暴露 `/uploads/...`。此外，商户申请草稿响应中的 `storefront_images` / `environment_images`、公开商户详情中的 `cover_image`，以及搜索商户列表中的 `cover_image` 也已补上统一的公共 URL 解析，不再直接透传库中的原始 uploads 路径。当前保留的 `resolvePublicUploadURLForClient(...)` 仅承担历史数据库字符串的桥接与 dev-only 路径映射，不再作为公共 `/uploads` 主契约存在。

## 5. 本轮执行顺序

本轮只执行以下顺序，不跨阶段跳跃：

1. 完成 A1。
2. 验证 A1 并回写勾选。
3. 再进入 A2 与 A3。

## 6. 本轮完成判定

本轮任务完成时，至少应满足：

1. 施工单已入库。
2. A1 已勾选完成。
3. 文档中的“暂缓项”仍保持未勾选状态，明确说明尚未删除私有签名与二维码旧链路。

## 7. 收尾说明

本轮主迁移阶段已完成，后续仅保留低风险尾扫：

1. 前端公共图片 helper 已显式识别 dev-only 的 `/dev/uploads/...` 开发态路径，不再继续强化 `/uploads/...` 作为主契约。
2. 小程序私有文档上传组件已将 `/dev/uploads/...` 与历史 `uploads/...` 一并视为不可直接渲染的兜底路径，避免把开发态公共路径误当成私有材料预览地址。
3. 后续若继续清理，优先处理历史设计文档、测试样例和注释中的旧 `/uploads` 叙述，而不是再变动运行时主链路。
4. Web 与小程序当前保留的相对路径处理仅用于历史数据和开发态调试，不再代表生产公共图片契约。
5. 本轮已移除前端对历史 `uploads/...` 相对路径的自动补全显示兼容；若后端再次返回此类值，将直接视为异常数据而不是继续兜底渲染。