# 媒体系统数据库结构设计稿

## 1. 文档目标

本文档用于将媒体系统最终态的数据库设计具体化，作为 migration、sqlc 查询、后端模型改造和前端联调的施工依据。

本文档解决以下问题：

1. media_assets 表如何建模。
2. 上传会话是否需要落库以及如何落库。
3. 现有业务表中的图片字段如何替换为 media_asset_id。
4. 多图场景如何从数组或 URL 字段迁移到关联表。
5. migration 应按什么顺序执行。
6. 外键、索引、约束如何设计。

## 2. 设计原则

### 2.1 核心原则

1. 业务表不再直接存 URL。
2. 业务表不再直接存 object_key。
3. 业务表统一引用 media_asset_id。
4. object_key、bucket、mime_type、审核状态等媒体属性统一收口在 media_assets。
5. 单图字段直接使用 media_asset_id 外键。
6. 多图场景使用独立关联表，禁止继续使用 text[]、jsonb URL 数组承载图片关系。

### 2.2 命名原则

1. 单图字段统一命名为 xxx_media_asset_id。
2. 多图关联表统一命名为 resource_media_assets 或保留语义化表名但字段统一为 media_asset_id。
3. 所有关联表包含 sort_order，必要时包含 role。

### 2.3 兼容原则

当前服务尚未正式上线，因此本次数据库设计采用直接到位原则，不保留长期双写结构。

允许的短期兼容仅限：

1. API 响应层继续返回 image_url、logo_url、avatar_url 字段名。
2. 数据库存储层已经切换为 media_asset_id。

## 3. 当前数据库现状梳理

### 3.1 已识别的单图字段

当前 migration 中已存在的典型单图字段如下：

1. users.avatar_url
2. merchants.logo_url
3. dishes.image_url
4. combo_sets.image_url
5. tables.qr_code_url
6. merchant_applications.business_license_image_url（历史字段，现已迁移为 business_license_media_asset_id）
7. merchant_applications.legal_person_id_front_url（历史字段，现已迁移为 id_card_front_media_asset_id）
8. merchant_applications.legal_person_id_back_url（历史字段，现已迁移为 id_card_back_media_asset_id）
9. merchant_applications.food_permit_url（历史字段，现已迁移为 food_permit_media_asset_id）
10. rider_applications.id_card_front_url（历史字段，现已迁移为 id_card_front_media_asset_id）
11. rider_applications.id_card_back_url（历史字段，现已迁移为 id_card_back_media_asset_id）
12. rider_applications.health_cert_url（历史字段，现已迁移为 health_cert_media_asset_id）
13. operator_applications.business_license_url（历史字段，现已迁移为 business_license_media_asset_id）
14. operator_applications.id_card_front_url（历史字段，现已迁移为 id_card_front_media_asset_id）
15. operator_applications.id_card_back_url（历史字段，现已迁移为 id_card_back_media_asset_id）
16. merchant_group_applications.license_image_url
17. merchant_groups.license_image_url
18. merchant_brands.logo_url

### 3.2 已识别的多图场景

当前 migration 中已识别的多图场景如下：

1. table_images.image_url
2. reviews.images text[]
3. merchant_applications.storefront_images jsonb
4. merchant_applications.environment_images jsonb

### 3.3 特殊说明

1. table_images 已是独立表，但内容字段是 image_url，需要替换为 media_asset_id。
2. reviews.images 当前为 text[]，需要拆表。
3. merchant_applications 的 storefront_images 与 environment_images 当前为 jsonb URL 数组，也需要拆表。

## 4. 目标数据库实体

最终数据库实体建议分为三层：

1. 媒体元数据表
2. 上传会话表
3. 业务资源与媒体关联层

## 5. media_assets 表设计

### 5.1 表职责

media_assets 用于统一存储所有媒体对象的元数据，是所有业务表引用媒体的唯一入口。

### 5.2 建议表结构

```sql
CREATE TABLE media_assets (
    id BIGSERIAL PRIMARY KEY,
    object_key TEXT NOT NULL,
    bucket_type TEXT NOT NULL,
    visibility TEXT NOT NULL,
    media_kind TEXT NOT NULL,
    media_category TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_ext TEXT,
    file_size BIGINT NOT NULL,
    width INT,
    height INT,
    checksum_sha256 TEXT NOT NULL,
    upload_status TEXT NOT NULL,
    moderation_status TEXT NOT NULL,
    source_client TEXT NOT NULL,
    uploaded_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    original_filename TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
```

