ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS bucket_type text;

UPDATE media_assets
SET bucket_type = CASE
    WHEN visibility = 'public' THEN 'public'
    ELSE 'private'
END
WHERE bucket_type IS NULL;

ALTER TABLE media_assets
    ALTER COLUMN bucket_type SET NOT NULL;
