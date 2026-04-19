DROP INDEX IF EXISTS idx_ecommerce_applyments_settlement_verify;

ALTER TABLE ecommerce_applyments
DROP CONSTRAINT IF EXISTS ecommerce_applyments_settlement_verify_status_check,
DROP COLUMN IF EXISTS settlement_verify_failed_notified_at,
DROP COLUMN IF EXISTS settlement_verify_fail_reason,
DROP COLUMN IF EXISTS settlement_verify_status,
DROP COLUMN IF EXISTS settlement_verify_check_count,
DROP COLUMN IF EXISTS settlement_verify_last_checked_at,
DROP COLUMN IF EXISTS settlement_verify_first_trade_at;