### 5.3 字段说明

| 字段 | 说明 |
|---|---|
| id | 媒体主键，业务表统一引用 |
| object_key | OSS 对象键，全局唯一 |
| bucket_type | public 或 private |
| visibility | public 或 private，对外语义字段 |
| media_kind | image、video、document，当前先用 image |
| media_category | logo、dish、table、review、license 等 |
| mime_type | 媒体 MIME 类型 |
| file_ext | 原始扩展名 |
| file_size | 文件大小 |
| width | 原图宽 |
| height | 原图高 |
| checksum_sha256 | 用于去重与校验 |
| upload_status | pending、uploaded、confirmed、failed、deleted |
| moderation_status | pending、approved、rejected、quarantined |
| source_client | web、weapp、operator-web |
| uploaded_by | 上传用户 |
| original_filename | 原始文件名，仅审计用途 |
| metadata | 预留扩展信息 |

### 5.4 约束设计

建议增加以下约束：

```sql
ALTER TABLE media_assets
ADD CONSTRAINT media_assets_bucket_type_check
CHECK (bucket_type IN ('public', 'private'));

ALTER TABLE media_assets
ADD CONSTRAINT media_assets_visibility_check
CHECK (visibility IN ('public', 'private'));

ALTER TABLE media_assets
ADD CONSTRAINT media_assets_media_kind_check
CHECK (media_kind IN ('image', 'video', 'document'));

ALTER TABLE media_assets
ADD CONSTRAINT media_assets_upload_status_check
CHECK (upload_status IN ('pending', 'uploaded', 'confirmed', 'failed', 'deleted'));

ALTER TABLE media_assets
ADD CONSTRAINT media_assets_moderation_status_check
CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'quarantined'));

ALTER TABLE media_assets
ADD CONSTRAINT media_assets_file_size_check
CHECK (file_size >= 0);
```

### 5.5 索引设计

建议索引：

```sql
CREATE UNIQUE INDEX media_assets_object_key_udx ON media_assets(object_key);
CREATE UNIQUE INDEX media_assets_checksum_visibility_udx ON media_assets(checksum_sha256, visibility, deleted_at)
WHERE deleted_at IS NULL;
CREATE INDEX media_assets_uploaded_by_idx ON media_assets(uploaded_by);
CREATE INDEX media_assets_category_idx ON media_assets(media_category);
CREATE INDEX media_assets_visibility_moderation_idx ON media_assets(visibility, moderation_status);
CREATE INDEX media_assets_created_at_desc_idx ON media_assets(created_at DESC);
CREATE INDEX media_assets_active_idx ON media_assets(upload_status, moderation_status)
WHERE deleted_at IS NULL;
```

说明：

1. object_key 必须全局唯一。
2. checksum 唯一索引建议带 deleted_at 条件，避免软删除对象影响复用策略。

## 6. media_upload_sessions 表设计

### 6.1 是否需要单独建表

需要。

原因：

1. 直传 OSS 需要服务端生成上传会话。
2. complete 必须幂等，需要可追踪的上传上下文。
3. 需要控制上传会话过期、审计、回放保护与垃圾清理。

### 6.2 建议表结构

