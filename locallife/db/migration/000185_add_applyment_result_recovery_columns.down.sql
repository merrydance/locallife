DROP INDEX IF EXISTS idx_ecommerce_applyments_result_recovery;

ALTER TABLE ecommerce_applyments
DROP COLUMN IF EXISTS result_task_processed_at,
DROP COLUMN IF EXISTS result_task_processed_state;