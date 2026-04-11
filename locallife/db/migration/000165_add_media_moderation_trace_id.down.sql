DROP INDEX IF EXISTS idx_media_assets_moderation_trace_id;

ALTER TABLE media_assets
DROP COLUMN IF EXISTS moderation_trace_id;