```sql
CREATE TABLE media_upload_sessions (
    id BIGSERIAL PRIMARY KEY,
    upload_id TEXT NOT NULL,
    uploaded_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_type TEXT NOT NULL,
    media_category TEXT NOT NULL,
    visibility TEXT NOT NULL,
    bucket_type TEXT NOT NULL,
    object_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    content_length BIGINT NOT NULL,
    checksum_sha256 TEXT NOT NULL,
    source_client TEXT NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.3 关键约束

```sql
CREATE UNIQUE INDEX media_upload_sessions_upload_id_udx ON media_upload_sessions(upload_id);
CREATE UNIQUE INDEX media_upload_sessions_object_key_udx ON media_upload_sessions(object_key);
CREATE INDEX media_upload_sessions_uploaded_by_idx ON media_upload_sessions(uploaded_by);
CREATE INDEX media_upload_sessions_status_expires_idx ON media_upload_sessions(status, expires_at);
```

status 建议值：

1. created
2. uploaded
3. completed
4. expired
5. failed

## 7. 单图字段改造清单

以下字段建议直接替换为 media_asset_id。

### 7.1 users

当前字段（历史）：

1. avatar_url

当前线上目标字段：

1. avatar_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.2 merchants

当前字段（历史）：

1. logo_url

当前线上目标字段：

1. logo_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.3 dishes

当前字段（历史）：

1. image_url

当前线上目标字段：

1. image_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.4 combo_sets

当前字段：

1. image_url

目标字段：

1. image_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.5 tables

当前字段：

1. qr_code_url

目标字段：

1. qr_code_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.6 merchant_applications

当前字段（历史）：

1. business_license_image_url
2. legal_person_id_front_url
3. legal_person_id_back_url
4. food_permit_url

当前线上目标字段：

1. business_license_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
2. legal_person_id_front_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
3. legal_person_id_back_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
4. food_permit_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.7 rider_applications

当前字段（历史）：

1. id_card_front_url
2. id_card_back_url
3. health_cert_url

当前线上目标字段：

1. id_card_front_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
2. id_card_back_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
3. health_cert_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.8 operator_applications

当前字段（历史）：

1. business_license_url
2. id_card_front_url
3. id_card_back_url

当前线上目标字段：

1. business_license_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
2. id_card_front_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL
3. id_card_back_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.9 merchant_group_applications

当前字段：

1. license_image_url

目标字段：

1. license_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.10 merchant_groups

当前字段：

1. license_image_url

目标字段：

1. license_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

### 7.11 merchant_brands

当前字段：

1. logo_url

目标字段：

1. logo_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL

## 8. 多图关系表设计

### 8.1 table_images 改造

当前表：

1. table_id
2. image_url
3. sort_order
4. is_primary

目标表结构：

```sql
ALTER TABLE table_images
ADD COLUMN media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE CASCADE;
```

最终字段建议：

1. id
2. table_id
3. media_asset_id
4. sort_order
5. role
6. created_at

说明：

1. 不再保留 image_url。
2. role 替代 is_primary，更通用，可取 primary、gallery。

### 8.2 reviews.images 拆表

当前 reviews.images 为 text[]，需要拆为独立关联表。

建议新表：review_images

```sql
CREATE TABLE review_images (
    id BIGSERIAL PRIMARY KEY,
    review_id BIGINT NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    media_asset_id BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (review_id, media_asset_id)
);
```

索引：

```sql
CREATE INDEX review_images_review_id_idx ON review_images(review_id);
CREATE INDEX review_images_media_asset_id_idx ON review_images(media_asset_id);
```

说明：

1. reviews.images 字段最终删除。
2. 前端顺序由 sort_order 控制。

### 8.3 merchant_applications 门头图与环境图拆表

当前字段：

1. storefront_images jsonb
2. environment_images jsonb

建议新表：merchant_application_images

```sql
CREATE TABLE merchant_application_images (
    id BIGSERIAL PRIMARY KEY,
    merchant_application_id BIGINT NOT NULL REFERENCES merchant_applications(id) ON DELETE CASCADE,
    media_asset_id BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    image_group TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_application_id, media_asset_id)
);
```

约束：

```sql
ALTER TABLE merchant_application_images
ADD CONSTRAINT merchant_application_images_group_check
CHECK (image_group IN ('storefront', 'environment'));
```

说明：

1. image_group 用于区分门头图与环境图。
2. storefront_images 与 environment_images 最终删除。

## 9. 资源图片角色建议

对多图资源建议引入 role，而不仅仅依赖 is_primary。

推荐值：

1. primary
2. gallery
3. cover
4. qr

如果业务表里已经有单独主图字段，则多图关系表可不再存 primary，只保留 gallery；但在当前项目里，统一用 role 更利于通用查询。

## 10. 外键删除策略

### 10.1 主表到 media_assets

建议使用：

1. ON DELETE SET NULL

适用于单图字段，因为媒体软删除或删除失败时，不应强制删除业务主记录。

### 10.2 关联表到 media_assets

建议使用：

1. ON DELETE CASCADE

适用于 review_images、table_images、merchant_application_images 这类纯关系表。

## 11. 软删除策略

### 11.1 media_assets

采用软删除：deleted_at。

删除流程：

1. 业务解绑。
2. media_assets.deleted_at 赋值。
3. 异步删除 OSS 对象。
4. 若 OSS 删除失败，允许后续重试。

### 11.2 关联表

review_images、table_images、merchant_application_images 不需要软删除字段，直接删除行即可。

## 12. migration 顺序建议

当前 migration 已到 000139，建议新增如下 migration：

### 12.1 000140_create_media_assets_and_upload_sessions.up.sql

内容：

1. 创建 media_assets。
2. 创建 media_upload_sessions。
3. 建立约束和索引。

### 12.2 000141_add_media_asset_ids_to_single_image_tables.up.sql

内容：

1. users.avatar_media_asset_id
2. merchants.logo_media_asset_id
3. dishes.image_media_asset_id
4. combo_sets.image_media_asset_id
5. tables.qr_code_media_asset_id
6. merchant_applications 各单图 media_asset_id
7. rider_applications 各单图 media_asset_id
8. operator_applications 各单图 media_asset_id
9. merchant_group_applications.license_media_asset_id
10. merchant_groups.license_media_asset_id
11. merchant_brands.logo_media_asset_id

### 12.3 000142_create_media_relation_tables.up.sql

内容：

1. review_images
2. merchant_application_images
3. table_images 新增 media_asset_id 与 role

### 12.4 000143_drop_legacy_image_columns_after_code_switch.up.sql

内容：

1. 删除 users.avatar_url
2. 删除 merchants.logo_url
3. 删除 dishes.image_url
4. 删除 combo_sets.image_url
5. 删除 tables.qr_code_url
6. 删除 merchant_applications 的单图 URL 字段
7. 删除 rider_applications 的 URL 字段
8. 删除 operator_applications 的 URL 字段
9. 删除 merchant_group_applications.license_image_url
10. 删除 merchant_groups.license_image_url
11. 删除 merchant_brands.logo_url
12. 删除 reviews.images
13. 删除 merchant_applications.storefront_images
14. 删除 merchant_applications.environment_images
15. 删除 table_images.image_url 与 is_primary

说明：

1. 因为未正式上线，可以把代码切换和 drop legacy column 放得更紧。
2. 若你们希望更稳，也可把 000143 拆成 000143 和 000144 两步。

## 13. sqlc 改造建议

### 13.1 新增查询组

建议新增：

1. media_asset.sql
2. media_upload_session.sql
3. review_image.sql
4. merchant_application_image.sql

### 13.2 典型查询

至少需要：

1. CreateMediaAsset
2. GetMediaAsset
3. GetMediaAssetByObjectKey
4. UpdateMediaAssetStatus
5. SoftDeleteMediaAsset
6. CreateMediaUploadSession
7. GetMediaUploadSessionByUploadID
8. CompleteMediaUploadSession
9. CreateReviewImage
10. ListReviewImagesByReview
11. CreateMerchantApplicationImage
12. ListMerchantApplicationImages

## 14. 响应组装层影响

数据库切到 media_asset_id 后，所有响应组装层都需要做统一媒体解析。

建议统一模式：

1. 查询业务主表。
2. 收集所需 media_asset_id。
3. 批量查询 media_assets。
4. 由 MediaURLResolver 生成公共图规格 URL 或私有签名地址。
5. 组装回原有 DTO 字段。

禁止在 handler 中直接把 media_asset_id 当成 URL 返回。

## 15. 风险点与处理

### 15.1 风险：多处业务查询改动面大

处理：

1. 先实现统一的 media hydration helper。
2. 避免每个 handler 自己查 media_assets。

### 15.2 风险：review.images 从数组变为关联表后查询复杂度上升

处理：

1. 增加批量查询接口。
2. 在 service 层统一聚合。

### 15.3 风险：table_images 既有独立表又要改 role

处理：

1. 保留表名 table_images，减少业务侵入。
2. 仅替换 image_url 为 media_asset_id，并补 role。

## 16. 施工验收点

数据库层面完成的标志：

1. media_assets 表与 media_upload_sessions 表已创建。
2. 所有单图业务表均已切换到 media_asset_id。
3. reviews.images 不再使用 text[]。
4. merchant_applications 的 storefront_images、environment_images 不再使用 jsonb URL 数组。
5. table_images 已切换为 media_asset_id。
6. 所有 legacy URL 字段已删除。
7. sqlc 生成代码可完整覆盖新增实体与查询。

## 17. 建议后续文档

本数据库设计稿确定后，建议继续输出：

1. 媒体接口契约稿。
2. 代码改造任务分解单。
3. 测试用例与回归清单。
