-- merchant_brands 表：品牌 logo 改用 media_asset_id
ALTER TABLE merchant_brands
    ADD COLUMN logo_media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN merchant_brands.logo_media_asset_id IS '品牌 Logo 媒体资产 ID，取代 logo_url 字段';
