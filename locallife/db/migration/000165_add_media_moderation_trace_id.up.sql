ALTER TABLE media_assets
ADD COLUMN moderation_trace_id text;

CREATE UNIQUE INDEX idx_media_assets_moderation_trace_id
ON media_assets (moderation_trace_id)
WHERE moderation_trace_id IS NOT NULL;

COMMENT ON COLUMN media_assets.moderation_trace_id IS '微信异步图审 trace_id，用于回调结果关联';