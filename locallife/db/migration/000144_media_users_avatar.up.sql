-- users 表：头像改用 media_asset_id
-- 保留 avatar_url 供兼容（微信头像等外部 URL 仍可存此字段，但应用上传头像走 media_asset_id）
ALTER TABLE users
    ADD COLUMN avatar_media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN users.avatar_media_asset_id IS '用户头像媒体资产 ID，用于应用内上传的头像；微信头像仍存 avatar_url';
