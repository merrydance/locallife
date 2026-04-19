DROP INDEX IF EXISTS idx_ecommerce_applyments_result_recovery;

CREATE INDEX idx_ecommerce_applyments_result_recovery
ON ecommerce_applyments (updated_at ASC, id ASC)
WHERE status IN ('submitted', 'checking', 'auditing', 'account_need_verify', 'to_be_signed', 'to_be_confirmed', 'signing', 'finish', 'rejected');