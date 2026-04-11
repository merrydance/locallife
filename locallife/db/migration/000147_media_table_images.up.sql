-- table_images 表：增加 media_asset_id 字段
-- 保留 image_url 供兼容读取，新上传的图片写 media_asset_id
ALTER TABLE table_images
    ADD COLUMN media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN table_images.media_asset_id IS '桌台图片媒体资产 ID，取代 image_url 字段';
