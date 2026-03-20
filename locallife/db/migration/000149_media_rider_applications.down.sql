ALTER TABLE rider_applications
    DROP COLUMN IF EXISTS id_card_front_media_asset_id,
    DROP COLUMN IF EXISTS id_card_back_media_asset_id,
    DROP COLUMN IF EXISTS health_cert_media_asset_id;
