ALTER TABLE merchant_group_applications
    DROP COLUMN IF EXISTS license_media_asset_id;

ALTER TABLE merchant_groups
    DROP COLUMN IF EXISTS license_media_asset_id;
