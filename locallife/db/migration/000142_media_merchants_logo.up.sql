-- merchants 表：logo 改用 media_asset_id
-- 保留 logo_url（旧字段）供 API 层兼容查询，不在此 migration 中删除
ALTER TABLE merchants
    ADD COLUMN logo_media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN merchants.logo_media_asset_id IS '商户 Logo 媒体资产 ID，取代 logo_url 字段';
