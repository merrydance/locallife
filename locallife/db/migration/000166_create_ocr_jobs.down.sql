DROP INDEX IF EXISTS idx_ocr_jobs_media_asset_id;
DROP INDEX IF EXISTS idx_ocr_jobs_status_next_retry_at;
DROP INDEX IF EXISTS idx_ocr_jobs_owner_created_at;
DROP INDEX IF EXISTS idx_ocr_jobs_idempotency_key;
DROP TABLE IF EXISTS ocr_jobs;