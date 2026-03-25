# 媒体系统 API 契约设计稿

## 1. 文档目标

本文档用于定义媒体系统最终态的 HTTP API 契约，作为后端实现、前端接入、Swagger 注释与联调测试的统一依据。

本文档与以下文档配套使用：

1. MEDIA_OSS_CDN_FINAL_IMPLEMENTATION_PLAN_2026-03-18.md
2. MEDIA_DATABASE_SCHEMA_DESIGN_2026-03-18.md
3. API_CONTRACT_STANDARDS.md

## 2. 契约设计原则

### 2.1 路径原则

所有媒体接口统一挂载于：

```text
/v1/media
```

采用资源化路径，避免新增动作型散落上传接口。

### 2.2 返回结构原则

本项目启用了统一响应包裹，所有 JSON API 应遵循：

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

错误响应：

```json
{
  "code": 40000,
  "message": "bad request",
  "data": {
    "code": 40001,
    "error": "specific error"
  }
}
```

说明：

1. HTTP 状态码仍表达主语义。
2. ErrorResponse 中的 code 为业务错误码。
3. 5xx 错误对前端只返回安全信息，不透出内部实现细节。

### 2.3 幂等原则

以下接口必须具备幂等性：

1. POST /v1/media/upload-sessions
2. POST /v1/media/complete
3. DELETE /v1/media/{id}

### 2.4 认证原则

除未来可能新增的内部回调接口外，媒体接口全部要求 BearerAuth。

## 3. 核心资源模型

媒体系统对外暴露四类核心资源：

1. 上传会话 upload_session
2. 媒体资产 media_asset
3. 私有访问授权 private_access_grant
4. 媒体删除任务 delete_request

## 4. 路由清单

### 4.1 核心接口

| Method | Path | 说明 |
|---|---|---|
| POST | /v1/media/upload-sessions | 创建上传会话 |
| POST | /v1/media/complete | 确认上传完成 |
| POST | /v1/media/private-access | 获取私有短期访问地址 |
| DELETE | /v1/media/{id} | 删除媒体 |
| GET | /v1/media/{id} | 获取单个媒体元数据 |

说明：

1. GET /v1/media/{id} 推荐实现，方便后台、审核和联调使用。
2. 如果首期不实现 GET /v1/media/{id}，不影响主链路上线，但建议保留在设计稿中。

## 5. 枚举定义

### 5.1 visibility

| 值 | 说明 |
|---|---|
| public | 公共媒体，可通过 CDN 访问 |
| private | 私有媒体，需签名访问 |

### 5.2 bucket_type

| 值 | 说明 |
|---|---|
| public | 存储在公共桶 |
| private | 存储在私有桶 |

### 5.3 media_kind

| 值 | 说明 |
|---|---|
| image | 图片 |
| video | 视频，预留 |
| document | 文档，预留 |

### 5.4 media_category

首期推荐值：

1. avatar
2. merchant_logo
3. merchant_storefront
4. merchant_environment
5. dish
6. combo
7. table_image
8. table_qrcode
9. review_image
10. merchant_business_license
11. merchant_food_permit
12. merchant_id_front
13. merchant_id_back
14. rider_id_front
15. rider_id_back
16. rider_health_cert
17. operator_business_license
18. operator_id_front
19. operator_id_back
19. group_license
20. brand_logo

### 5.5 upload_status

1. pending
2. uploaded
3. confirmed
4. failed
5. deleted

### 5.6 moderation_status

1. pending
2. approved
3. rejected
4. quarantined

### 5.7 source_client

1. web
2. weapp
3. operator-web
4. admin-web
5. system

## 6. DTO 定义

### 6.1 MediaVariants

```json
{
  "thumb_url": "https://cdn.example.com/...",
  "card_url": "https://cdn.example.com/...",
  "detail_url": "https://cdn.example.com/...",
  "original_url": "https://cdn.example.com/..."
}
```

### 6.2 MediaAssetResponse

