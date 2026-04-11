ALTER TABLE operator_applications
    DROP COLUMN IF EXISTS business_license_media_asset_id,
    DROP COLUMN IF EXISTS id_card_front_media_asset_id,
    DROP COLUMN IF EXISTS id_card_back_media_asset_id;
