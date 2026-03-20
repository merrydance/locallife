-- operator_applications 表：证照图片改用 media_asset_id
ALTER TABLE operator_applications
    ADD COLUMN business_license_media_asset_id bigint REFERENCES media_assets(id),
    ADD COLUMN id_card_front_media_asset_id     bigint REFERENCES media_assets(id),
    ADD COLUMN id_card_back_media_asset_id      bigint REFERENCES media_assets(id);

COMMENT ON COLUMN operator_applications.business_license_media_asset_id IS '运营商营业执照媒体资产 ID';
COMMENT ON COLUMN operator_applications.id_card_front_media_asset_id     IS '运营商法人身份证正面媒体资产 ID';
COMMENT ON COLUMN operator_applications.id_card_back_media_asset_id      IS '运营商法人身份证背面媒体资产 ID';
