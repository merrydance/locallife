DROP INDEX IF EXISTS idx_ecommerce_applyments_account_authorize_state;

ALTER TABLE ecommerce_applyments
DROP COLUMN IF EXISTS account_authorize_state_checked_at,
DROP COLUMN IF EXISTS account_authorize_state,
DROP COLUMN IF EXISTS account_willingness_reject_reason,
DROP COLUMN IF EXISTS account_willingness_qrcode_data,
DROP COLUMN IF EXISTS account_willingness_state,
DROP COLUMN IF EXISTS account_willingness_applyment_id,
DROP COLUMN IF EXISTS account_willingness_business_code;