```json
{
  "id": 123,
  "object_key": "merchant/dish/1001/abc123.jpg",
  "bucket_type": "public",
  "visibility": "public",
  "media_kind": "image",
  "media_category": "dish",
  "mime_type": "image/jpeg",
  "file_size": 438221,
  "width": 1920,
  "height": 1080,
  "upload_status": "confirmed",
  "moderation_status": "approved",
  "uploaded_by": 1001,
  "source_client": "web",
  "variants": {
    "thumb_url": "https://cdn.example.com/...",
    "card_url": "https://cdn.example.com/...",
    "detail_url": "https://cdn.example.com/...",
    "original_url": "https://cdn.example.com/..."
  },
  "created_at": "2026-03-18T10:00:00Z",
  "updated_at": "2026-03-18T10:00:00Z"
}
```

### 6.3 UploadFormFields

```json
{
  "key": "merchant/dish/1001/abc123.jpg",
  "policy": "...",
  "signature": "...",
  "access_key_id": "...",
  "success_action_status": "204"
}
```

说明：

1. 字段名根据 OSS 厂商最终可能略有不同。
2. 契约上以“后端透明透传 upload 表单字段”为原则，不让前端硬编码厂商字段。

## 7. POST /v1/media/upload-sessions

### 7.1 用途

创建一个新的媒体上传会话，并返回直传 OSS 所需的对象键与上传凭证。

### 7.2 鉴权

需要 BearerAuth。

### 7.3 请求体

```json
{
  "business_type": "merchant_dish",
  "media_category": "dish",
  "visibility": "public",
  "filename": "dish.jpg",
  "content_type": "image/jpeg",
  "content_length": 438221,
  "checksum_sha256": "1d3f...",
  "source_client": "web",
  "resource_hint": {
    "resource_type": "dish",
    "resource_id": 0
  }
}
```

### 7.4 请求字段约束

| 字段 | 必填 | 约束 |
|---|---|---|
| business_type | 是 | 业务类型字符串，受 MediaPolicy 校验 |
| media_category | 是 | 必须为已定义 category |
| visibility | 是 | public 或 private |
| filename | 是 | 长度 1-255 |
| content_type | 是 | 必须为允许的 MIME 类型 |
| content_length | 是 | >0 且 <= MEDIA_MAX_UPLOAD_BYTES |
| checksum_sha256 | 是 | 64 位十六进制字符串 |
| source_client | 是 | web、weapp 等 |
| resource_hint | 否 | 资源上下文提示，不作为可信授权依据 |

### 7.5 成功响应

```json
{
  "upload_id": "up_01JABCDEF1234567890",
  "object_key": "merchant/dish/1001/1d3f....jpg",
  "bucket_type": "public",
  "visibility": "public",
  "upload_host": "https://oss-direct.example.com",
  "expire_at": "2026-03-18T10:05:00Z",
  "form": {
    "key": "merchant/dish/1001/1d3f....jpg",
    "policy": "...",
    "signature": "...",
    "access_key_id": "...",
    "success_action_status": "204"
  }
}
```

### 7.6 状态码

1. 200：成功
2. 400：参数不合法
3. 401：未认证
4. 403：无上传权限
5. 409：重复会话冲突或业务状态不允许上传
6. 500：服务内部错误
7. 503：对象存储服务未配置

### 7.7 幂等规则

对于相同用户、相同 business_type、相同 media_category、相同 checksum_sha256、相同 visibility、相同 resource_hint，在未过期且未完成的情况下，可返回已有 upload_session。

### 7.8 典型错误

建议新增以下业务错误码：

1. MediaUploadNotAllowed
2. MediaContentTypeNotAllowed
3. MediaFileTooLarge
4. MediaChecksumRequired
5. MediaStorageUnavailable
6. MediaUploadSessionConflict

## 8. POST /v1/media/complete

### 8.1 用途

客户端完成 OSS 直传后，调用本接口让后端确认对象存在、生成 media_assets、并返回 media_asset_id。

### 8.2 鉴权

需要 BearerAuth。

