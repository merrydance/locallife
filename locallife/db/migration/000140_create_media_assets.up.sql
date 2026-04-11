-- 媒体资产主表
-- 统一管理所有上传至 OSS 的图片/文件元数据，业务表通过 media_asset_id 关联
CREATE TABLE media_assets (
    id                bigserial    PRIMARY KEY,
    object_key        text         NOT NULL,        -- OSS 对象键（路径），如 uploads/public/merchants/1/dishes/abc.jpg
    visibility        text         NOT NULL,        -- public | private
    media_category    text         NOT NULL,        -- logo | dish | table | review | avatar | business_license | food_permit | id_card_front | id_card_back | health_cert | operator_license | operator_id_card_front | operator_id_card_back
    mime_type         text         NOT NULL,        -- image/jpeg | image/png | image/webp 等
    file_size         bigint       NOT NULL,        -- 字节数
    width             integer,                      -- 原图宽度（px），上传完成后写入
    height            integer,                      -- 原图高度（px），上传完成后写入
    checksum_sha256   text         NOT NULL,        -- 文件内容 SHA-256，用于去重与校验
    upload_status     text         NOT NULL DEFAULT 'pending',     -- pending | uploaded | confirmed | failed | deleted
    moderation_status text         NOT NULL DEFAULT 'pending',     -- pending | approved | rejected | quarantined
    uploaded_by       bigint       NOT NULL REFERENCES users(id),  -- 上传用户
    source_client     text         NOT NULL,        -- web | weapp | operator-web | server
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),
    deleted_at        timestamptz                   -- 软删除时间，非 NULL 表示已删除

    CONSTRAINT media_assets_upload_status_check CHECK (upload_status IN ('pending', 'uploaded', 'confirmed', 'failed', 'deleted')),
    CONSTRAINT media_assets_moderation_status_check CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'quarantined')),
    CONSTRAINT media_assets_visibility_check CHECK (visibility IN ('public', 'private'))
);

CREATE UNIQUE INDEX idx_media_assets_object_key ON media_assets (object_key);
CREATE INDEX idx_media_assets_uploaded_by ON media_assets (uploaded_by);
CREATE INDEX idx_media_assets_media_category ON media_assets (media_category);
CREATE INDEX idx_media_assets_visibility_moderation ON media_assets (visibility, moderation_status);
CREATE INDEX idx_media_assets_created_at ON media_assets (created_at DESC);
CREATE INDEX idx_media_assets_checksum ON media_assets (checksum_sha256);

COMMENT ON TABLE media_assets IS '媒体资产表，统一管理 OSS 上传文件的元数据';
COMMENT ON COLUMN media_assets.object_key IS 'OSS 对象键，全局唯一，不包含域名';
COMMENT ON COLUMN media_assets.visibility IS '可见性：public（公共桶，走 CDN）或 private（私有桶，鉴权后签名访问）';
COMMENT ON COLUMN media_assets.media_category IS '媒体用途类别，决定 object_key 前缀和权限策略';
COMMENT ON COLUMN media_assets.upload_status IS '上传状态：pending（待上传）→ uploaded（已传到 OSS）→ confirmed（后端已确认）→ failed/deleted';
COMMENT ON COLUMN media_assets.moderation_status IS '内容审核状态：pending → approved/rejected/quarantined';
COMMENT ON COLUMN media_assets.source_client IS '来源客户端类型，用于审计';
