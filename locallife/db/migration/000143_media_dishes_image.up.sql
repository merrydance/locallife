-- dishes 表：图片改用 media_asset_id
-- 保留 image_url 供兼容，migration 完成后由 MediaURLResolver 动态生成
ALTER TABLE dishes
    ADD COLUMN image_media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN dishes.image_media_asset_id IS '菜品图片媒体资产 ID，取代 image_url 字段';
