DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'merchant_applications'
          AND column_name = 'legal_person_id_front_media_asset_id'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'merchant_applications'
          AND column_name = 'id_card_front_media_asset_id'
    ) THEN
        ALTER TABLE merchant_applications
            RENAME COLUMN legal_person_id_front_media_asset_id TO id_card_front_media_asset_id;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'merchant_applications'
          AND column_name = 'legal_person_id_back_media_asset_id'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'merchant_applications'
          AND column_name = 'id_card_back_media_asset_id'
    ) THEN
        ALTER TABLE merchant_applications
            RENAME COLUMN legal_person_id_back_media_asset_id TO id_card_back_media_asset_id;
    END IF;
END $$;