-- 补充 merchant_group_applications 和 merchant_groups 的媒体资产字段（000093 遗漏）
ALTER TABLE merchant_group_applications
    ADD COLUMN license_media_asset_id bigint REFERENCES media_assets(id);

ALTER TABLE merchant_groups
    ADD COLUMN license_media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN merchant_group_applications.license_media_asset_id IS '集团营业执照图片媒体资产 ID，取代 license_image_url 字段';
COMMENT ON COLUMN merchant_groups.license_media_asset_id             IS '集团营业执照图片媒体资产 ID，取代 license_image_url 字段';
