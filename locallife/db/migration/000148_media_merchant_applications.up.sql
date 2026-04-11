-- merchant_applications 表：证照图片改用 media_asset_id
ALTER TABLE merchant_applications
    ADD COLUMN business_license_media_asset_id bigint REFERENCES media_assets(id),
    ADD COLUMN food_permit_media_asset_id      bigint REFERENCES media_assets(id),
    ADD COLUMN id_card_front_media_asset_id    bigint REFERENCES media_assets(id),
    ADD COLUMN id_card_back_media_asset_id     bigint REFERENCES media_assets(id);

COMMENT ON COLUMN merchant_applications.business_license_media_asset_id IS '营业执照图片媒体资产 ID';
COMMENT ON COLUMN merchant_applications.food_permit_media_asset_id      IS '食品经营许可证图片媒体资产 ID';
COMMENT ON COLUMN merchant_applications.id_card_front_media_asset_id    IS '法人身份证正面媒体资产 ID';
COMMENT ON COLUMN merchant_applications.id_card_back_media_asset_id     IS '法人身份证背面媒体资产 ID';