### 8.3 请求体

```json
{
  "upload_id": "up_01JABCDEF1234567890",
  "object_key": "merchant/dish/1001/1d3f....jpg",
  "etag": "\"5d41402abc4b2a76b9719d911017c592\"",
  "resource_bind": {
    "resource_type": "dish",
    "resource_id": 0,
    "slot": "primary"
  }
}
```

### 8.4 字段说明

| 字段 | 必填 | 说明 |
|---|---|---|
| upload_id | 是 | upload_session 标识 |
| object_key | 是 | 必须与 upload_session 一致 |
| etag | 否 | 对象存储返回值，用于附加校验 |
| resource_bind | 否 | 业务绑定信息，允许为空 |

### 8.5 成功响应

```json
{
  "media_asset": {
    "id": 123,
    "object_key": "merchant/dish/1001/1d3f....jpg",
    "bucket_type": "public",
    "visibility": "public",
    "media_kind": "image",
    "media_category": "dish",
    "mime_type": "image/jpeg",
    "file_size": 438221,
    "width": 1920,
    "height": 1080,
    "upload_status": "confirmed",
    "moderation_status": "approved",
    "uploaded_by": 1001,
    "source_client": "web",
    "variants": {
      "thumb_url": "https://cdn.example.com/...",
      "card_url": "https://cdn.example.com/...",
      "detail_url": "https://cdn.example.com/...",
      "original_url": "https://cdn.example.com/..."
    },
    "created_at": "2026-03-18T10:00:03Z",
    "updated_at": "2026-03-18T10:00:03Z"
  }
}
```

### 8.6 状态码

1. 200：成功
2. 400：参数不合法
3. 401：未认证
4. 403：无权限完成此上传会话
5. 404：upload_session 不存在
6. 409：对象未上传、对象校验不匹配、会话已过期
7. 500：内部错误
8. 502：OSS 元数据读取失败

### 8.7 幂等规则

若相同 upload_id 已经成功 complete：

1. 不重复创建 media_assets
2. 不重复写业务绑定
3. 直接返回既有 media_asset

### 8.8 服务端校验要求

服务端必须校验：

1. upload_id 存在且属于当前用户
2. upload_session 未过期
3. object_key 与 upload_session 一致
4. OSS 对象真实存在
5. content_length 与实际对象大小一致或满足容忍策略
6. MIME 类型与会话申请阶段相符
7. checksum 若可校验则应校验

## 9. POST /v1/media/private-access

### 9.1 用途

对私有媒体进行二次鉴权后，生成短期访问地址。

### 9.2 鉴权

需要 BearerAuth。

### 9.3 请求体

```json
{
  "media_asset_id": 456,
  "reason": "merchant_audit_review"
}
```

### 9.4 字段约束

| 字段 | 必填 | 说明 |
|---|---|---|
| media_asset_id | 是 | 必须指向 private 资产 |
| reason | 是 | 访问原因，便于审计 |

### 9.5 成功响应

```json
{
  "media_asset_id": 456,
  "download_url": "https://private-download.example.com/...",
  "expire_at": "2026-03-18T10:05:00Z"
}
```

### 9.6 状态码

1. 200：成功
2. 400：参数错误
3. 401：未认证
4. 403：无访问权限
5. 404：媒体不存在
6. 409：媒体状态不允许访问
7. 500：内部错误
8. 502：签名地址生成失败

### 9.7 审计要求

每次签发 private-access 都必须记录：

1. request_id
2. user_id
3. media_asset_id
4. object_key
5. reason
6. expire_at

## 10. DELETE /v1/media/{id}

### 10.1 用途

删除媒体资产。

### 10.2 鉴权

需要 BearerAuth。

### 10.3 路径参数

| 参数 | 说明 |
|---|---|
| id | media_asset_id |

### 10.4 请求体

无。

### 10.5 成功响应

```json
{
  "id": 123,
  "status": "deleted"
}
```

### 10.6 状态码

