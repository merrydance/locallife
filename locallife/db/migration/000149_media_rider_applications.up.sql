-- rider_applications 表：证照图片改用 media_asset_id
ALTER TABLE rider_applications
    ADD COLUMN id_card_front_media_asset_id bigint REFERENCES media_assets(id),
    ADD COLUMN id_card_back_media_asset_id  bigint REFERENCES media_assets(id),
    ADD COLUMN health_cert_media_asset_id   bigint REFERENCES media_assets(id);

COMMENT ON COLUMN rider_applications.id_card_front_media_asset_id IS '骑手身份证正面媒体资产 ID';
COMMENT ON COLUMN rider_applications.id_card_back_media_asset_id  IS '骑手身份证背面媒体资产 ID';
COMMENT ON COLUMN rider_applications.health_cert_media_asset_id   IS '骑手健康证媒体资产 ID';
