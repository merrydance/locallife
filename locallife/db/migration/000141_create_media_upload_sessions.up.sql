-- 上传会话表
-- 记录每次客户端申请直传 OSS 的会话，用于幂等控制、状态跟踪、complete 校验
CREATE TABLE media_upload_sessions (
    id              text         PRIMARY KEY,      -- upload_id，如 up_01J...，由后端生成
    media_asset_id  bigint       REFERENCES media_assets(id), -- complete 后关联
    user_id         bigint       NOT NULL REFERENCES users(id),
    business_type   text         NOT NULL,         -- merchant_dish | review | merchant_logo 等，与 MediaPolicy 对应
    media_category  text         NOT NULL,         -- 与 media_assets.media_category 一致
    visibility      text         NOT NULL,         -- public | private
    object_key      text         NOT NULL,         -- 预分配的 OSS 对象键
    checksum_sha256 text         NOT NULL,         -- 客户端上传前提供的文件 SHA-256
    content_type    text         NOT NULL,         -- 声明的 MIME 类型
    content_length  bigint       NOT NULL,         -- 声明的文件大小（字节）
    status          text         NOT NULL DEFAULT 'pending', -- pending | completed | expired | failed
    expire_at       timestamptz  NOT NULL,         -- 直传凭证有效期
    created_at      timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT media_upload_sessions_status_check CHECK (status IN ('pending', 'completed', 'expired', 'failed')),
    CONSTRAINT media_upload_sessions_visibility_check CHECK (visibility IN ('public', 'private'))
);

-- 幂等查询：同用户同类别同 checksum 的未完成会话唯一
CREATE UNIQUE INDEX idx_media_upload_sessions_idempotent
    ON media_upload_sessions (user_id, media_category, checksum_sha256)
    WHERE status = 'pending';

CREATE INDEX idx_media_upload_sessions_user_id ON media_upload_sessions (user_id);
CREATE INDEX idx_media_upload_sessions_expire_at ON media_upload_sessions (expire_at);

COMMENT ON TABLE media_upload_sessions IS '媒体上传会话表，每次申请直传 OSS 创建一条记录';
COMMENT ON COLUMN media_upload_sessions.id IS 'upload_id，客户端在 complete 时回传用于校验';
COMMENT ON COLUMN media_upload_sessions.object_key IS '预分配的 OSS 对象键，complete 时必须与此一致';
COMMENT ON COLUMN media_upload_sessions.expire_at IS '直传凭证过期时间，过期后 complete 失败';