1. 200：删除成功或幂等返回
2. 401：未认证
3. 403：无删除权限
4. 404：媒体不存在
5. 409：媒体仍被业务引用，不允许删除
6. 500：内部错误

### 10.7 幂等规则

如果媒体已被标记 deleted：

1. 返回 200
2. status 仍返回 deleted

### 10.8 删除语义

服务端执行：

1. 校验媒体权限
2. 检查是否仍被业务资源引用
3. 标记 media_assets.deleted_at
4. 将 upload_status 设为 deleted
5. 投递异步 OSS 删除任务

## 11. GET /v1/media/{id}

### 11.1 用途

获取单个媒体元数据，供管理后台、审核或业务详情页使用。

### 11.2 鉴权

需要 BearerAuth。

### 11.3 成功响应

返回 MediaAssetResponse。

### 11.4 状态码

1. 200：成功
2. 401：未认证
3. 403：无访问权限
4. 404：媒体不存在

## 12. 错误码建议

建议新增以下 APIError：

| 错误名 | HTTP | 建议 code | message |
|---|---|---|---|
| ErrMediaUploadNotAllowed | 403 | 40320 | media upload not allowed |
| ErrMediaContentTypeNotAllowed | 400 | 40097 | media content type not allowed |
| ErrMediaFileTooLarge | 400 | 40098 | media file too large |
| ErrMediaChecksumRequired | 400 | 40099 | checksum_sha256 is required |
| ErrMediaUploadSessionNotFound | 404 | 40420 | media upload session not found |
| ErrMediaUploadSessionExpired | 409 | 40920 | media upload session expired |
| ErrMediaUploadSessionConflict | 409 | 40921 | media upload session conflict |
| ErrMediaObjectNotFound | 409 | 40922 | uploaded object not found in storage |
| ErrMediaObjectMismatch | 409 | 40923 | uploaded object does not match upload session |
| ErrMediaAssetNotFound | 404 | 40421 | media asset not found |
| ErrMediaPrivateAccessDenied | 403 | 40321 | no permission to access private media |
| ErrMediaStillReferenced | 409 | 40924 | media asset is still referenced |
| ErrMediaStorageUnavailable | 503 | 50320 | media storage unavailable |
| ErrMediaStorageOperationFailed | 502 | 50220 | media storage operation failed |

## 13. 前端联调要求

### 13.1 Web 与小程序统一流程

统一遵循：

1. 调用 upload-sessions
2. 直传 OSS
3. 调用 complete
4. 拿到 media_asset_id
5. 后续业务接口提交 media_asset_id

### 13.2 禁止行为

禁止以下行为：

1. 继续调用旧的 multipart 业务上传接口
2. 前端自行拼接 object_key
3. 前端直接持久化 object_key 到业务表单
4. 前端默认读取 original_url 用于列表页展示

## 14. 响应兼容策略

### 14.1 业务接口

业务接口短期仍可继续返回 image_url、logo_url、avatar_url 等字段名。

但返回值生成方式必须切换为：

1. 从 media_asset_id 关联 media_assets
2. 由 MediaURLResolver 生成 variants
3. 将默认展示图回填到兼容字段

### 14.2 新媒体接口

新媒体接口不再返回“只包含 image_url 的简化结构”，而是统一返回带 media_asset_id 的完整媒体对象。

## 15. Swagger 与测试要求

### 15.1 Swagger 要求

每个媒体接口必须补齐：

1. @Summary
2. @Description
3. @Tags Media
4. @Security BearerAuth
5. @Param
6. @Success
7. @Failure
8. @Router

### 15.2 测试要求

至少覆盖：

1. upload-sessions 正常创建
2. upload-sessions 幂等复用
3. complete 成功
4. complete 重复调用幂等
5. private-access 权限校验
6. delete 被引用冲突
7. delete 幂等删除

## 16. 建议后续文档

在本契约稿之后，建议继续补充：

1. Web 直传施工单
2. 小程序直传施工单
3. 媒体模块代码结构设计稿
4. 测试与验收清单
